package repository

import "fmt"

// Redis key 统一管理(前缀 fawn:)。
const (
	keyPrefix = "fawn"

	keyStockPrefix = "stock" // fawn:stock:{productID}:{size} → 预扣库存(int)
	keyCartPrefix  = "cart"  // fawn:cart:{userID} → Hash, field={productID}:{size}
	keyPDPrefix    = "pd"    // fawn:pd:{productID} → 商品详情 JSON
	keyHome        = "home"  // fawn:home → 首页聚合 JSON
	keyBlkPrefix   = "blk"   // fawn:blk:{tokenSHA256} → 登出黑名单
	keyUdisPrefix  = "udis"  // fawn:udis:{userID} → 用户禁用标记(管理员封禁,即时生效)
	keyUrolePrefix = "urole" // fawn:urole:{userID} → 角色覆盖值(管理员调整角色,即时生效)
)

// StockKey 预扣库存 key。
func StockKey(productID uint64, size string) string {
	return fmt.Sprintf("%s:%s:%d:%s", keyPrefix, keyStockPrefix, productID, size)
}

// CartKey 购物车 key。
func CartKey(userID uint64) string {
	return fmt.Sprintf("%s:%s:%d", keyPrefix, keyCartPrefix, userID)
}

// PDKey 商品详情缓存 key。
func PDKey(productID uint64) string {
	return fmt.Sprintf("%s:%s:%d", keyPrefix, keyPDPrefix, productID)
}

// HomeKey 首页缓存 key。
func HomeKey() string {
	return fmt.Sprintf("%s:%s", keyPrefix, keyHome)
}

// BlkKey token 黑名单 key。
func BlkKey(tokenSHA string) string {
	return fmt.Sprintf("%s:%s:%s", keyPrefix, keyBlkPrefix, tokenSHA)
}

// UserDisabledKey 用户禁用标记 key。
func UserDisabledKey(userID uint64) string {
	return fmt.Sprintf("%s:%s:%d", keyPrefix, keyUdisPrefix, userID)
}

// RoleOverrideKey 角色覆盖 key。
func RoleOverrideKey(userID uint64) string {
	return fmt.Sprintf("%s:%s:%d", keyPrefix, keyUrolePrefix, userID)
}
