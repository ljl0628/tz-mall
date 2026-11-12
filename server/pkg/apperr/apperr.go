// Package apperr 业务错误与系统错误的统一模型。
//
//   - Biz:可预期的业务失败(库存不足、状态冲突…),带业务码与用户可读信息,
//     不带堆栈;handler 层记 warn 即可,不属于告警对象;
//   - System:基础设施/未知异常(DB、Redis、MQ…),包装底层错误并捕获调用栈,
//     handler 层记 error(含 zap.Error),需要人工介入。
//
// pkg/resp.BizError 是 apperr.Biz 的类型别名,service 层既有代码零改动;
// 两类错误在 resp.Fail 处统一分流(业务 warn / 系统 error)。
package apperr

import (
	"fmt"
	"runtime"
)

// Biz 业务错误。
type Biz struct {
	Code int    // 业务错误码(见 pkg/resp 错误码常量)
	Msg  string // 用户可读信息
}

func (e *Biz) Error() string { return e.Msg }

// NewBiz 构造业务错误。
func NewBiz(code int, msg string) *Biz { return &Biz{Code: code, Msg: msg} }

// System 系统错误:底层错误 + 发生操作 + 构造点调用栈。
type System struct {
	Op    string // 出错操作描述,如 "order.create 落库"
	Err   error  // 底层错误
	Stack []byte // Wrap 时捕获的调用栈
}

func (e *System) Error() string {
	if e.Op != "" {
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	}
	return e.Err.Error()
}

// Unwrap 支持 errors.Is / errors.As 透传底层错误。
func (e *System) Unwrap() error { return e.Err }

// Wrap 包装系统错误并捕获调用栈;err 为 nil 时返回 nil,便于 `return apperr.Wrap(op, err)` 直传。
func Wrap(op string, err error) *System {
	if err == nil {
		return nil
	}
	return &System{Op: op, Err: err, Stack: callerStack()}
}

// callerStack 捕获当前调用栈(跳过本函数与 Wrap 两帧)。
func callerStack() []byte {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return buf[:n]
}
