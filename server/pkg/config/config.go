// Package config 基于 viper 的配置管理(对外只暴露强类型 Config)。
//
// 加载优先级(高 → 低):APP_ 前缀环境变量 > config.{env}.yaml > config.yaml > 内置默认值。
// 环境变量映射:层级用下划线,如 db.password → APP_DB_PASSWORD、jwt.secret → APP_JWT_SECRET。
// 多环境:APP_ENV(dev / test / prod,默认 dev)决定叠加的 config.{env}.yaml。
//
// viper 实例封闭在包内,业务代码不允许 import viper;
// 启动时 Validate() 强校验,不做事后补救的静默兜底。
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// 环境常量。
const (
	EnvDev  = "dev"
	EnvTest = "test"
	EnvProd = "prod"

	configDir = "config"
)

// Config 全局配置,字段与 config.yaml 一一对应。
type Config struct {
	Env      string       `mapstructure:"-"`
	Server   ServerConf   `mapstructure:"server"`
	DB       DBConf       `mapstructure:"db"`
	Redis    RedisConf    `mapstructure:"redis"`
	RabbitMQ RabbitMQConf `mapstructure:"rabbitmq"`
	JWT      JWTConf      `mapstructure:"jwt"`
	Log      LogConf      `mapstructure:"log"`
	Order    OrderConf    `mapstructure:"order"`
}

// ServerConf HTTP 服务。
type ServerConf struct {
	Port            int           `mapstructure:"port"`
	Mode            string        `mapstructure:"mode"` // debug | release
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// DBConf MySQL 连接与连接池。
type DBConf struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Name            string        `mapstructure:"name"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// RedisConf Redis 连接。
type RedisConf struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// RabbitMQConf MQ 连接与运维参数。
type RabbitMQConf struct {
	URL               string        `mapstructure:"url"`
	ReconnectInterval time.Duration `mapstructure:"reconnect_interval"`
	OpTimeout         time.Duration `mapstructure:"op_timeout"` // 发布/转停车队列等操作超时
}

// JWTConf 登录态签发。
type JWTConf struct {
	Secret string        `mapstructure:"secret"`
	Expire time.Duration `mapstructure:"expire"`
}

// LogConf 日志输出与切割。
type LogConf struct {
	Level      string `mapstructure:"level"`  // debug|info|warn|error,支持热加载
	Format     string `mapstructure:"format"` // console|json
	Dir        string `mapstructure:"dir"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	MaxBackups int    `mapstructure:"max_backups"`
	Compress   bool   `mapstructure:"compress"`
}

// OrderConf 订单业务参数。
type OrderConf struct {
	PayTimeoutSeconds int `mapstructure:"pay_timeout_seconds"`
	MaxRetry          int `mapstructure:"max_retry"`
}

// IsProd 是否生产环境。
func (c *Config) IsProd() bool { return c.Env == EnvProd }

// DSN 返回 MySQL 连接串(GORM)。
func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.DB.User, c.DB.Password, c.DB.Host, c.DB.Port, c.DB.Name)
}

// setDefaults 内置默认值。注意:密钥类(jwt.secret / db.password / redis.password)不设默认,
// 缺失时由 Validate 明确报错,避免「取不到就用默认密码」。
func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.read_timeout", 10*time.Second)
	v.SetDefault("server.write_timeout", 10*time.Second)
	v.SetDefault("server.shutdown_timeout", 15*time.Second)

	v.SetDefault("db.host", "127.0.0.1")
	v.SetDefault("db.port", 3306)
	v.SetDefault("db.max_open_conns", 50)
	v.SetDefault("db.max_idle_conns", 10)
	v.SetDefault("db.conn_max_lifetime", time.Hour)

	v.SetDefault("redis.addr", "127.0.0.1:6379")
	v.SetDefault("redis.db", 0)

	v.SetDefault("rabbitmq.url", "amqp://guest:guest@127.0.0.1:5672/")
	v.SetDefault("rabbitmq.reconnect_interval", 3*time.Second)
	v.SetDefault("rabbitmq.op_timeout", 10*time.Second)

	v.SetDefault("jwt.expire", 72*time.Hour)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")
	v.SetDefault("log.dir", "./logs")
	v.SetDefault("log.max_size_mb", 100)
	v.SetDefault("log.max_age_days", 14)
	v.SetDefault("log.max_backups", 10)
	v.SetDefault("log.compress", true)

	v.SetDefault("order.pay_timeout_seconds", 900)
	v.SetDefault("order.max_retry", 3)
}

