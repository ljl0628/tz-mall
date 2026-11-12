package repository

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// deductLua 批量预扣库存 Lua 脚本(原子执行):
//
//	入参:KEYS[i] = 库存 key(productID:size),ARGV[i] = 对应扣减数量
//	逻辑:先统一校验所有 key 库存是否充足,不足则不扣任何库存并返回 {0, i};
//	      全部充足则逐项 DECRBY,返回 {1}。
//	注意:key 不存在视为库存 0,防止未初始化时超卖。
var deductLua = redis.NewScript(`
if #KEYS ~= #ARGV then
  return {0, 0}
end
for i = 1, #KEYS do
  local raw = redis.call('GET', KEYS[i])
  local stock = 0
  if raw then
    stock = tonumber(raw)
  end
  if stock < tonumber(ARGV[i]) then
    return {0, i}
  end
end
for i = 1, #KEYS do
  redis.call('DECRBY', KEYS[i], ARGV[i])
end
return {1}
`)

// CacheStock Redis 预扣库存仓储。
type CacheStock struct{ rdb *redis.Client }

// NewCacheStock 构造。
func NewCacheStock(rdb *redis.Client) *CacheStock { return &CacheStock{rdb: rdb} }

// StockItem 一次预扣中的一个条目。
type StockItem struct {
	ProductID uint64
	Size      string
	Quantity  int
}

// key/argv 组装。
func buildStockArgs(items []StockItem) ([]string, []any) {
	keys := make([]string, 0, len(items))
	argv := make([]any, 0, len(items))
	for _, it := range items {
		keys = append(keys, StockKey(it.ProductID, it.Size))
		argv = append(argv, it.Quantity)
	}
	return keys, argv
}

// DeductBatch 批量原子预扣。成功返回 nil;第 failIdx(1 起)项库存不足时返回错误。
// 部分不足时不会扣减任何一项(脚本内先校验后扣减)。
func (s *CacheStock) DeductBatch(ctx context.Context, items []StockItem, nameOf func(StockItem) string) error {
	keys, argv := buildStockArgs(items)
	res, err := deductLua.Run(ctx, s.rdb, keys, argv...).Result()
	if err != nil {
		return fmt.Errorf("redis 预扣库存脚本执行失败: %w", err)
	}
	arr, ok := res.([]any)
	if !ok || len(arr) == 0 {
		return fmt.Errorf("redis 预扣库存脚本返回异常: %v", res)
	}
	if code, _ := arr[0].(int64); code == 1 {
		return nil
	}
	var failIdx int64
	if len(arr) > 1 {
		failIdx, _ = arr[1].(int64)
	}
	name := "未知商品"
	if failIdx >= 1 && int(failIdx) <= len(items) && nameOf != nil {
		name = nameOf(items[failIdx-1])
	}
	return fmt.Errorf("%s 库存不足", name)
}

// RefundBatch 回补库存(逐项 INCRBY)。幂等性由调用方(订单状态条件更新)保证,
// 同一订单的取消只会成功执行一次,因此不会重复回补。
func (s *CacheStock) RefundBatch(ctx context.Context, items []StockItem) error {
	pipe := s.rdb.Pipeline()
	for _, it := range items {
		pipe.IncrBy(ctx, StockKey(it.ProductID, it.Size), int64(it.Quantity))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis 回补库存失败: %w", err)
	}
	return nil
}

// SyncFromMySQL 将 MySQL 库存同步进 Redis(仅当 key 不存在时写入,SETNX 语义):
// 避免覆盖运行期内因预扣产生的差额;Redis 数据丢失后可安全重建。
func (s *CacheStock) SyncFromMySQL(ctx context.Context, items []StockItem) error {
	pipe := s.rdb.Pipeline()
	for _, it := range items {
		pipe.SetNX(ctx, StockKey(it.ProductID, it.Size), it.Quantity, 0)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("同步库存到 redis 失败: %w", err)
	}
	return nil
}

// SetForcibly 管理端改库存时,以 MySQL 为准强制覆盖 Redis 值。
func (s *CacheStock) SetForcibly(ctx context.Context, productID uint64, size string, stock int) error {
	return s.rdb.Set(ctx, StockKey(productID, size), stock, 0).Err()
}

// Delete 删除商品时清理库存 key。
func (s *CacheStock) Delete(ctx context.Context, productID uint64, sizes []string) error {
	keys := make([]string, 0, len(sizes))
	for _, sz := range sizes {
		keys = append(keys, StockKey(productID, sz))
	}
	if len(keys) == 0 {
		return nil
	}
	return s.rdb.Del(ctx, keys...).Err()
}

// Get 查询单个 SKU 的 Redis 库存(不存在返回 -1,调用方回落 MySQL)。
func (s *CacheStock) Get(ctx context.Context, productID uint64, size string) (int, error) {
	v, err := s.rdb.Get(ctx, StockKey(productID, size)).Int()
	if err == redis.Nil {
		return -1, nil
	}
	return v, err
}
