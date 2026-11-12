package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"fawn-mall/internal/model"
	"fawn-mall/internal/repository"
	"fawn-mall/pkg/logger"
	"fawn-mall/pkg/resp"
)

// FinanceService 财务中心:退款售后(管理员直接退款 / 用户申请审核)。
// 流水查询、看板、对账、日结见后续方法(按模块逐步补充)。
type FinanceService struct {
	finance *repository.FinanceRepo
	orders  *repository.OrderRepo
	stock   *repository.CacheStock
}

// NewFinanceService 构造。
func NewFinanceService(f *repository.FinanceRepo, o *repository.OrderRepo, stock *repository.CacheStock) *FinanceService {
	return &FinanceService{finance: f, orders: o, stock: stock}
}

// RefundApplyReq 用户申请退款请求。
type RefundApplyReq struct {
	Reason string `json:"reason" binding:"required,min=2,max=200"`
}

// ApplyRefund 用户对已支付订单申请退款:仅创建「待处理」售后单,不改变订单状态,
// 等管理员审核;审核通过才执行真正的退款事务。
func (s *FinanceService) ApplyRefund(ctx context.Context, userID uint64, orderNo, reason string) error {
	order, err := s.orders.GetByOrderNo(orderNo, false)
	if err != nil {
		return err
	}
	// 订单不存在与无权访问同提示,不泄露他人订单是否存在
	if order == nil || order.UserID != userID {
		return resp.ErrOrderNotFound
	}
	if order.Status != model.OrderStatusPaid {
		return resp.NewBiz(resp.CodeOrderStatus, "仅「已支付」的订单可申请退款")
	}
	refund := &model.RefundOrder{
		RefundNo: GenRefundNo(),
		OrderNo:  orderNo,
		UserID:   userID,
		Reason:   reason,
		Status:   model.RefundStatusPending,
	}
	err = s.finance.CreateRefundApply(refund)
	switch {
	case err == nil:
	case errors.Is(err, repository.ErrOrderNotPaid):
		return resp.NewBiz(resp.CodeOrderStatus, "仅「已支付」的订单可申请退款")
	case errors.Is(err, repository.ErrRefundApplyExists):
		return resp.NewBiz(resp.CodeRefundApplyDup, "该订单已存在进行中的退款申请,请等待处理")
	case errors.Is(err, repository.ErrOrderNotFound):
		return resp.ErrOrderNotFound
	default:
		return err
	}
	logger.Info(ctx, "用户提交退款申请", zap.String("order_no", orderNo),
		zap.Uint64("user_id", userID), zap.String("refund_no", refund.RefundNo))
	return nil
}

// AdminRefundReq 管理员直接退款请求。
type AdminRefundReq struct {
	OrderNo string `json:"order_no" binding:"required,max=32"`
	Reason  string `json:"reason" binding:"required,min=2,max=200"`
}

// AdminRefund 管理员对「已支付」订单直接发起退款(整单全额):
// 事务内条件更新 paid → refunded,受影响行数为 0 即拒绝(重复操作或状态异常),
// 绝不超额退款;成功则追加一笔出账流水与一条已退款售后单,并回补库存。
func (s *FinanceService) AdminRefund(ctx context.Context, adminID uint64, orderNo, reason string) (*model.RefundOrder, error) {
	order, err := s.orders.GetByOrderNo(orderNo, true)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, resp.ErrOrderNotFound
	}
	now := time.Now()
	flow := &model.PayFlow{
		FlowNo:    GenFlowNo(),
		OrderNo:   orderNo,
		UserID:    order.UserID,
		Direction: model.FlowDirectionOut,
		BizType:   model.FlowBizRefund,
		Amount:    order.PayAmount,
		Status:    model.FlowStatusSuccess,
		Remark:    "退款:" + reason,
	}
	refund := &model.RefundOrder{
		RefundNo:    GenRefundNo(),
		OrderNo:     orderNo,
		UserID:      order.UserID,
		Amount:      order.PayAmount,
		Reason:      reason,
		Status:      model.RefundStatusRefunded,
		OperatorID:  adminID,
		ProcessedAt: &now,
	}
	ok, err := s.finance.AdminRefund(order, flow, refund)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, resp.NewBiz(resp.CodeRefundConflict, "重复操作或状态异常:该订单可能已退款、已发货或已取消")
	}
	s.restoreRedisStock(ctx, orderNo, order.Items)
	logger.Info(ctx, "管理员退款成功", zap.String("order_no", orderNo),
		zap.Uint64("admin_id", adminID), zap.Int64("amount", order.PayAmount),
		zap.String("flow_no", flow.FlowNo), zap.String("refund_no", refund.RefundNo))
	return refund, nil
}