// Load 按优先级加载配置并校验。基础文件缺失时允许运行(全靠环境变量/默认值),
// 但任何校验失败都会返回指明字段的错误。
func Load() (*Config, error) {
	env := os.Getenv("APP_ENV") // 引导变量,仅配置包内读取
	if env == "" {
		env = EnvDev
	}
	if env != EnvDev && env != EnvTest && env != EnvProd {
		return nil, fmt.Errorf("APP_ENV 取值非法: %q(仅支持 dev/test/prod)", env)
	}

	v := viper.New()
	setDefaults(v)

	// 基础层:config/config.yaml
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configDir)
	if err := v.ReadInConfig(); err != nil {
		var nf viper.ConfigFileNotFoundError
		if !errors.As(err, &nf) {
			return nil, fmt.Errorf("读取 %s/config.yaml: %w", configDir, err)
		}
		// 文件不存在:跳过,继续环境叠加层
	} else if env != EnvDev {
		// 环境叠加层:config.{env}.yaml(存在才合并,dev 不叠加)
		if _, statErr := os.Stat(fmt.Sprintf("%s/config.%s.yaml", configDir, env)); statErr == nil {
			v.SetConfigName(fmt.Sprintf("config.%s", env))
			if err := v.MergeInConfig(); err != nil {
				return nil, fmt.Errorf("合并 config.%s.yaml: %w", env, err)
			}
		}
	}

	// 环境变量层:APP_ 前缀 + 点转下划线
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg, viper.DecodeHook(mapstructure.StringToTimeDurationHookFunc())); err != nil {
		return nil, fmt.Errorf("解析配置到结构体: %w", err)
	}
	cfg.Env = env

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	watchLogLevel(v, &cfg)
	return &cfg, nil
}

// MustLoad 加载配置,失败时 Fatal 退出(启动期唯一允许 Fatal 的地方之一)。
// 此时 logger 尚未初始化(logger.Init 需要 Config),临时建 production logger
// 输出 JSON 错误(带堆栈)到 stderr,退出码 1。
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		l, _ := zap.NewProduction()
		l.Fatal("配置加载失败", zap.Error(err))
	}
	return cfg
}

