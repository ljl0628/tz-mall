package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fawn-mall/internal/model"
	"fawn-mall/internal/repository"
	"fawn-mall/internal/testutil"
)

// stubPublisher 取消/超时链路不发布消息,仅占位满足依赖(被测路径不经过 MQ)。
type stubPublisher struct{}

func (stubPublisher) Publish(ctx context.Context, queue string, v any) error { return nil }

// setupOrderSvc 组装真实的 repo(service 持有真实 MySQL/Redis 句柄)。
func setupOrderSvc(t *testing.T) (*OrderService, *repository.OrderRepo, *repository.CacheStock) {
	t.Helper()
	db, rdb := testutil.Setup(t)
	orders := repository.NewOrderRepo(db)
	products := repository.NewProductRepo(db)
	cart := repository.NewCartRepo(rdb)
	stock := repository.NewCacheStock(rdb)
	return NewOrderService(orders, cart, products, stock, stubPublisher{}), orders, stock
}

// TestCancelIfUnpaid_TimeoutCancelAndRefund 核心链路:
// 下单落库(预扣+MySQL 扣减)→ 超时取消 → MySQL 与 Redis 库存同时回补 → 幂等。
func TestCancelIfUnpaid_TimeoutCancelAndRefund(t *testing.T) {
	svc, orderRepo, stock := setupOrderSvc(t)
	ctx := context.Background()
	db, rdb := testutil.Setup(t)

	// 造商品:两个尺码各 10 件
	p := testutil.NewProduct(t, db, "超时取消测试商品", []model.SKU{
		{Size: "110", Stock: 10}, {Size: "120", Stock: 10},
	})
	// Redis 库存初始化(SETNX)
	if err := stock.SyncFromMySQL(ctx, []repository.StockItem{
		{ProductID: p.ID, Size: "110", Quantity: 10},
		{ProductID: p.ID, Size: "120", Quantity: 10},
	}); err != nil {
		t.Fatalf("同步库存失败: %v", err)
	}
	t.Cleanup(func() {
		rdb.Del(ctx, repository.StockKey(p.ID, "110"), repository.StockKey(p.ID, "120"))
	})

	// 模拟下单接口:Redis Lua 预扣(Place 阶段)
	preDeduct := []repository.StockItem{
		{ProductID: p.ID, Size: "110", Quantity: 1},
		{ProductID: p.ID, Size: "120", Quantity: 1},
	}
	if err := stock.DeductBatch(ctx, preDeduct, func(repository.StockItem) string { return p.Name }); err != nil {
		t.Fatalf("Redis 预扣失败: %v", err)
	}

	// 模拟 MQ 消费者落库:MySQL 条件扣库存 + 创建订单
	orderNo := GenOrderNo()
	order := &model.Order{
		OrderNo: orderNo, UserID: 1, Status: model.OrderStatusPendingPayment,
		TotalAmount: 19800, PayAmount: 19800,
		Receiver: "测试妈妈", Phone: "13800138000", Address: "浙江省杭州市西湖区测试路 1 号",
	}
	items := []model.OrderItem{
		{ProductID: p.ID, ProductName: p.Name, Size: "110", Price: 9900, Quantity: 1},
		{ProductID: p.ID, ProductName: p.Name, Size: "120", Price: 9900, Quantity: 1},
	}
	if err := orderRepo.CreateOrderWithDeduct(order, items); err != nil {
		t.Fatalf("订单落库失败: %v", err)
	}
	t.Cleanup(func() {
		db.Where("order_id = ?", order.ID).Delete(&model.OrderItem{})
		db.Delete(&model.Order{}, order.ID)
	})

	// 校验扣减后两边库存
	if v, _ := stock.Get(ctx, p.ID, "110"); v != 9 {
		t.Fatalf("Redis 库存 = %d,期望 9", v)
	}
	var sku110 model.SKU
	db.Where("product_id = ? AND size = ?", p.ID, "110").First(&sku110)
	if sku110.Stock != 9 {
		t.Fatalf("MySQL 库存 = %d,期望 9", sku110.Stock)
	}

	// 超时取消
	ok, err := svc.CancelIfUnpaid(ctx, orderNo, CancelReasonTimeout)
	if err != nil || !ok {
		t.Fatalf("超时取消失败: ok=%v err=%v", ok, err)
	}

	after, _ := orderRepo.GetByOrderNo(orderNo, true)
	if after.Status != model.OrderStatusCanceled {
		t.Errorf("订单状态 = %s,期望 canceled", after.Status)
	}
	if after.CancelReason != CancelReasonTimeout {
		t.Errorf("取消原因 = %s", after.CancelReason)
	}
	if v, _ := stock.Get(ctx, p.ID, "110"); v != 10 {
		t.Errorf("Redis 回补后库存 = %d,期望 10", v)
	}
	db.Where("product_id = ? AND size = ?", p.ID, "110").First(&sku110)
	if sku110.Stock != 10 {
		t.Errorf("MySQL 回补后库存 = %d,期望 10", sku110.Stock)
	}

	// 幂等:重复取消返回 false 且不再回补
	ok, err = svc.CancelIfUnpaid(ctx, orderNo, CancelReasonTimeout)
	if err != nil || ok {
		t.Errorf("重复取消应返回 false: ok=%v err=%v", ok, err)
	}
	if v, _ := stock.Get(ctx, p.ID, "110"); v != 10 {
		t.Errorf("重复取消不应再次回补: %d", v)
	}
}