// ProcessRefundReq 管理员处理退款申请请求。
type ProcessRefundReq struct {
	Approve bool `json:"approve"`
}

// ProcessRefund 管理员处理用户的退款申请:同意 → 执行退款事务(条件更新防并发);
// 拒绝 → 售后单标记 rejected,订单不动。
func (s *FinanceService) ProcessRefund(ctx context.Context, adminID uint64, refundNo string, approve bool) error {
	items, err := s.finance.ProcessRefundApply(refundNo, approve, adminID, GenFlowNo())
	switch {
	case err == nil:
	case errors.Is(err, repository.ErrRefundNotFound):
		return resp.NewBiz(resp.CodeRefundNotFound, "退款售后单不存在")
	case errors.Is(err, repository.ErrRefundNotPending):
		return resp.NewBiz(resp.CodeRefundConflict, "重复操作或状态异常:该申请已被处理")
	case errors.Is(err, repository.ErrOrderNotPaid):
		return resp.NewBiz(resp.CodeRefundConflict, "订单当前状态不允许退款(可能已发货或已退款),申请已作废")
	default:
		return err
	}
	if approve && len(items) > 0 {
		s.restoreRedisStock(ctx, refundNo, items)
	}
	logger.Info(ctx, "管理员处理退款申请", zap.String("refund_no", refundNo),
		zap.Uint64("admin_id", adminID), zap.Bool("approve", approve))
	return nil
}

