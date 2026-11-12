// Package jwtx JWT 签发与解析(HS256)。
package jwtx

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 自定义载荷:用户 ID、角色、用户名。
type Claims struct {
	UserID   uint64 `json:"uid"`
	Role     string `json:"role"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Manager JWT 签发器。
type Manager struct {
	secret []byte
	expire time.Duration
}

// New 构造 Manager,expire 为 token 有效期(来自配置 jwt.expire)。
func New(secret string, expire time.Duration) *Manager {
	return &Manager{secret: []byte(secret), expire: expire}
}

// ExpireDuration 返回 token 有效期。
func (m *Manager) ExpireDuration() time.Duration {
	return m.expire
}

// Generate 签发 token。
func (m *Manager) Generate(userID uint64, role, username string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Role:     role,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ExpireDuration())),
			Issuer:    "fawn-mall",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Parse 解析并校验 token,失败返回错误。
func (m *Manager) Parse(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("非法签名算法")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("token 无效")
	}
	return claims, nil
}
