package repository

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// UserState 用户级登录态守卫(Redis 覆盖值):
// JWT 本身无状态,签发后无法撤销。管理员「封禁用户 / 调整角色」后,
// 已发出去的 token 最长还要等 ExpireHours 才自然过期——期间被封用户仍能下单、
// 被降权管理员仍能进管理接口,这是不可接受的。
// 因此在 Redis 写两类覆盖键,鉴权中间件逐请求校验,让管理操作即时生效:
//
//	fawn:udis:{userID}  存在即拒绝该用户所有请求(封禁)
//	fawn:urole:{userID} 角色覆盖值,优先于 token 内的角色(升降权)
//
// TTL 与 token 有效期一致:过期后覆盖键自动清理,新 token 携带的就是最新状态。
type UserState struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewUserState 构造,tokenTTL 为 JWT 有效期(覆盖键的 TTL)。
func NewUserState(rdb *redis.Client, tokenTTL time.Duration) *UserState {
	return &UserState{rdb: rdb, ttl: tokenTTL}
}

// SetDisabled 封禁用户:覆盖键存在期间,该用户所有请求被拒。
func (s *UserState) SetDisabled(ctx context.Context, userID uint64) error {
	return s.rdb.Set(ctx, UserDisabledKey(userID), 1, s.ttl).Err()
}

// ClearDisabled 解封用户。
func (s *UserState) ClearDisabled(ctx context.Context, userID uint64) error {
	return s.rdb.Del(ctx, UserDisabledKey(userID)).Err()
}

// IsDisabled 用户是否处于封禁状态。
func (s *UserState) IsDisabled(ctx context.Context, userID uint64) (bool, error) {
	n, err := s.rdb.Exists(ctx, UserDisabledKey(userID)).Result()
	return n > 0, err
}

// SetRole 记录角色覆盖值(管理员调整角色后即时生效,优先于 token 内角色)。
func (s *UserState) SetRole(ctx context.Context, userID uint64, role string) error {
	return s.rdb.Set(ctx, RoleOverrideKey(userID), role, s.ttl).Err()
}

// ClearRole 清除角色覆盖(回退到 token 内角色),主要供测试与数据修正使用。
func (s *UserState) ClearRole(ctx context.Context, userID uint64) error {
	return s.rdb.Del(ctx, RoleOverrideKey(userID)).Err()
}

// RoleOverride 读取角色覆盖值,无覆盖(或键已过期)返回 ""。
func (s *UserState) RoleOverride(ctx context.Context, userID uint64) (string, error) {
	v, err := s.rdb.Get(ctx, RoleOverrideKey(userID)).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return v, nil
}
