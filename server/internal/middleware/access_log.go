// AccessLog:访问日志中间件,记录 method/path/status/latency/ip,
// 自动携带 request_id(来自 RequestID 中间件)与 user_id(来自 JWTAuth);
// 按 HTTP 状态码分级:5xx → error,4xx → warn,其余 → info。
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"fawn-mall/pkg/logger"
)

// AccessLog 请求访问日志。须挂在 RequestID 之后(依赖 request_id)。
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Int64("latency_ms", time.Since(start).Milliseconds()),
			zap.String("ip", c.ClientIP()),
			zap.Int("body_size", c.Writer.Size()),
		}
		ctx := c.Request.Context()
		switch {
		case c.Writer.Status() >= 500:
			logger.Error(ctx, "access", fields...)
		case c.Writer.Status() >= 400:
			logger.Warn(ctx, "access", fields...)
		default:
			logger.Info(ctx, "access", fields...)
		}
	}
}
