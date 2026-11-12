package service

import (
	"context"
	"errors"
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
