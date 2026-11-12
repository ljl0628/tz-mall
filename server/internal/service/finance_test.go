package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fawn-mall/internal/model"
	"fawn-mall/internal/repository"
	"fawn-mall/internal/testutil"
	"fawn-mall/pkg/resp"
)

// setupFinanceSvc 组装财务服务(真实 MySQL/Redis,禁止 mock)。
func setupFinanceSvc(t *testing.T) (*FinanceService, *OrderService, *repository.OrderRepo, *repository.CacheStock) {
	t.Helper()
	orderSvc, orderRepo, stock := setupOrderSvc(t)
	db, _ := testutil.Setup(t)
	return NewFinanceService(repository.NewFinanceRepo(db), orderRepo, stock), orderSvc, orderRepo, stock
}

// createPaidOrder 造一笔已支付订单:商品(库存 5)→ Redis 预扣 1 → MySQL 扣减落库 → 模拟支付(写入进账流水)。
// 返回支付后的订单与商品 ID,并注册全程清理。
func createPaidOrder(t *testing.T, orderSvc *OrderService, orderRepo *repository.OrderRepo,
	stock *repository.CacheStock, uid uint64, name string) (*model.Order, uint64) {
	t.Helper()
	ctx := context.Background()
	db, rdb := testutil.Setup(t)

	p := testutil.NewProduct(t, db, name, []model.SKU{{Size: "120", Stock: 5}})
	t.Cleanup(func() { rdb.Del(ctx, repository.StockKey(p.ID, "120")) })
	if err := stock.SyncFromMySQL(ctx, []repository.StockItem{
		{ProductID: p.ID, Size: "120", Quantity: 5},
	}); err != nil {
		t.Fatalf("同步库存失败: %v", err)
	}
	if err := stock.DeductBatch(ctx, []repository.StockItem{
		{ProductID: p.ID, Size: "120", Quantity: 1},
	}, func(repository.StockItem) string { return name }); err != nil {
		t.Fatalf("Redis 预扣失败: %v", err)
	}

	order := &model.Order{
		OrderNo: GenOrderNo(), UserID: uid, Status: model.OrderStatusPendingPayment,
		TotalAmount: 9900, PayAmount: 9900,
		Receiver: "退款测试妈妈", Phone: "13800138000", Address: "广东省深圳市南山区测试路 5 号",
	}
	if err := orderRepo.CreateOrderWithDeduct(order, []model.OrderItem{
		{ProductID: p.ID, ProductName: name, Size: "120", Price: 9900, Quantity: 1},
	}); err != nil {
		t.Fatalf("订单落库失败: %v", err)
	}
	t.Cleanup(func() {
		db.Where("order_no = ?", order.OrderNo).Delete(&model.PayFlow{})
		db.Where("order_no = ?", order.OrderNo).Delete(&model.RefundOrder{})
		db.Where("order_id = ?", order.ID).Delete(&model.OrderItem{})
		db.Delete(&model.Order{}, order.ID)
	})

	paid, err := orderSvc.Pay(ctx, uid, order.OrderNo)
	if err != nil {
		t.Fatalf("模拟支付失败: %v", err)
	}
	return paid, p.ID
}

// assertBizCode 断言错误为指定业务码。
func assertBizCode(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatalf("期望业务错误码 %d,得到 nil", want)
	}
	var be *resp.BizError
	if !errors.As(err, &be) || be.Code != want {
		t.Fatalf("期望业务错误码 %d,得到: %v", want, err)
	}
}

// countFlows 统计订单指定方向的流水条数。
func countFlows(t *testing.T, orderNo string, direction int8) int64 {
	t.Helper()
	db, _ := testutil.Setup(t)
	var n int64
	if err := db.Model(&model.PayFlow{}).
		Where("order_no = ? AND direction = ?", orderNo, direction).
		Count(&n).Error; err != nil {
		t.Fatalf("统计流水失败: %v", err)
	}
	return n
}

