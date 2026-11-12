package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/redis/go-redis/v9"
)

// Blacklist 登出 token 黑名单(登录态相关缓存):
// JWT 无状态,登出时把 token 摘要写入黑名单,TTL 为 token 剩余有效期;
// 鉴权中间件同时校验黑名单,实现「退出登录」即时失效。
type Blacklist struct{ rdb *redis.Client }

// NewBlacklist 构造。
func NewBlacklist(rdb *redis.Client) *Blacklist { return &Blacklist{rdb: rdb} }

func tokenSHA(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// Add 将 token 加入黑名单,ttl 为剩余有效期。
func (b *Blacklist) Add(ctx context.Context, token string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil // 已过期的 token 无需拉黑
	}
	return b.rdb.Set(ctx, BlkKey(tokenSHA(token)), 1, ttl).Err()
}

// IsBlocked 查询 token 是否已被拉黑。
func (b *Blacklist) IsBlocked(ctx context.Context, token string) (bool, error) {
	n, err := b.rdb.Exists(ctx, BlkKey(tokenSHA(token))).Result()
	return n > 0, err
}
