// Package logger 基于 zap 的统一日志体系(业务代码只允许通过本包打日志)。
//
//   - dev:彩色 console 输出到 stdout,同时写文件;
//   - prod:JSON encoder 只写文件;
//   - 文件经 lumberjack 按大小切割、按天数保留、可压缩;
//   - 级别热切换:内部持有 zap.AtomicLevel,配置热加载时调用 SetLevel 即时生效;
//   - 请求级字段:WithContext(ctx) 自动携带 context 里的 request_id / user_id。
package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 上下文键(避免导出 string key 被业务方伪造,用私有类型)。
type ctxKey string

const (
	ctxKeyRequestID ctxKey = "request_id"
	ctxKeyUserID    ctxKey = "user_id"
)

var (
	std       *zap.Logger
	atomicLvl zap.AtomicLevel
	syncer    func() error
)

// Options 日志初始化参数(来自 config.LogConf + 环境标识)。
type Options struct {
	Service    string // service 字段
	Env        string // env 字段
	Level      string // debug|info|warn|error
	Format     string // console|json
	Dir        string // 日志目录
	MaxSizeMB  int
	MaxAgeDays int
	MaxBackups int
	Compress   bool
}

// Init 初始化全局日志器(main 里调用一次;重复调用以最后一次为准)。
func Init(opt Options) {
	lvl := zap.NewAtomicLevelAt(parseLevel(opt.Level))

	encCfg := zap.NewDevelopmentEncoderConfig()
	encCfg.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}

	console := zapcore.NewConsoleEncoder(encCfg)
	jsonEnc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())

	rotate := &lumberjack.Logger{
		Filename:   filepath.Join(opt.Dir, "fawn-mall.log"),
		MaxSize:    opt.MaxSizeMB,
		MaxAge:     opt.MaxAgeDays,
		MaxBackups: opt.MaxBackups,
		Compress:   opt.Compress,
	}

	var cores []zapcore.Core
	fields := []zap.Field{zap.String("service", opt.Service), zap.String("env", opt.Env)}
	if opt.Env == "prod" {
		// 生产:JSON 只写文件(容器场景 stdout 交给业务,结构化走文件采集)
		cores = append(cores, zapcore.NewCore(jsonEnc, zapcore.AddSync(rotate), lvl))
	} else {
		// 开发:彩色 console 上屏,同时落文件
		cores = append(cores,
			zapcore.NewCore(console, zapcore.AddSync(os.Stdout), lvl),
			zapcore.NewCore(console, zapcore.AddSync(rotate), lvl),
		)
	}
	l := zap.New(zapcore.NewTee(cores...),
		zap.AddCaller(), callerSkip, zap.Fields(fields...))

	std = l
	atomicLvl = lvl
	syncer = l.Sync
}

func parseLevel(s string) zapcore.Level {
	switch strings.ToLower(s) {
	case "debug":
		return zapcore.DebugLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// callerSkip 让封装函数的 caller 显示为业务调用点。
var callerSkip = zap.AddCallerSkip(1)

// SetLevel 热切换日志级别(仅级别;debug/info/warn/error),非法值忽略。
func SetLevel(level string) {
	if std == nil {
		return
	}
	atomicLvl.SetLevel(parseLevel(level))
}

// L 返回全局 *zap.Logger(未初始化时返回 Nop,保证可用)。
func L() *zap.Logger {
	if std == nil {
		return zap.NewNop()
	}
	return std
}

// Sync 刷盘,进程退出前调用。
func Sync() {
	if syncer != nil {
		_ = syncer()
	}
}

// ---------- context 携带的请求级字段 ----------

// WithRequestID 把 request_id 放入 context(中间件调用)。
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// WithUserID 把 user_id 放入 context(鉴权中间件调用)。
func WithUserID(ctx context.Context, userID uint64) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, userID)
}

// RequestIDFromContext 读取 request_id(无则空串)。
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// UserIDFromContext 读取 user_id。
func UserIDFromContext(ctx context.Context) uint64 {
	if ctx == nil {
		return 0
	}
	if v, ok := ctx.Value(ctxKeyUserID).(uint64); ok {
		return v
	}
	return 0
}

// WithContext 返回带 request_id / user_id 的 logger(存在才附加)。
func WithContext(ctx context.Context) *zap.Logger {
	l := L()
	if ctx == nil {
		return l
	}
	if rid := RequestIDFromContext(ctx); rid != "" {
		l = l.With(zap.String("request_id", rid))
	}
	if uid := UserIDFromContext(ctx); uid != 0 {
		l = l.With(zap.Uint64("user_id", uid))
	}
	return l
}

// ---------- 包级便捷函数:业务代码统一走这里 ----------
// 字段构造用 zap.Int64 / zap.String / zap.Error 等;禁止业务包自建 logger 实例。

func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Debug(msg, fields...)
}
func Info(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Info(msg, fields...)
}
func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Warn(msg, fields...)
}
func Error(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Error(msg, fields...)
}

// ---------- 脱敏工具 ----------

// MaskPhone 手机号脱敏:138****8888;非 11 位原样返回(便于排查异常输入)。
func MaskPhone(p string) string {
	if len(p) != 11 {
		return p
	}
	return p[:3] + "****" + p[7:]
}

// MaskToken token 脱敏:只保留前 6 位。
func MaskToken(t string) string {
	if len(t) <= 6 {
		return strings.Repeat("*", len(t))
	}
	return t[:6] + "******"
}

// MaskAny 通用前缀脱敏(密码等)。
func MaskAny(s string, keep int) string {
	if s == "" {
		return "(空)"
	}
	if len(s) <= keep {
		return strings.Repeat("*", len(s))
	}
	return fmt.Sprintf("%s%s", s[:keep], strings.Repeat("*", 6))
}