// TestAdminRefund_Concurrency 防并发退款(财务中心核心验收):
// 同一已支付订单 20 个并发退款,恰好 1 个成功、19 个「重复操作或状态异常」;
// 出账流水与售后单各恰好 1 条,MySQL 与 Redis 库存恰好回补一次,绝不超额退款。
func TestAdminRefund_Concurrency(t *testing.T) {
	finSvc, orderSvc, orderRepo, stock := setupFinanceSvc(t)
	ctx := context.Background()
	db, _ := testutil.Setup(t)

	order, pid := createPaidOrder(t, orderSvc, orderRepo, stock, 7, "并发退款测试商品")
	orderNo := order.OrderNo

	var okCount, conflict atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := finSvc.AdminRefund(ctx, 99, orderNo, "并发退款测试")
			if err == nil {
				okCount.Add(1)
				return
			}
			var be *resp.BizError
			if errors.As(err, &be) && be.Code == resp.CodeRefundConflict {
				conflict.Add(1)
			}
		}()
	}
	wg.Wait()

	if okCount.Load() != 1 || conflict.Load() != 19 {
		t.Fatalf("并发退款:成功 %d 次、拒绝 %d 次,期望 1/19", okCount.Load(), conflict.Load())
	}

	after, _ := orderRepo.GetByOrderNo(orderNo, false)
	if after.Status != model.OrderStatusRefunded {
		t.Errorf("订单终态 = %s,期望 refunded", after.Status)
	}
	if after.RefundTime == nil {
		t.Errorf("退款时间未落库")
	}
	if n := countFlows(t, orderNo, model.FlowDirectionIn); n != 1 {
		t.Errorf("进账流水 = %d 条,期望 1(支付流水不受退款影响)", n)
	}
	if n := countFlows(t, orderNo, model.FlowDirectionOut); n != 1 {
		t.Errorf("出账流水 = %d 条,期望恰好 1(绝不超额退款)", n)
	}
	var refunds int64
	db.Model(&model.RefundOrder{}).Where("order_no = ?", orderNo).Count(&refunds)
	if refunds != 1 {
		t.Errorf("售后单 = %d 条,期望 1", refunds)
	}
	var sku model.SKU
	db.Where("product_id = ? AND size = ?", pid, "120").First(&sku)
	if sku.Stock != 5 {
		t.Errorf("MySQL 回补后库存 = %d,期望恰好回补一次到 5", sku.Stock)
	}
	if v, _ := stock.Get(ctx, pid, "120"); v != 5 {
		t.Errorf("Redis 回补后库存 = %d,期望 5", v)
	}
}

