package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// CartItem 购物车项(存储于 Redis Hash,field = "{productID}:{size}",
// 同款不同尺码为不同购物项)。
type CartItem struct {
	ProductID uint64    `json:"product_id"`
	Size      string    `json:"size"`
	Quantity  int       `json:"quantity"`
	Selected  bool      `json:"selected"`  // 结算勾选
	Name      string    `json:"name"`      // 加购时快照(列表时刷新)
	Image     string    `json:"image"`     // 加购时快照
	Price     int64     `json:"price"`     // 加购时快照(分)
	AgeGroup  string    `json:"age_group"` // 加购时快照
	AddedAt   time.Time `json:"added_at"`
}

// Field 购物车项在 Hash 中的 field。
func (it CartItem) Field() string {
	return fmt.Sprintf("%d:%s", it.ProductID, it.Size)
}

// CartKeyItem 由 field 解析出 productID/size。
func CartKeyItem(field string) (uint64, string, bool) {
	parts := strings.SplitN(field, ":", 2)
	if len(parts) != 2 {
		return 0, "", false
	}
	pid, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return pid, parts[1], true
}

// CartRepo 购物车仓储(Redis Hash)。
type CartRepo struct{ rdb *redis.Client }

// NewCartRepo 构造。
func NewCartRepo(rdb *redis.Client) *CartRepo { return &CartRepo{rdb: rdb} }

// Add 加购:同款同码数量累加(上限 99);返回累加后的数量。
func (r *CartRepo) Add(ctx context.Context, userID uint64, item CartItem, maxQty int) (*CartItem, error) {
	key := CartKey(userID)
	raw, err := r.rdb.HGet(ctx, key, item.Field()).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	if err == nil {
		var old CartItem
		if err := json.Unmarshal([]byte(raw), &old); err == nil {
			old.Quantity += item.Quantity
			if old.Quantity > maxQty {
				old.Quantity = maxQty
			}
			old.Name, old.Image, old.Price, old.AgeGroup = item.Name, item.Image, item.Price, item.AgeGroup
			if err := r.save(ctx, userID, old); err != nil {
				return nil, err
			}
			return &old, nil
		}
	}
	if item.Quantity > maxQty {
		item.Quantity = maxQty
	}
	item.AddedAt = time.Now()
	if err := r.save(ctx, userID, item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CartRepo) save(ctx context.Context, userID uint64, item CartItem) error {
	b, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return r.rdb.HSet(ctx, CartKey(userID), item.Field(), string(b)).Err()
}

// List 全量购物车项。
func (r *CartRepo) List(ctx context.Context, userID uint64) ([]CartItem, error) {
	m, err := r.rdb.HGetAll(ctx, CartKey(userID)).Result()
	if err != nil {
		return nil, err
	}
	items := make([]CartItem, 0, len(m))
	for _, raw := range m {
		var it CartItem
		if err := json.Unmarshal([]byte(raw), &it); err == nil {
			items = append(items, it)
		}
	}
	return items, nil
}

// Get 单项查询。
func (r *CartRepo) Get(ctx context.Context, userID uint64, productID uint64, size string) (*CartItem, error) {
	raw, err := r.rdb.HGet(ctx, CartKey(userID), fmt.Sprintf("%d:%s", productID, size)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var it CartItem
	if err := json.Unmarshal([]byte(raw), &it); err != nil {
		return nil, nil
	}
	return &it, nil
}

// UpdateQuantity 修改数量。
func (r *CartRepo) UpdateQuantity(ctx context.Context, userID uint64, productID uint64, size string, qty int) error {
	it, err := r.Get(ctx, userID, productID, size)
	if err != nil {
		return err
	}
	if it == nil {
		return nil
	}
	it.Quantity = qty
	return r.save(ctx, userID, *it)
}

// UpdateSelected 更新勾选状态。
func (r *CartRepo) UpdateSelected(ctx context.Context, userID uint64, productID uint64, size string, selected bool) error {
	it, err := r.Get(ctx, userID, productID, size)
	if err != nil {
		return err
	}
	if it == nil {
		return nil
	}
	it.Selected = selected
	return r.save(ctx, userID, *it)
}

// UpdateAllSelected 全选/取消全选。
func (r *CartRepo) UpdateAllSelected(ctx context.Context, userID uint64, selected bool) error {
	items, err := r.List(ctx, userID)
	if err != nil {
		return err
	}
	pipe := r.rdb.Pipeline()
	key := CartKey(userID)
	for _, it := range items {
		it.Selected = selected
		b, _ := json.Marshal(it)
		pipe.HSet(ctx, key, it.Field(), string(b))
	}
	_, err = pipe.Exec(ctx)
	return err
}

// Remove 删除单项。
func (r *CartRepo) Remove(ctx context.Context, userID uint64, productID uint64, size string) error {
	return r.rdb.HDel(ctx, CartKey(userID), fmt.Sprintf("%d:%s", productID, size)).Err()
}

// RemoveFields 删除多个 field(下单成功后清理已结算项)。
func (r *CartRepo) RemoveFields(ctx context.Context, userID uint64, fields []string) error {
	if len(fields) == 0 {
		return nil
	}
	return r.rdb.HDel(ctx, CartKey(userID), fields...).Err()
}

// Count 购物车总件数(角标展示)。
func (r *CartRepo) Count(ctx context.Context, userID uint64) (int, error) {
	n, err := r.rdb.HLen(ctx, CartKey(userID)).Result()
	return int(n), err
}