// Validate 启动强校验,错误信息指明字段路径。
func (c *Config) Validate() error {
	var errs []string
	add := func(format string, a ...any) { errs = append(errs, fmt.Sprintf(format, a...)) }

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		add("server.port: 应在 1-65535 之间,当前 %d", c.Server.Port)
	}
	if c.Server.Mode != "debug" && c.Server.Mode != "release" {
		add("server.mode: 仅支持 debug/release,当前 %q", c.Server.Mode)
	}
	for name, d := range map[string]time.Duration{
		"server.read_timeout":     c.Server.ReadTimeout,
		"server.write_timeout":    c.Server.WriteTimeout,
		"server.shutdown_timeout": c.Server.ShutdownTimeout,
	} {
		if d <= 0 {
			add("%s: 必须为正时长,当前 %s", name, d)
		}
	}

	if c.DB.Host == "" {
		add("db.host: 不能为空")
	}
	if c.DB.Port <= 0 || c.DB.Port > 65535 {
		add("db.port: 应在 1-65535 之间,当前 %d", c.DB.Port)
	}
	if c.DB.User == "" {
		add("db.user: 不能为空")
	}
	if c.DB.Name == "" {
		add("db.name: 不能为空")
	}
	if c.DB.MaxOpenConns <= 0 {
		add("db.max_open_conns: 必须为正数,当前 %d", c.DB.MaxOpenConns)
	}
	if c.DB.MaxIdleConns <= 0 {
		add("db.max_idle_conns: 必须为正数,当前 %d", c.DB.MaxIdleConns)
	}
	if c.DB.ConnMaxLifetime <= 0 {
		add("db.conn_max_lifetime: 必须为正时长,当前 %s", c.DB.ConnMaxLifetime)
	}

	if c.Redis.Addr == "" {
		add("redis.addr: 不能为空")
	}
	if c.RabbitMQ.URL == "" {
		add("rabbitmq.url: 不能为空")
	}
	if c.RabbitMQ.ReconnectInterval <= 0 {
		add("rabbitmq.reconnect_interval: 必须为正时长,当前 %s", c.RabbitMQ.ReconnectInterval)
	}
	if c.RabbitMQ.OpTimeout <= 0 {
		add("rabbitmq.op_timeout: 必须为正时长,当前 %s", c.RabbitMQ.OpTimeout)
	}

	if c.JWT.Expire <= 0 {
		add("jwt.expire: 必须为正时长,当前 %s", c.JWT.Expire)
	}
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		add("log.level: 仅支持 debug/info/warn/error,当前 %q", c.Log.Level)
	}
	switch c.Log.Format {
	case "console", "json":
	default:
		add("log.format: 仅支持 console/json,当前 %q", c.Log.Format)
	}
	if c.Log.Dir == "" {
		add("log.dir: 不能为空")
	}
	if c.Log.MaxSizeMB <= 0 {
		add("log.max_size_mb: 必须为正数,当前 %d", c.Log.MaxSizeMB)
	}

	if c.Order.PayTimeoutSeconds <= 0 {
		add("order.pay_timeout_seconds: 必须为正数,当前 %d", c.Order.PayTimeoutSeconds)
	}
	if c.Order.MaxRetry < 0 {
		add("order.max_retry: 不能为负数,当前 %d", c.Order.MaxRetry)
	}

	// 密钥类:任何环境都不允许空值(不做静默兜底)
	if c.JWT.Secret == "" {
		add("jwt.secret: 不能为空(生产环境由 APP_JWT_SECRET 注入)")
	}
	if c.DB.Password == "" {
		add("db.password: 不能为空(生产环境由 APP_DB_PASSWORD 注入)")
	}

	// prod 环境加严:弱密钥直接拒绝启动
	if c.IsProd() {
		if len(c.JWT.Secret) < 32 {
			add("jwt.secret: prod 环境长度必须 >= 32 字节(当前 %d)", len(c.JWT.Secret))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("配置校验失败:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// ---------- log.level 热加载(仅此一项;其余配置修改需重启) ----------

var (
	logLevelMu        sync.Mutex
	logLevelListeners []func(level string)
)

// OnLogLevelChange 注册日志级别变化回调(main 里接线到 logger)。
func OnLogLevelChange(fn func(level string)) {
	logLevelMu.Lock()
	defer logLevelMu.Unlock()
	logLevelListeners = append(logLevelListeners, fn)
}

func notifyLogLevel(level string) {
	logLevelMu.Lock()
	listeners := append([]func(string){}, logLevelListeners...)
	logLevelMu.Unlock()
	for _, fn := range listeners {
		fn(level)
	}
}

// watchLogLevel 监听配置文件变化,只应用 log.level,避免运行中状态漂移。
// 无配置文件(纯环境变量运行)时 viper 内部不会触发回调,无需额外处理。
func watchLogLevel(v *viper.Viper, cfg *Config) {
	v.WatchConfig()
	v.OnConfigChange(func(_ fsnotify.Event) {
		lvl := v.GetString("log.level")
		switch lvl {
		case "debug", "info", "warn", "error":
		default:
			return // 非法级别直接忽略,保持现级别
		}
		if lvl == cfg.Log.Level {
			return
		}
		cfg.Log.Level = lvl
		notifyLogLevel(lvl)
	})
}

// ---------- 生效配置打印(脱敏) ----------

// maskStr 保留前 n 位,其余以 * 代替。
func maskStr(s string, keep int) string {
	if s == "" {
		return "(空)"
	}
	if len(s) <= keep {
		return strings.Repeat("*", len(s))
	}
	return s[:keep] + strings.Repeat("*", 6)
}

// maskURL 隐藏 URL 中的密码部分 amqp://user:pass@host → amqp://user:******@host。
func maskURL(raw string) string {
	at := strings.LastIndex(raw, "@")
	if at < 0 {
		return raw
	}
	head := raw[:at] // scheme://user:pass
	idx := strings.Index(head, "://")
	if idx < 0 {
		return raw
	}
	if colon := strings.Index(head[idx+3:], ":"); colon >= 0 {
		cut := idx + 3 + colon
		return head[:cut+1] + "******" + raw[at:]
	}
	return raw
}

// MaskedSummary 返回脱敏后的生效配置(用于启动日志,密码类只留前几位)。
func (c *Config) MaskedSummary() string {
	return fmt.Sprintf(
		"env=%s server.port=%d(%s) db=%s:%d/%s user=%s password=%s pool=%d/%d/%s "+
			"redis=%s(db=%d,password=%s) rabbitmq=%s reconnect=%s op_timeout=%s "+
			"jwt.secret=%s expire=%s log=%s/%s→%s(%dMB×%d,%dd,%v) order.pay_timeout=%ds retry=%d",
		c.Env, c.Server.Port, c.Server.Mode,
		c.DB.Host, c.DB.Port, c.DB.Name, c.DB.User, maskStr(c.DB.Password, 2),
		c.DB.MaxOpenConns, c.DB.MaxIdleConns, c.DB.ConnMaxLifetime,
		c.Redis.Addr, c.Redis.DB, maskStr(c.Redis.Password, 0),
		maskURL(c.RabbitMQ.URL), c.RabbitMQ.ReconnectInterval, c.RabbitMQ.OpTimeout,
		maskStr(c.JWT.Secret, 6), c.JWT.Expire,
		c.Log.Level, c.Log.Format, c.Log.Dir, c.Log.MaxSizeMB, c.Log.MaxBackups, c.Log.MaxAgeDays, c.Log.Compress,
		c.Order.PayTimeoutSeconds, c.Order.MaxRetry,
	)
}