// TestRefundApplyLifecycle 用户申请 → 管理员审核通过全链路;
// 重复申请、冒用申请、重复处理、已退款再退均被正确拒绝。
func TestRefundApplyLifecycle(t *testing.T) {
	finSvc, orderSvc, orderRepo, stock := setupFinanceSvc(t)
	ctx := context.Background()
	db, _ := testutil.Setup(t)

	order, pid := createPaidOrder(t, orderSvc, orderRepo, stock, 8, "退款申请测试商品")

	// 用户提交申请
	if err := finSvc.ApplyRefund(ctx, order.UserID, order.OrderNo, "尺码不合适,想换大两码"); err != nil {
		t.Fatalf("提交退款申请失败: %v", err)
	}
	// 重复申请被拒
	assertBizCode(t, finSvc.ApplyRefund(ctx, order.UserID, order.OrderNo, "再申请一次"), resp.CodeRefundApplyDup)
	// 他人冒用被拒(订单不存在同提示)
	assertBizCode(t, finSvc.ApplyRefund(ctx, order.UserID+1, order.OrderNo, "冒用申请"), resp.CodeOrderNotFound)

	// 管理员审核通过
	var row model.RefundOrder
	if err := db.Where("order_no = ?", order.OrderNo).First(&row).Error; err != nil {
		t.Fatalf("查询售后单失败: %v", err)
	}
	if err := finSvc.ProcessRefund(ctx, 99, row.RefundNo, true); err != nil {
		t.Fatalf("审核通过失败: %v", err)
	}

	after, _ := orderRepo.GetByOrderNo(order.OrderNo, false)
	if after.Status != model.OrderStatusRefunded {
		t.Errorf("订单状态 = %s,期望 refunded", after.Status)
	}
	if n := countFlows(t, order.OrderNo, model.FlowDirectionOut); n != 1 {
		t.Errorf("出账流水 = %d 条,期望 1", n)
	}
	var updated model.RefundOrder
	db.First(&updated, row.ID)
	if updated.Status != model.RefundStatusRefunded || updated.OperatorID != 99 || updated.ProcessedAt == nil {
		t.Errorf("售后单终态异常: %+v", updated)
	}
	var sku model.SKU
	db.Where("product_id = ? AND size = ?", pid, "120").First(&sku)
	if sku.Stock != 5 {
		t.Errorf("MySQL 库存 = %d,期望回补到 5", sku.Stock)
	}
	if v, _ := stock.Get(ctx, pid, "120"); v != 5 {
		t.Errorf("Redis 库存 = %d,期望 5", v)
	}

	// 重复处理同一申请被拒
	assertBizCode(t, finSvc.ProcessRefund(ctx, 99, row.RefundNo, true), resp.CodeRefundConflict)
	// 已退款订单不能再申请
	assertBizCode(t, finSvc.ApplyRefund(ctx, order.UserID, order.OrderNo, "再来一次"), resp.CodeOrderStatus)
	// 已退款订单管理员直接退款也被拒
	_, err := finSvc.AdminRefund(ctx, 99, order.OrderNo, "直接退款")
	assertBizCode(t, err, resp.CodeRefundConflict)
}

// TestRefundReject 拒绝申请:订单保持已支付、无出账流水、库存不回补。
func TestRefundReject(t *testing.T) {
	finSvc, orderSvc, orderRepo, stock := setupFinanceSvc(t)
	ctx := context.Background()
	db, _ := testutil.Setup(t)

	order, pid := createPaidOrder(t, orderSvc, orderRepo, stock, 9, "退款拒绝测试商品")
	if err := finSvc.ApplyRefund(ctx, order.UserID, order.OrderNo, "七天无理由"); err != nil {
		t.Fatalf("提交申请失败: %v", err)
	}
	var row model.RefundOrder
	db.Where("order_no = ?", order.OrderNo).First(&row)
	if err := finSvc.ProcessRefund(ctx, 99, row.RefundNo, false); err != nil {
		t.Fatalf("拒绝失败: %v", err)
	}

	after, _ := orderRepo.GetByOrderNo(order.OrderNo, false)
	if after.Status != model.OrderStatusPaid {
		t.Errorf("拒绝后订单状态 = %s,期望仍为 paid", after.Status)
	}
	if n := countFlows(t, order.OrderNo, model.FlowDirectionOut); n != 0 {
		t.Errorf("拒绝不应产生出账流水: %d 条", n)
	}
	var sku model.SKU
	db.Where("product_id = ? AND size = ?", pid, "120").First(&sku)
	if sku.Stock != 4 {
		t.Errorf("拒绝后 MySQL 库存 = %d,期望保持扣减后的 4", sku.Stock)
	}
	if v, _ := stock.Get(ctx, pid, "120"); v != 4 {
		t.Errorf("拒绝后 Redis 库存 = %d,期望 4", v)
	}
	// 拒绝后可再次申请(无进行中的售后单)
	if err := finSvc.ApplyRefund(ctx, order.UserID, order.OrderNo, "重新申请"); err != nil {
		t.Fatalf("拒绝后应允许重新申请: %v", err)
	}
}

