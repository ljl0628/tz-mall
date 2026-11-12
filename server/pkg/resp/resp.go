// Package resp 统一响应结构 { code, msg, data } 与业务错误码。
package resp

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
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
type BizError struct {
	Code int
	Msg  string
}

func (e *BizError) Error() string { return e.Msg }

// NewBiz 构造业务错误。
func NewBiz(code int, msg string) *BizError { return &BizError{Code: code, Msg: msg} }

// 预置常用业务错误,handler/service 直接引用。
var (
	ErrUserExists      = &BizError{CodeUserExists, "用户名已被注册"}
	ErrLoginFailed     = &BizError{CodeUserOrPassWrong, "用户名或密码错误"}
	ErrProductNotFound = &BizError{CodeProductNotFound, "商品不存在或已下架"}
	ErrStockNotEnough  = &BizError{CodeStockNotEnough, "库存不足"}
	ErrCartEmpty       = &BizError{CodeCartEmpty, "请先在购物车勾选要结算的商品"}
	ErrOrderNotFound   = &BizError{CodeOrderNotFound, "订单不存在"}
	ErrOrderStatus     = &BizError{CodeOrderStatus, "当前订单状态不支持该操作"}
	ErrCartItem        = &BizError{CodeCartItemNotFound, "购物车中不存在该商品"}
	ErrCategory        = &BizError{CodeCategoryNotFound, "分类不存在"}
	ErrCategoryDup     = &BizError{CodeCategoryDuplicated, "分类名称已存在"}
	ErrUserDisabled    = &BizError{CodeUserDisabled, "账号已被禁用,请联系管理员"}
	ErrUserNotFound    = &BizError{CodeUserNotFound, "用户不存在"}
	ErrSelfOp          = &BizError{CodeSelfOp, "不能对自己执行该操作"}
)

// OK 输出成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: CodeOK, Msg: "ok", Data: data})
}

// Fail 输出失败响应:业务错误按其错误码,其余按 500 处理(不向客户端泄露内部细节)。
func Fail(c *gin.Context, err error) {
	var be *BizError
	if errors.As(err, &be) {
		c.JSON(http.StatusOK, Body{Code: be.Code, Msg: be.Msg})
		return
	}
	c.JSON(http.StatusOK, Body{Code: CodeServerError, Msg: "服务开小差了,请稍后重试"})
}
