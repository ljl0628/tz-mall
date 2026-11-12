// Package migrations 内嵌 SQL 迁移文件(golang-migrate 格式:NNNNNN_name.up.sql / .down.sql)。
// 生产环境由 pkg/dbmigrate 在启动时执行;开发/测试环境沿用 GORM AutoMigrate。
package migrations

import "embed"

// FS 内嵌全部迁移脚本。
//
//go:embed *.sql
var FS embed.FS
