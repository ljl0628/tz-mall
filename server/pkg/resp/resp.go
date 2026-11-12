// Package resp 统一响应结构 { code, msg, data } 与业务错误码。
package resp

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"fawn-mall/pkg/apperr"
	"fawn-mall/pkg/logger"
)

// 统一错误码:0 成功;4xxxx 客户端/权限;5xxxx 服务端;6xxxx 业务。
const (
	CodeOK           = 0
	CodeParamError   = 40001 // 参数校验失败
	CodeUnauthorized = 40101 // 未登录 / token 失效
	CodeForbidden    = 40301 // 无权限(非管理员)
	CodeNotFound     = 40401 // 资源不存在
	CodeServerError  = 50001 // 服务端内部错误

	CodeUserExists         = 60001 // 用户名已存在
	CodeUserOrPassWrong    = 60002 // 用户名或密码错误
	CodeProductNotFound    = 60003 // 商品不存在或已下架
	CodeStockNotEnough     = 60004 // 库存不足
	CodeCartEmpty          = 60005 // 购物车未勾选商品
	CodeOrderNotFound      = 60006 // 订单不存在
	CodeOrderStatus        = 60007 // 订单状态不允许该操作
	CodeCartItemNotFound   = 60008 // 购物车项不存在
	CodeCategoryNotFound   = 60009 // 分类不存在
	CodeCategoryDuplicated = 60010 // 分类已存在
	CodeSelfDelete         = 60011 // 管理员不能删除自己
	CodeAdminExists        = 60012 // 管理员账号已存在
	CodeUserDisabled       = 60013 // 账号已被禁用
	CodeUserNotFound       = 60014 // 用户不存在
	CodeSelfOp             = 60015 // 不能对自己执行该操作
)

// Body 统一响应体。
type Body struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// BizError 业务错误,handler/service 之间传递,最终由 Fail 统一输出。
// 实现收敛在 pkg/apperr(与系统错误统一建模),此处为类型别名,业务代码零改动。
type BizError = apperr.Biz

// NewBiz 构造业务错误。
func NewBiz(code int, msg string) *BizError { return apperr.NewBiz(code, msg) }

// 预置常用业务错误,handler/service 直接引用。
var (
	ErrUserExists      = &BizError{Code: CodeUserExists, Msg: "用户名已被注册"}
	ErrLoginFailed     = &BizError{Code: CodeUserOrPassWrong, Msg: "用户名或密码错误"}
	ErrProductNotFound = &BizError{Code: CodeProductNotFound, Msg: "商品不存在或已下架"}
	ErrStockNotEnough  = &BizError{Code: CodeStockNotEnough, Msg: "库存不足"}
	ErrCartEmpty       = &BizError{Code: CodeCartEmpty, Msg: "请先在购物车勾选要结算的商品"}
	ErrOrderNotFound   = &BizError{Code: CodeOrderNotFound, Msg: "订单不存在"}
	ErrOrderStatus     = &BizError{Code: CodeOrderStatus, Msg: "当前订单状态不支持该操作"}
	ErrCartItem        = &BizError{Code: CodeCartItemNotFound, Msg: "购物车中不存在该商品"}
	ErrCategory        = &BizError{Code: CodeCategoryNotFound, Msg: "分类不存在"}
	ErrCategoryDup     = &BizError{Code: CodeCategoryDuplicated, Msg: "分类名称已存在"}
	ErrUserDisabled    = &BizError{Code: CodeUserDisabled, Msg: "账号已被禁用,请联系管理员"}
	ErrUserNotFound    = &BizError{Code: CodeUserNotFound, Msg: "用户不存在"}
	ErrSelfOp          = &BizError{Code: CodeSelfOp, Msg: "不能对自己执行该操作"}
)

// OK 输出成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: CodeOK, Msg: "ok", Data: data})
}

// Fail 输出失败响应并分级记录日志(此处是错误日志的统一出口):
//   - 业务错误(apperr.Biz):warn + 业务码,正常业务流不需要告警;
//   - 系统错误(apperr.System):error + 操作名 + 底层错误 + 调用栈;
//   - 未包装的其他错误:按系统错误处理,error + 原始错误。
//
// 对客户端一律不泄露内部细节,非业务错误统一返回 50001。
func Fail(c *gin.Context, err error) {
	ctx := c.Request.Context()

	var be *BizError
	if errors.As(err, &be) {
		logger.Warn(ctx, "业务错误", zap.Int("code", be.Code), zap.String("msg", be.Msg))
		c.JSON(http.StatusOK, Body{Code: be.Code, Msg: be.Msg})
		return
	}

	var se *apperr.System
	if errors.As(err, &se) {
		logger.Error(ctx, "系统错误", zap.String("op", se.Op),
			zap.Error(se.Err), zap.ByteString("stack", se.Stack))
	} else {
		logger.Error(ctx, "未分类错误,按系统错误处理", zap.Error(err))
	}
	c.JSON(http.StatusOK, Body{Code: CodeServerError, Msg: "服务开小差了,请稍后重试"})
}