// TestRunStatementsRange 手动日结返回实际重算区间:
// days 只控制重算窗口大小,不决定台账列表条数——列表是全部历史日结
// (曾因此被误解为「选 2 天却返回 4 天」,区间随结果返回以便前端明示)。
func TestRunStatementsRange(t *testing.T) {
	finSvc, _, _, _ := setupFinanceSvc(t)
	ctx := context.Background()
	db, _ := testutil.Setup(t)
	today := TodayCST()
	yesterday := cstDate(time.Now().In(cstLoc).AddDate(0, 0, -1))
	t.Cleanup(func() {
		db.Where("stat_date IN ?", []string{today, yesterday}).Delete(&model.DailyStatement{})
	})

	// date=今天、days=2 → 恰好重算 [昨天, 今天],区间随结果返回
	res, err := finSvc.RunStatements(ctx, StatementRunReq{Date: today, Days: 2})
	if err != nil {
		t.Fatalf("RunStatements 失败: %v", err)
	}
	if res.DoneDays != 2 || res.From != yesterday || res.To != today {
		t.Fatalf("重算区间异常: %+v(期望 from=%s to=%s done=2)", res, yesterday, today)
	}
	var n int64
	db.Model(&model.DailyStatement{}).Where("stat_date IN ?", []string{today, yesterday}).Count(&n)
	if n != 2 {
		t.Errorf("应恰好落 2 行日结,实际 %d", n)
	}

	// days 缺省按 1 天,from=to=所选日期
	res, err = finSvc.RunStatements(ctx, StatementRunReq{Date: today})
	if err != nil || res.DoneDays != 1 || res.From != today || res.To != today {
		t.Fatalf("缺省 days 应按 1 天: %+v err=%v", res, err)
	}

	// 未来日期拒绝
	future := cstDate(time.Now().In(cstLoc).AddDate(0, 0, 1))
	if _, err := finSvc.RunStatements(ctx, StatementRunReq{Date: future}); err == nil {
		t.Fatal("未来日期应拒绝日结")
	}
}

