// RequestID:为每个请求生成/透传 request_id,写入 context 与响应头,
// 供访问日志、业务日志、错误日志串联同一条请求链路。
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"fawn-mall/pkg/logger"
)

// HeaderRequestID 请求/响应头字段名(客户端可传入,服务端原样回带)。
const HeaderRequestID = "X-Request-Id"

// RequestID 解析请求头中的 X-Request-Id;缺失则生成
// 「毫秒时间戳-随机数」格式的唯一 ID,并注入 request context。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(HeaderRequestID)
		if rid == "" {
			rid = newRequestID()
		}
		c.Request = c.Request.WithContext(logger.WithRequestID(c.Request.Context(), rid))
		c.Header(HeaderRequestID, rid) // 响应头回带,便于前端/网关反馈问题定位
		c.Next()
	}
}

// newRequestID 13 位毫秒时间戳 + 8 位随机十六进制:可按时间粗排序且全局唯一。
func newRequestID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return strconv.FormatInt(time.Now().UnixMilli(), 10) + "-" + hex.EncodeToString(b[:])
}
