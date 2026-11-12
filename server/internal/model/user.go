package model

import "time"

// 角色常量。
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// 用户状态常量(管理端可封禁违规用户)。
const (
	UserStatusDisabled = 0 // 已禁用:拒绝登录,已有 token 即时失效
	UserStatusActive   = 1 // 正常
)

// User 用户。
type User struct {
	ID           uint64    `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"size:128;not null" json:"-"`
	Nickname     string    `gorm:"size:64" json:"nickname"`
	Role         string    `gorm:"size:16;not null;default:user;index" json:"role"`
	Status       int8      `gorm:"not null;default:1;index" json:"status"` // 1 正常 0 禁用
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
