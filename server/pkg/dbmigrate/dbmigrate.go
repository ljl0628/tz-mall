// Package dbmigrate 生产库版本化迁移(golang-migrate + 内嵌 SQL)。
//
// 与 GORM AutoMigrate 的分工:
//   - prod:启动时执行 migrations/ 下的 SQL(带 schema_migrations 版本表,可追溯/可回滚)
//   - dev/test:沿用 AutoMigrate(见 cmd/server/main.go),保持本地零依赖
package dbmigrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	mysqldriver "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/zap"

	"fawn-mall/migrations"
	"fawn-mall/pkg/logger"
)

// Up 升级到最新版本。dsn 为 GORM 风格连接串(user:pass@tcp(host:port)/db?...),
// mysql 驱动要求 multiStatements 以支持单文件多条语句,缺失时自动追加。
// ErrNoChange(已在最新版)视为成功。
func Up(ctx context.Context, dsn string) error {
	if !strings.Contains(dsn, "multiStatements") {
		dsn += "&multiStatements=true"
	}
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("加载迁移文件: %w", err)
	}
	defer src.Close()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("打开迁移连接: %w", err)
	}
	defer db.Close()

	driver, err := mysqldriver.WithInstance(db, &mysqldriver.Config{})
	if err != nil {
		return fmt.Errorf("初始化迁移驱动: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "mysql", driver)
	if err != nil {
		return fmt.Errorf("初始化迁移器: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("迁移执行失败: %w", err)
	}
	version, dirty, _ := m.Version()
	logger.Info(ctx, "数据库迁移完成", zap.Uint("version", version), zap.Bool("dirty", dirty))
	return nil
}
