// Recovery:panic 统一恢复中间件。
// 记录 panic 值 + 完整调用栈(request_id 可串联),返回统一 50001 业务码
// (项目约定:所有响应 HTTP 200 + 业务码,前端统一按 code 处理),进程不退出。
package middleware

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"fawn-mall/pkg/logger"
	"fawn-mall/pkg/resp"
)

// Recovery 捕获 handler 链路中的 panic。须最先挂载(兜住其余中间件的 panic)。
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, rec any) {
		logger.Error(c.Request.Context(), "panic recovered,返回统一错误",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Any("panic", rec),
			zap.Stack("stack"))
		c.AbortWithStatusJSON(200, resp.Body{Code: resp.CodeServerError, Msg: "服务开小差了,请稍后重试"})
	})
}