// TestPayAndCancelConcurrency 支付与取消并发竞争:
// 同一订单 50 个支付 + 50 个取消并发,恰好一个成功,且成功取消时库存只回补一次。
func TestPayAndCancelConcurrency(t *testing.T) {
	svc, orderRepo, stock := setupOrderSvc(t)
	ctx := context.Background()
	db, rdb := testutil.Setup(t)

	p := testutil.NewProduct(t, db, "并发竞争测试商品", []model.SKU{{Size: "120", Stock: 5}})
	if err := stock.SyncFromMySQL(ctx, []repository.StockItem{
		{ProductID: p.ID, Size: "120", Quantity: 5},
	}); err != nil {
		t.Fatalf("同步库存失败: %v", err)
	}
	// 模拟下单接口的 Redis 预扣(1 件)
	if err := stock.DeductBatch(ctx, []repository.StockItem{{ProductID: p.ID, Size: "120", Quantity: 1}},
		func(repository.StockItem) string { return p.Name }); err != nil {
		t.Fatalf("预扣失败: %v", err)
	}
	t.Cleanup(func() { rdb.Del(ctx, repository.StockKey(p.ID, "120")) })

	orderNo := GenOrderNo()
	order := &model.Order{
		OrderNo: orderNo, UserID: 2, Status: model.OrderStatusPendingPayment,
		TotalAmount: 9900, PayAmount: 9900,
		Receiver: "并发妈妈", Phone: "13800138000", Address: "北京市朝阳区测试路 2 号",
	}
	items := []model.OrderItem{{ProductID: p.ID, ProductName: p.Name, Size: "120", Price: 9900, Quantity: 1}}
	if err := orderRepo.CreateOrderWithDeduct(order, items); err != nil {
		t.Fatalf("订单落库失败: %v", err)
	}
	t.Cleanup(func() {
		db.Where("order_id = ?", order.ID).Delete(&model.OrderItem{})
		db.Delete(&model.Order{}, order.ID)
	})

	var paid, canceled, failed atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				if _, err := svc.Pay(ctx, 2, orderNo); err == nil {
					paid.Add(1)
				} else {
					failed.Add(1)
				}
			} else {
				if _, err := svc.Cancel(ctx, 2, orderNo); err == nil {
					canceled.Add(1)
				} else {
					failed.Add(1)
				}
			}
		}(i)
	}
	wg.Wait()

	if paid.Load()+canceled.Load() != 1 {
		t.Errorf("支付成功 %d 次 + 取消成功 %d 次,应恰好合计 1", paid.Load(), canceled.Load())
	}
	after, _ := orderRepo.GetByOrderNo(orderNo, false)
	want := model.OrderStatusPaid
	if canceled.Load() == 1 {
		want = model.OrderStatusCanceled
	}
	if after.Status != want {
		t.Errorf("订单终态 = %s,期望 %s", after.Status, want)
	}
	// 取消成功时库存恰好回补一次;支付成功时库存保持扣减
	rest, _ := stock.Get(ctx, p.ID, "120")
	if canceled.Load() == 1 && rest != 5 {
		t.Errorf("取消后 Redis 库存 = %d,期望 5(恰好回补一次)", rest)
	}
	if paid.Load() == 1 && rest != 4 {
		t.Errorf("支付后 Redis 库存 = %d,期望 4(不回补)", rest)
	}
}

// TestCreateOrderWithDeduct_NoOversell MySQL 侧最后防线:
// 库存 5,并发落库 10 个各买 1 的订单,只允许 5 个成功。
func TestCreateOrderWithDeduct_NoOversell(t *testing.T) {
	_, orderRepo, _ := setupOrderSvc(t)
	db, _ := testutil.Setup(t)

	p := testutil.NewProduct(t, db, "MySQL防超卖测试", []model.SKU{{Size: "130", Stock: 5}})

	var success atomic.Int64
	var mu sync.Mutex
	orderIDs := make([]uint64, 0, 10)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			o := &model.Order{
				OrderNo: fmt.Sprintf("FKTEST%06d%04d", time.Now().UnixNano()%1000000, i),
				UserID:  3, Status: model.OrderStatusPendingPayment,
				TotalAmount: 9900, PayAmount: 9900,
				Receiver: "抢购妈妈", Phone: "13800138000", Address: "上海市浦东新区测试路 3 号",
			}
			err := orderRepo.CreateOrderWithDeduct(o, []model.OrderItem{
				{ProductID: p.ID, ProductName: p.Name, Size: "130", Price: 9900, Quantity: 1},
			})
			if err == nil {
				success.Add(1)
				mu.Lock()
				orderIDs = append(orderIDs, o.ID)
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if success.Load() != 5 {
		t.Errorf("并发落库成功 %d 单,期望恰好 5(不超卖)", success.Load())
	}
	t.Cleanup(func() {
		for _, id := range orderIDs {
			db.Where("order_id = ?", id).Delete(&model.OrderItem{})
			db.Delete(&model.Order{}, id)
		}
	})
}
