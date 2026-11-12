package repository

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"fawn-mall/internal/testutil"
)

// 全部用例连接真实 Redis(docker compose 起的 redis:7),验证 Lua 预扣脚本的原子性。

// TestLuaDeductBatch_ConcurrentNoOversell 并发扣减不超卖:
// 初始库存 50,200 个并发各扣 1,成功次数必须恰好 50,最终库存为 0。
func TestLuaDeductBatch_ConcurrentNoOversell(t *testing.T) {
	_, rdb := testutil.Setup(t)
	ctx := context.Background()
	stock := NewCacheStock(rdb)

	pid := uint64(900001)
	size := "110"
	key := StockKey(pid, size)
	t.Cleanup(func() { rdb.Del(ctx, key) })

	if err := rdb.Set(ctx, key, 50, 0).Err(); err != nil {
		t.Fatalf("初始化库存失败: %v", err)
	}

	var success, fail atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := stock.DeductBatch(ctx, []StockItem{{ProductID: pid, Size: size, Quantity: 1}},
				func(it StockItem) string { return fmt.Sprintf("%d-%s", it.ProductID, it.Size) })
			if err == nil {
				success.Add(1)
			} else {
				fail.Add(1)
			}
		}()
	}
	wg.Wait()

	if success.Load() != 50 {
		t.Errorf("并发预扣成功次数 = %d,期望 50(超卖或漏扣)", success.Load())
	}
	if fail.Load() != 150 {
		t.Errorf("失败次数 = %d,期望 150", fail.Load())
	}
	rest, _ := stock.Get(ctx, pid, size)
	if rest != 0 {
		t.Errorf("最终库存 = %d,期望 0", rest)
	}
}

// TestLuaDeductBatch_AllOrNothing 批量预扣原子性:
// 第二项库存不足时,第一项也不能被扣减。
func TestLuaDeductBatch_AllOrNothing(t *testing.T) {
	_, rdb := testutil.Setup(t)
	ctx := context.Background()
	stock := NewCacheStock(rdb)

	k1, k2 := StockKey(900002, "100"), StockKey(900002, "120")
	t.Cleanup(func() { rdb.Del(ctx, k1, k2) })
	rdb.Set(ctx, k1, 10, 0)
	rdb.Set(ctx, k2, 1, 0) // 故意不足

	items := []StockItem{
		{ProductID: 900002, Size: "100", Quantity: 5},
		{ProductID: 900002, Size: "120", Quantity: 2},
	}
	if err := stock.DeductBatch(ctx, items, func(it StockItem) string { return "测试商品" }); err == nil {
		t.Fatal("第二项库存不足,应返回错误")
	}
	v1, _ := stock.Get(ctx, 900002, "100")
	if v1 != 10 {
		t.Errorf("第一项不应被扣减: got %d, want 10", v1)
	}
}

// TestRefundBatch 回补正确性。
func TestRefundBatch(t *testing.T) {
	_, rdb := testutil.Setup(t)
	ctx := context.Background()
	stock := NewCacheStock(rdb)

	key := StockKey(900003, "130")
	t.Cleanup(func() { rdb.Del(ctx, key) })
	rdb.Set(ctx, key, 8, 0)

	items := []StockItem{{ProductID: 900003, Size: "130", Quantity: 3}}
	if err := stock.DeductBatch(ctx, items, func(StockItem) string { return "x" }); err != nil {
		t.Fatalf("预扣失败: %v", err)
	}
	if err := stock.RefundBatch(ctx, items); err != nil {
		t.Fatalf("回补失败: %v", err)
	}
	v, _ := stock.Get(ctx, 900003, "130")
	if v != 8 {
		t.Errorf("回补后库存 = %d,期望 8", v)
	}
}

// TestSyncFromMySQL SETNX 语义:已存在的 key 不被覆盖。
func TestSyncFromMySQL(t *testing.T) {
	_, rdb := testutil.Setup(t)
	ctx := context.Background()
	stock := NewCacheStock(rdb)

	key := StockKey(900004, "140")
	t.Cleanup(func() { rdb.Del(ctx, key) })
	rdb.Set(ctx, key, 3, 0) // 模拟预扣后的运行时值

	if err := stock.SyncFromMySQL(ctx, []StockItem{{ProductID: 900004, Size: "140", Quantity: 10}}); err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	v, _ := stock.Get(ctx, 900004, "140")
	if v != 3 {
		t.Errorf("SETNX 不应覆盖已有值: got %d, want 3", v)
	}
}