// restoreRedisStock 事务外回补 Redis 预扣库存(best-effort 重试;
// 失败仅影响可售余量偏保守,不会超卖,与订单取消回补同策略)。
func (s *FinanceService) restoreRedisStock(ctx context.Context, key string, items []model.OrderItem) {
	stockItems := make([]repository.StockItem, 0, len(items))
	for _, it := range items {
		stockItems = append(stockItems, repository.StockItem{
			ProductID: it.ProductID, Size: it.Size, Quantity: it.Quantity,
		})
	}
	var rerr error
	for attempt := 0; attempt < 3; attempt++ {
		if rerr = s.stock.RefundBatch(ctx, stockItems); rerr == nil {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
	}
	if rerr != nil {
		logger.Error(ctx, "退款成功但 Redis 库存回补失败,请人工核对",
			zap.String("key", key), zap.Error(rerr))
	}
}

// ---------- 北京时间口径(财务"天"的边界,不依赖容器时区) ----------

// cstLoc 业务时区:日结/看板/趋势的"天"固定按北京时间切分。
var cstLoc = time.FixedZone("Asia/Shanghai", 8*3600)

// TodayCST 今天(北京时间,YYYY-MM-DD)。
func TodayCST() string { return time.Now().In(cstLoc).Format("2006-01-02") }

// cstDate 时刻所在的北京时间日期串。
func cstDate(t time.Time) string { return t.In(cstLoc).Format("2006-01-02") }

// DayRange 北京时间某日的 [00:00, 次日 00:00) 绝对时刻区间。
func DayRange(date string) (time.Time, time.Time, error) {
	d, err := time.ParseInLocation("2006-01-02", date, cstLoc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("日期格式应为 YYYY-MM-DD")
	}
	return d, d.Add(24 * time.Hour), nil
}

// ---------- 查询:流水 / 售后单 ----------

// ListFlows 资金流水分页(支持日期范围、订单号、流水号、方向筛选)。
func (s *FinanceService) ListFlows(ctx context.Context, q repository.FlowQuery) (*PageList, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 50 {
		q.PageSize = 10
	}
	if q.Direction != 0 && q.Direction != model.FlowDirectionIn && q.Direction != model.FlowDirectionOut {
		return nil, resp.NewBiz(resp.CodeParamError, "流水方向参数非法")
	}
	list, total, err := s.finance.ListFlows(q)
	if err != nil {
		return nil, err
	}
	return &PageList{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// ListRefunds 售后单分页。
func (s *FinanceService) ListRefunds(ctx context.Context, status string, page, pageSize int) (*PageList, error) {
	switch status {
	case "", model.RefundStatusPending, model.RefundStatusRefunded, model.RefundStatusRejected:
	default:
		return nil, resp.NewBiz(resp.CodeParamError, "售后单状态非法")
	}
	list, total, err := s.finance.ListRefunds(status, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &PageList{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

// ---------- 经营看板 / 趋势 ----------

// DashboardResp 看板核心指标(金额单位:分)。
type DashboardResp struct {
	TodayPay       int64 `json:"today_pay"`       // 今日销售额
	TodayRefund    int64 `json:"today_refund"`    // 今日退款额
	MonthNet       int64 `json:"month_net"`       // 本月净收入 = 本月日结(截至昨日) + 今日实时
	PendingRefunds int64 `json:"pending_refunds"` // 待处理退款申请数
}

// Dashboard 经营看板:历史读日结表,仅"今日"实时聚合流水表(小表+索引),
// 不在订单表上做实时大聚合。
func (s *FinanceService) Dashboard(ctx context.Context) (*DashboardResp, error) {
	today := TodayCST()
	start, end, err := DayRange(today)
	if err != nil {
		return nil, err
	}
	sum, err := s.finance.SumFlows(start, end)
	if err != nil {
		return nil, err
	}
	monthStart := time.Now().In(cstLoc).Format("2006-01") + "-01"
	stmts, err := s.finance.ListStatements(monthStart, today)
	if err != nil {
		return nil, err
	}
	var monthNet int64
	for _, st := range stmts {
		if st.StatDate == today {
			continue // 今日以实时口径计,避免与日结重复
		}
		monthNet += st.NetAmount
	}
	monthNet += sum.PayAmount - sum.RefundAmount
	pending, err := s.finance.CountPendingRefunds()
	if err != nil {
		return nil, err
	}
	return &DashboardResp{
		TodayPay:       sum.PayAmount,
		TodayRefund:    sum.RefundAmount,
		MonthNet:       monthNet,
		PendingRefunds: pending,
	}, nil
}

// TrendPoint 趋势图单日数据(金额单位:分)。
type TrendPoint struct {
	Date         string `json:"date"`
	PayAmount    int64  `json:"pay_amount"`
	RefundAmount int64  `json:"refund_amount"`
	NetAmount    int64  `json:"net_amount"`
}

// Trend 近 N 天收入趋势(默认 7 天,上限 90):
// 历史日期读日结表,今日实时聚合流水表,缺失日期补零保证图表连续。
func (s *FinanceService) Trend(ctx context.Context, days int) ([]TrendPoint, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	today := TodayCST()
	base, _ := time.ParseInLocation("2006-01-02", today, cstLoc)
	from := cstDate(base.AddDate(0, 0, -(days - 1)))
	stmts, err := s.finance.ListStatements(from, today)
	if err != nil {
		return nil, err
	}
	byDate := make(map[string]model.DailyStatement, len(stmts))
	for _, st := range stmts {
		byDate[st.StatDate] = st
	}
	start, end, _ := DayRange(today)
	sum, err := s.finance.SumFlows(start, end)
	if err != nil {
		return nil, err
	}
	points := make([]TrendPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := cstDate(base.AddDate(0, 0, -i))
		if d == today {
			points = append(points, TrendPoint{
				Date: d, PayAmount: sum.PayAmount, RefundAmount: sum.RefundAmount,
				NetAmount: sum.PayAmount - sum.RefundAmount,
			})
			continue
		}
		if st, ok := byDate[d]; ok {
			points = append(points, TrendPoint{
				Date: d, PayAmount: st.PayAmount, RefundAmount: st.RefundAmount,
				NetAmount: st.NetAmount,
			})
			continue
		}
		points = append(points, TrendPoint{Date: d}) // 无日结补零
	}
	return points, nil
}

// ---------- 对账 ----------

// ReconcileSide 单侧对账结果(金额单位:分)。
type ReconcileSide struct {
	OrderTotal int64 `json:"order_total"` // 订单侧总额
	FlowTotal  int64 `json:"flow_total"`  // 流水侧总额
	Diff       int64 `json:"diff"`        // 差额 = 流水侧 - 订单侧
	OrderCount int64 `json:"order_count"`
	FlowCount  int64 `json:"flow_count"`
}

// ReconcileResp 一键对账结果:进账/退款双口径比对,不平给出差异明细。
type ReconcileResp struct {
	Balanced    bool                           `json:"balanced"`
	PaySide     ReconcileSide                  `json:"pay_side"`
	RefundSide  ReconcileSide                  `json:"refund_side"`
	Mismatches  []repository.ReconcileMismatch `json:"mismatches,omitempty"`
	GeneratedAt string                         `json:"generated_at"`
}

// Reconcile 一键对账:比对「订单表已支付总额」与「支付流水成功总额」,
// 以及退款侧双口径;一致返回平账,不一致返回具体差额与差异订单明细。
func (s *FinanceService) Reconcile(ctx context.Context) (*ReconcileResp, error) {
	sums, err := s.finance.ReconcileSums()
	if err != nil {
		return nil, err
	}
	out := &ReconcileResp{
		PaySide: ReconcileSide{
			OrderTotal: sums.OrderPay, FlowTotal: sums.FlowPay,
			Diff:       sums.FlowPay - sums.OrderPay,
			OrderCount: sums.OrderPayCnt, FlowCount: sums.FlowPayCnt,
		},
		RefundSide: ReconcileSide{
			OrderTotal: sums.OrderRefund, FlowTotal: sums.FlowRefund,
			Diff:       sums.FlowRefund - sums.OrderRefund,
			OrderCount: sums.OrderRefundCnt, FlowCount: sums.FlowRefundCnt,
		},
		GeneratedAt: time.Now().In(cstLoc).Format("2006-01-02 15:04:05"),
	}
	out.Balanced = out.PaySide.Diff == 0 && out.RefundSide.Diff == 0
	if !out.Balanced {
		if out.Mismatches, err = s.finance.MismatchList(20); err != nil {
			return nil, err
		}
	}
	logger.Info(ctx, "一键对账执行", zap.Bool("balanced", out.Balanced),
		zap.Int64("pay_diff", out.PaySide.Diff), zap.Int64("refund_diff", out.RefundSide.Diff))
	return out, nil
}

// ---------- 日结 ----------

// StatementRunReq 手动日结请求:date 缺省昨天;days > 1 时回填 [date-days+1, date]。
type StatementRunReq struct {
	Date string `json:"date"`
	Days int    `json:"days" binding:"gte=0,lte=366"`
}

// RunDailyStatement 聚合指定日期(北京时间)的成功流水写日结表,幂等。
// date 缺省昨天;允许对当天执行(手动出快照)。
func (s *FinanceService) RunDailyStatement(ctx context.Context, date string) error {
	if date == "" {
		date = cstDate(time.Now().AddDate(0, 0, -1))
	}
	start, end, err := DayRange(date)
	if err != nil {
		return resp.NewBiz(resp.CodeParamError, err.Error())
	}
	sum, err := s.finance.SumFlows(start, end)
	if err != nil {
		return err
	}
	st := &model.DailyStatement{
		StatDate:     date,
		PayAmount:    sum.PayAmount,
		RefundAmount: sum.RefundAmount,
		NetAmount:    sum.PayAmount - sum.RefundAmount,
		PayCount:     int(sum.PayCount),
		RefundCount:  int(sum.RefundCount),
	}
	if err := s.finance.UpsertStatement(st); err != nil {
		return err
	}
	logger.Info(ctx, "日结完成", zap.String("stat_date", date),
		zap.Int64("pay", sum.PayAmount), zap.Int64("refund", sum.RefundAmount),
		zap.Int64("net", st.NetAmount))
	return nil
}

// RunStatements 手动触发日结:单日或区间回填,返回完成天数。
func (s *FinanceService) RunStatements(ctx context.Context, req StatementRunReq) (int, error) {
	if req.Date == "" {
		req.Date = cstDate(time.Now().AddDate(0, 0, -1))
	}
	base, err := time.ParseInLocation("2006-01-02", req.Date, cstLoc)
	if err != nil {
		return 0, resp.NewBiz(resp.CodeParamError, "日期格式应为 YYYY-MM-DD")
	}
	if req.Date > TodayCST() { // ISO 日期字符串可直接字典序比较
		return 0, resp.NewBiz(resp.CodeParamError, "不能对未来日期日结")
	}
	days := req.Days
	if days <= 0 {
		days = 1
	}
	done := 0
	for i := days - 1; i >= 0; i-- {
		if err := s.RunDailyStatement(ctx, cstDate(base.AddDate(0, 0, -i))); err != nil {
			return done, err
		}
		done++
	}
	return done, nil
}

// ListStatements 日结账单分页(对账管理页展示)。
func (s *FinanceService) ListStatements(ctx context.Context, page, pageSize int) (*PageList, error) {
	list, total, err := s.finance.ListStatementsPage(page, pageSize)
	if err != nil {
		return nil, err
	}
	return &PageList{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}
