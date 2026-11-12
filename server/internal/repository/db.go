package repository

import (
	"fmt"

	"fawn-mall/pkg/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// NewDB 建立 GORM(MySQL)连接,连接池参数来自配置(db.max_* 系列)。
func NewDB(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(cfg.DB.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.DB.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.DB.ConnMaxLifetime)
	return db, nil
}