// TestReconcileAndStatement 一键对账 + 日结 + 看板全链路:
// 支付/退款后平账;人为删除进账流水 → 不平并给出差额与明细;
// 日结幂等聚合;看板今日指标与趋势今日点位取实时流水。
func TestReconcileAndStatement(t *testing.T) {
	finSvc, orderSvc, orderRepo, stock := setupFinanceSvc(t)
	ctx := context.Background()
	db, _ := testutil.Setup(t)
	today := TodayCST()
	t.Cleanup(func() { db.Where("stat_date = ?", today).Delete(&model.DailyStatement{}) })

	order, _ := createPaidOrder(t, orderSvc, orderRepo, stock, 10, "对账日结测试商品")

	// 自愈:清掉历史孤儿流水/售后单(订单已被此前测试清理但账本残留),
	// 保证对账断言的基数干净;对账逻辑本身不受影响。
	db.Exec("DELETE FROM pay_flows WHERE order_no NOT IN (SELECT order_no FROM orders)")
	db.Exec("DELETE FROM refund_orders WHERE order_no NOT IN (SELECT order_no FROM orders)")

	// 支付后平账
	r, err := finSvc.Reconcile(ctx)
	if err != nil {
		t.Fatalf("对账失败: %v", err)
	}
	if !r.Balanced {
		t.Fatalf("支付后应平账: %+v", r)
	}

	// 退款后仍平账(进账侧不变,退款侧双口径一致)
	if _, err := finSvc.AdminRefund(ctx, 99, order.OrderNo, "对账退款"); err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	r, err = finSvc.Reconcile(ctx)
	if err != nil || !r.Balanced {
		t.Fatalf("退款后应平账: balanced=%v err=%v", r.Balanced, err)
	}
	if r.RefundSide.OrderTotal != order.PayAmount || r.RefundSide.FlowTotal != order.PayAmount {
		t.Errorf("退款侧口径异常: %+v", r.RefundSide)
	}

	// 日结(今日):进账/出账各一笔,净收入为 0;幂等重跑数值不变
	if err := finSvc.RunDailyStatement(ctx, today); err != nil {
		t.Fatalf("日结失败: %v", err)
	}
	if err := finSvc.RunDailyStatement(ctx, today); err != nil {
		t.Fatalf("日结幂等重跑失败: %v", err)
	}
	var st model.DailyStatement
	if err := db.Where("stat_date = ?", today).First(&st).Error; err != nil {
		t.Fatalf("查询日结失败: %v", err)
	}
	if st.PayAmount != order.PayAmount || st.RefundAmount != order.PayAmount || st.NetAmount != 0 {
		t.Errorf("日结金额异常: %+v(期望进/出各 %d,净 0)", st, order.PayAmount)
	}
	if st.PayCount != 1 || st.RefundCount != 1 {
		t.Errorf("日结笔数异常: %+v", st)
	}

	// 看板:今日销售额/退款额等于实付金额,待处理申请为 0
	dash, err := finSvc.Dashboard(ctx)
	if err != nil {
		t.Fatalf("看板失败: %v", err)
	}
	if dash.TodayPay != order.PayAmount || dash.TodayRefund != order.PayAmount {
		t.Errorf("看板今日指标异常: %+v", dash)
	}
	if dash.PendingRefunds != 0 {
		t.Errorf("待处理申请应清零: %+v", dash)
	}

	// 趋势:最后一个点位是今天,取实时流水
	points, err := finSvc.Trend(ctx, 7)
	if err != nil {
		t.Fatalf("趋势失败: %v", err)
	}
	if len(points) != 7 || points[len(points)-1].Date != today {
		t.Fatalf("趋势点位异常: %d 个,末日 %s", len(points), points[len(points)-1].Date)
	}
	last := points[len(points)-1]
	if last.PayAmount != order.PayAmount || last.RefundAmount != order.PayAmount || last.NetAmount != 0 {
		t.Errorf("趋势今日点位异常: %+v", last)
	}

	// 回归:昨日日结必须出现在趋势倒数第二点。DATE 列读回 string 字段时
	// 会变成 RFC3339(2026-11-12T00:00:00Z),曾导致按日期 map 匹配不上、历史天全为 0。
	yesterday := cstDate(time.Now().In(cstLoc).AddDate(0, 0, -1))
	db.Create(&model.DailyStatement{
		StatDate: yesterday, PayAmount: 12300, RefundAmount: 2300,
		NetAmount: 10000, PayCount: 2, RefundCount: 1,
	})
	t.Cleanup(func() { db.Where("stat_date = ?", yesterday).Delete(&model.DailyStatement{}) })
	points, err = finSvc.Trend(ctx, 7)
	if err != nil {
		t.Fatalf("趋势(含昨日日结)失败: %v", err)
	}
	hist := points[len(points)-2]
	if hist.Date != yesterday || hist.PayAmount != 12300 || hist.RefundAmount != 2300 || hist.NetAmount != 10000 {
		t.Errorf("趋势昨日点位应读日结表: %+v(期望日期 %s 进 12300 出 2300 净 10000)", hist, yesterday)
	}

	// 人为删除进账流水 → 不平,差额 = 流水侧 - 订单侧 = -实付金额,并给出差异明细
	db.Where("order_no = ? AND direction = ?", order.OrderNo, model.FlowDirectionIn).
		Delete(&model.PayFlow{})
	r, err = finSvc.Reconcile(ctx)
	if err != nil {
		t.Fatalf("对账失败: %v", err)
	}
	if r.Balanced {
		t.Fatal("流水缺失应判不平")
	}
	if r.PaySide.Diff != -order.PayAmount {
		t.Errorf("进账差额 = %d,期望 %d", r.PaySide.Diff, -order.PayAmount)
	}
	if len(r.Mismatches) == 0 {
		t.Error("不平时应给出差异明细")
	}
}
