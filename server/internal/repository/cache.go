package repository

import (
	"context"
	"encoding/json"
	"time"

	"fawn-mall/internal/model"

	"github.com/redis/go-redis/v9"
)

// 缓存 TTL 策略:
//   - 商品详情:fawn:pd:{id},TTL 30 分钟;管理端增删改商品时主动 DEL(双保险)。
//   - 首页聚合:fawn:home,TTL 10 分钟;商品/分类变更时主动 DEL。
//     即使主动失效遗漏,TTL 也能保证数据最终一致。
const (
	pdCacheTTL   = 30 * time.Minute
	homeCacheTTL = 10 * time.Minute
)

// Cache 商品/首页缓存仓储。
type Cache struct{ rdb *redis.Client }

// NewCache 构造。
func NewCache(rdb *redis.Client) *Cache { return &Cache{rdb: rdb} }

// GetProductDetail 读缓存,未命中返回 nil。
func (c *Cache) GetProductDetail(ctx context.Context, id uint64) (*model.ProductDetail, error) {
	raw, err := c.rdb.Get(ctx, PDKey(id)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var d model.ProductDetail
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, nil // 脏数据视为未命中
	}
	return &d, nil
}

// SetProductDetail 写缓存。
func (c *Cache) SetProductDetail(ctx context.Context, d *model.ProductDetail) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, PDKey(d.ID), string(b), pdCacheTTL).Err()
}

// InvalidateProduct 主动失效商品详情缓存(更新/删除商品时调用)。
func (c *Cache) InvalidateProduct(ctx context.Context, id uint64) error {
	return c.rdb.Del(ctx, PDKey(id)).Err()
}

// GetHome 读首页聚合缓存。
func (c *Cache) GetHome(ctx context.Context, out any) (bool, error) {
	raw, err := c.rdb.Get(ctx, HomeKey()).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return false, nil
	}
	return true, nil
}

// SetHome 写首页聚合缓存。
func (c *Cache) SetHome(ctx context.Context, in any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, HomeKey(), string(b), homeCacheTTL).Err()
}

// InvalidateHome 主动失效首页缓存。
func (c *Cache) InvalidateHome(ctx context.Context) error {
	return c.rdb.Del(ctx, HomeKey()).Err()
}
