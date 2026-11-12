// Package testutil 测试基础设施:连接真实 MySQL / Redis / RabbitMQ(禁止 mock 依赖)。
// 环境变量与运行时配置一致(TZ_ 前缀),默认指向本地 docker compose 起的服务;
// CI 中 GitHub Actions services 同样监听 localhost,无需额外配置。
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"

	"fawn-mall/internal/model"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var (
	once  sync.Once
	db    *gorm.DB
	rdb   *redis.Client
	dbErr error
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// MySQLDSN 测试库 DSN(默认 fawn_mall_test,与开发库隔离;
// 端口默认 3316 与 docker-compose 宿主映射一致,CI 中通过环境变量覆盖)。
func MySQLDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		env("TZ_MYSQL_USER", "fawn"),
		env("TZ_MYSQL_PASSWORD", "fawn_dev_123"),
		env("TZ_MYSQL_HOST", "127.0.0.1"),
		env("TZ_MYSQL_PORT", "3316"),
		env("TZ_MYSQL_DATABASE", "fawn_mall_test"),
	)
}

// RedisClient 返回测试 Redis 客户端(默认 DB 1,与开发 DB 0 隔离;
// 地址默认 127.0.0.1:6381 与 docker-compose 宿主映射一致,CI 中覆盖)。
func RedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     env("TZ_REDIS_ADDR", "127.0.0.1:6381"),
		Password: env("TZ_REDIS_PASSWORD", ""),
		DB:       1,
	})
}

// RabbitMQURL 测试 RabbitMQ 地址(默认 5673 与 docker-compose 宿主映射一致,CI 中覆盖)。
func RabbitMQURL() string {
	return env("TZ_RABBITMQ_URL", "amqp://fawn:fawn_dev_123@127.0.0.1:5673/")
}

// Setup 返回连接真实 MySQL 与 Redis 的测试基础设施(懒加载单例,AutoMigrate)。
// 首次运行会用 root 账号自动创建测试库并授权(幂等),与 CI 流程一致。
func Setup(t *testing.T) (*gorm.DB, *redis.Client) {
	t.Helper()
	once.Do(func() {
		rdb = RedisClient()
		if err := rdb.Ping(context.Background()).Err(); err != nil {
			dbErr = fmt.Errorf("测试需要真实 Redis(先 task infra):%w", err)
			return
		}
		provisionTestDB()
		db, dbErr = gorm.Open(mysql.Open(MySQLDSN()), &gorm.Config{
			Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		})
		if dbErr != nil {
			return
		}
		dbErr = db.AutoMigrate(
			&model.User{}, &model.Category{}, &model.Product{}, &model.SKU{},
			&model.Order{}, &model.OrderItem{},
		)
	})
	if dbErr != nil {
		t.Fatalf("测试基础设施初始化失败: %v", dbErr)
	}
	return db, rdb
}

// provisionTestDB 用 root 连接创建测试库(若不存在)并授权应用账号,幂等。
func provisionTestDB() {
	rootDSN := fmt.Sprintf("root:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		env("TZ_MYSQL_ROOT_PASSWORD", "fawn_root_dev"),
		env("TZ_MYSQL_HOST", "127.0.0.1"),
		env("TZ_MYSQL_PORT", "3316"),
	)
	root, err := sql.Open("mysql", rootDSN)
	if err != nil {
		return
	}
	defer root.Close()
	user := env("TZ_MYSQL_USER", "fawn")
	testDB := env("TZ_MYSQL_DATABASE", "fawn_mall_test")
	_, _ = root.Exec("CREATE DATABASE IF NOT EXISTS " + testDB + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	_, _ = root.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO '%s'@'%%'", testDB, user))
}

// NewProduct 创建测试商品与 SKU,并注册自动清理。
func NewProduct(t *testing.T, db *gorm.DB, name string, skus []model.SKU) model.Product {
	t.Helper()
	p := model.Product{
		CategoryID: 1, Name: name, AgeGroup: model.AgeLittle,
		Material: "测试棉", Season: model.SeasonSpringSummer,
		Description: "测试商品", Image: "https://picsum.photos/seed/test/600/600",
		Price: 9900, Status: model.ProductStatusOn,
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("创建测试商品失败: %v", err)
	}
	for i := range skus {
		skus[i].ProductID = p.ID
		if err := db.Create(&skus[i]).Error; err != nil {
			t.Fatalf("创建测试 SKU 失败: %v", err)
		}
	}
	t.Cleanup(func() {
		db.Where("product_id = ?", p.ID).Delete(&model.SKU{})
		db.Delete(&model.Product{}, p.ID)
	})
	return p
}
