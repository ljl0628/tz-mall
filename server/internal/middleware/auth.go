// Package middleware Gin 中间件:CORS、JWT 鉴权、管理员鉴权、请求日志、统一恢复。
package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"fawn-mall/pkg/jwtx"
	"fawn-mall/pkg/logger"
	"fawn-mall/pkg/resp"
)

// CORS 跨域中间件:开发环境前端跑在 5173,后端 8080,需放行跨域。
// token 走 Authorization 头,无需 cookie,Allow-Origin 使用通配即可。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// RequestLog 请求访问日志(方法/路径/状态/耗时)。
// 过渡实现:步骤 3 将升级为 RequestID + AccessLog 中间件(带 request_id、按状态码分级)。
func RequestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info(c.Request.Context(), "http",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Int64("latency_ms", time.Since(start).Milliseconds()),
			zap.String("ip", c.ClientIP()),
		)
	}
}

// Recovery 统一 panic 恢复,输出 JSON 错误而非断连。
// 过渡实现:步骤 3 将升级为带 zap.Stack 的统一 500 恢复中间件。
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, rec any) {
		logger.Error(c.Request.Context(), "panic recovered",
			zap.String("path", c.Request.URL.Path), zap.Any("panic", rec))
		c.AbortWithStatusJSON(200, resp.Body{Code: resp.CodeServerError, Msg: "服务开小差了,请稍后重试"})
	})
}

// 上下文键。
type ctxKey string

const userClaimsKey ctxKey = "user_claims"

// JWTAuth 鉴权中间件:解析 Bearer token → 校验黑名单 → 校验用户级守卫 → 注入用户信息。
func JWTAuth(jm *jwtx.Manager, bl TokenBlacklist, ug UserGuard) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		var token string
		if strings.HasPrefix(header, "Bearer ") {
			token = strings.TrimPrefix(header, "Bearer ")
		}
		if token == "" {
			abortUnauthorized(c, "请先登录")
			return
		}
		claims, err := jm.Parse(token)
		if err != nil {
			abortUnauthorized(c, "登录已过期,请重新登录")
			return
		}
		// 登出黑名单校验
		if bl != nil {
			if blocked, err := bl.IsBlocked(c.Request.Context(), token); err == nil && blocked {
				abortUnauthorized(c, "登录已失效,请重新登录")
				return
			}
		}
		// 用户级守卫:管理员封禁 / 调整角色后,对已签发 token 即时生效。
		// Redis 异常时放行并回退 token 内声明(与黑名单一致的降级策略)。
		if ug != nil {
			if disabled, err := ug.IsDisabled(c.Request.Context(), claims.UserID); err == nil && disabled {
				abortUnauthorized(c, "账号已被禁用,请联系管理员")
				return
			}
			if role, err := ug.RoleOverride(c.Request.Context(), claims.UserID); err == nil && role != "" {
				claims.Role = role
			}
		}
		c.Set(string(userClaimsKey), claims)
		c.Set("token", token)
		c.Next()
	}
}

// AdminRequired 管理员权限(需在 JWTAuth 之后)。
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetClaims(c)
		if claims == nil || claims.Role != "admin" {
			c.AbortWithStatusJSON(200, resp.Body{Code: resp.CodeForbidden, Msg: "需要管理员权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// TokenBlacklist token 黑名单查询接口(由 repository.Blacklist 实现)。
type TokenBlacklist interface {
	IsBlocked(ctx context.Context, token string) (bool, error)
}

// UserGuard 用户级状态守卫(由 repository.UserState 实现):
// 管理员封禁用户 / 调整角色后,已签发的 JWT 无法撤销,
// 借助 Redis 覆盖值让这些操作对存量 token 即时生效。
type UserGuard interface {
	IsDisabled(ctx context.Context, userID uint64) (bool, error)
	RoleOverride(ctx context.Context, userID uint64) (string, error) // 无覆盖返回 ""
}

func abortUnauthorized(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(200, resp.Body{Code: resp.CodeUnauthorized, Msg: msg})
	c.Abort()
}

// GetClaims 取当前请求的用户信息(未登录返回 nil)。
func GetClaims(c *gin.Context) *jwtx.Claims {
	v, ok := c.Get(string(userClaimsKey))
	if !ok {
		return nil
	}
	claims, _ := v.(*jwtx.Claims)
	return claims
}

// GetToken 取当前请求的原始 token。
func GetToken(c *gin.Context) string {
	v, _ := c.Get("token")
	t, _ := v.(string)
	return t
}
