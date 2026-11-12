// Package config 加载 config.yaml 并支持环境变量覆盖(TZ_ 前缀)。
package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config 全局配置结构,字段与 config/config.yaml 一一对应。
type Config struct {
	Server struct {
		Port int    `yaml:"port"`
		Mode string `yaml:"mode"`
	} `yaml:"server"`

	MySQL struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Database string `yaml:"database"`
	} `yaml:"mysql"`

	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`

	RabbitMQ struct {
		URL string `yaml:"url"`
	} `yaml:"rabbitmq"`

	JWT struct {
		Secret      string `yaml:"secret"`
		ExpireHours int    `yaml:"expire_hours"`
	} `yaml:"jwt"`

	Order struct {
		PayTimeoutSeconds int `yaml:"pay_timeout_seconds"`
		MaxRetry          int `yaml:"max_retry"`
	} `yaml:"order"`

	Log struct {
		Level string `yaml:"level"`
	} `yaml:"log"`
}

// Load 读取指定路径的 YAML 配置,应用默认值,再用 TZ_ 前缀环境变量覆盖。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件 %s: %w", path, err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件: %w", err)
	}
	applyDefaults(cfg)
	applyEnv(cfg)
	return cfg, nil
}

func applyDefaults(c *Config) {
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.Mode == "" {
		c.Server.Mode = "debug"
	}
	if c.JWT.ExpireHours == 0 {
		c.JWT.ExpireHours = 72
	}
	if c.JWT.Secret == "" {
		c.JWT.Secret = "fawn-jwt-dev-secret-change-me"
	}
	if c.Order.PayTimeoutSeconds == 0 {
		c.Order.PayTimeoutSeconds = 900
	}
	if c.Order.MaxRetry == 0 {
		c.Order.MaxRetry = 3
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
}

// applyEnv 环境变量覆盖。显式列出全部键,避免反射魔法难以排查。
func applyEnv(c *Config) {
	overrideStr("TZ_SERVER_MODE", &c.Server.Mode)
	overrideInt("TZ_SERVER_PORT", &c.Server.Port)

	overrideStr("TZ_MYSQL_HOST", &c.MySQL.Host)
	overrideInt("TZ_MYSQL_PORT", &c.MySQL.Port)
	overrideStr("TZ_MYSQL_USER", &c.MySQL.User)
	overrideStr("TZ_MYSQL_PASSWORD", &c.MySQL.Password)
	overrideStr("TZ_MYSQL_DATABASE", &c.MySQL.Database)

	overrideStr("TZ_REDIS_ADDR", &c.Redis.Addr)
	overrideStr("TZ_REDIS_PASSWORD", &c.Redis.Password)
	overrideInt("TZ_REDIS_DB", &c.Redis.DB)

	overrideStr("TZ_RABBITMQ_URL", &c.RabbitMQ.URL)

	overrideStr("TZ_JWT_SECRET", &c.JWT.Secret)
	overrideInt("TZ_JWT_EXPIRE_HOURS", &c.JWT.ExpireHours)

	overrideInt("TZ_ORDER_PAY_TIMEOUT_SECONDS", &c.Order.PayTimeoutSeconds)
	overrideInt("TZ_ORDER_MAX_RETRY", &c.Order.MaxRetry)

	overrideStr("TZ_LOG_LEVEL", &c.Log.Level)
}

func overrideStr(key string, dst *string) {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		*dst = v
	}
}

func overrideInt(key string, dst *int) {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			*dst = n
		}
	}
}

// DSN 返回 MySQL 连接串(GORM)。
func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.MySQL.User, c.MySQL.Password, c.MySQL.Host, c.MySQL.Port, c.MySQL.Database)
}
