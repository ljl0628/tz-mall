// 小鹿童装 Fawn Mall 后端入口:
// 配置加载 → 日志 → MySQL(AutoMigrate + 种子)→ Redis → RabbitMQ(拓扑 + 消费者)→ HTTP。
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"fawn-mall/internal/handler"
	"fawn-mall/internal/model"
	"fawn-mall/internal/mq"
	"fawn-mall/internal/repository"
	"fawn-mall/internal/seed"
	"fawn-mall/internal/service"
	"fawn-mall/pkg/config"
	"fawn-mall/pkg/dbmigrate"
	"fawn-mall/pkg/jwtx"
	"fawn-mall/pkg/logger"
	"fawn-mall/pkg/storage"
)

// migrateSchema 表结构同步:生产执行 golang-migrate 版本化迁移,其余环境 AutoMigrate。
func migrateSchema(ctx context.Context, cfg *config.Config, db *gorm.DB) error {
	if cfg.IsProd() {
		logger.Info(ctx, "生产环境:执行 golang-migrate 版本化迁移")
		return dbmigrate.Up(ctx, cfg.DSN())
	}
	return db.AutoMigrate(
		&model.User{}, &model.Category{}, &model.Product{}, &model.SKU{},
		&model.Order{}, &model.OrderItem{}, &model.Address{},
	)
}

func main() {
	// 1. 配置:viper 加载(环境变量 > config.{env}.yaml > config.yaml > 默认值),启动即强校验
	cfg := config.MustLoad()
	logger.Init(logger.Options{
		Service: "fawn-mall", Env: cfg.Env,
		Level: cfg.Log.Level, Format: cfg.Log.Format, Dir: cfg.Log.Dir,
		MaxSizeMB: cfg.Log.MaxSizeMB, MaxAgeDays: cfg.Log.MaxAgeDays,
		MaxBackups: cfg.Log.MaxBackups, Compress: cfg.Log.Compress,
	})
	// 日志级别热加载:配置文件变更仅即时作用于 log.level,其余配置重启生效
	config.OnLogLevelChange(logger.SetLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger.Info(ctx, "配置加载完成(密码已脱敏)", zap.String("effective", cfg.MaskedSummary()))

	// 2. MySQL:连接 + 表结构同步 + 种子数据
	// 生产走 golang-migrate 版本化迁移(server/migrations,可追溯可回滚);
	// 开发/测试沿用 GORM AutoMigrate,本地零依赖起步。
	db, err := repository.NewDB(cfg)
	if err != nil {
		logger.Error(ctx, "MySQL 初始化失败", zap.Error(err))
		os.Exit(1)
	}
	if err := migrateSchema(ctx, cfg, db); err != nil {
		logger.Error(ctx, "表结构迁移失败", zap.Error(err))
		os.Exit(1)
	}
	logger.Info(ctx, "MySQL 已连接,表结构已同步")

	userRepo := repository.NewUserRepo(db)
	if err := seed.Seed(db, userRepo); err != nil {
		logger.Error(ctx, "种子数据写入失败", zap.Error(err))
		os.Exit(1)
	}

	// 3. Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error(ctx, "Redis 连接失败", zap.Error(err))
		os.Exit(1)
	}
	logger.Info(ctx, "Redis 已连接", zap.String("addr", cfg.Redis.Addr))

	// 4. 仓储与服务组装
	categoryRepo := repository.NewCategoryRepo(db)
	productRepo := repository.NewProductRepo(db)
	orderRepo := repository.NewOrderRepo(db)
	cartRepo := repository.NewCartRepo(rdb)
	addressRepo := repository.NewAddressRepo(db)
	cacheRepo := repository.NewCache(rdb)
	stockRepo := repository.NewCacheStock(rdb)
	blackRepo := repository.NewBlacklist(rdb)

	jwtMgr := jwtx.New(cfg.JWT.Secret, cfg.JWT.Expire)
	userState := repository.NewUserState(rdb, jwtMgr.ExpireDuration())
	mqClient := mq.New(cfg.RabbitMQ.URL, cfg.Order.PayTimeoutSeconds, cfg.Order.MaxRetry,
		cfg.RabbitMQ.ReconnectInterval, cfg.RabbitMQ.OpTimeout)

	authSvc := service.NewAuthService(userRepo, blackRepo, jwtMgr)
	productSvc := service.NewProductService(productRepo, categoryRepo, cacheRepo)
	cartSvc := service.NewCartService(cartRepo, productRepo, cacheRepo)
	payTimeout := time.Duration(cfg.Order.PayTimeoutSeconds) * time.Second
	orderSvc := service.NewOrderService(orderRepo, cartRepo, productRepo, stockRepo, mqClient, payTimeout)
	addressSvc := service.NewAddressService(addressRepo)
	adminSvc := service.NewAdminService(productRepo, categoryRepo, orderRepo, cacheRepo, stockRepo, userRepo, userState)

	// 对象存储(可选):配置了 qiniu.* 才启用商品图上传
	var uploader storage.Uploader
	if cfg.Qiniu.AccessKey != "" {
		uploader, err = storage.NewQiniu(cfg.Qiniu)
		if err != nil {
			logger.Error(ctx, "七牛对象存储初始化失败", zap.Error(err))
			os.Exit(1)
		}
		logger.Info(ctx, "七牛对象存储已启用", zap.String("bucket", cfg.Qiniu.Bucket), zap.String("domain", cfg.Qiniu.Domain))
	}

	// 5. MQ 消费者注册 + 后台运行(自动重连)
	mqClient.RegisterHandler(mq.QueueOrderCreate, orderSvc.HandleOrderCreate)
	mqClient.RegisterHandler(mq.QueueOrderCancel, orderSvc.HandleOrderCancel)
	go mqClient.Run(ctx)

	// 6. 启动时同步 MySQL 库存 → Redis(SETNX,不覆盖运行中差额)
	syncStockToRedis(ctx, productRepo, stockRepo)

	// 7. HTTP 服务
	router := handler.NewRouter(handler.RouterDeps{
		Cfg:     cfg,
		JWT:     jwtMgr,
		Black:   blackRepo,
		Guard:   userState,
		Auth:    handler.NewAuthHandler(authSvc),
		Product: handler.NewProductHandler(productSvc),
		Cart:    handler.NewCartHandler(cartSvc),
		Order:   handler.NewOrderHandler(orderSvc),
		Address: handler.NewAddressHandler(addressSvc),
		Admin:   handler.NewAdminHandler(adminSvc, uploader),
	})
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}
	go func() {
		logger.Info(ctx, "HTTP 服务启动", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(ctx, "HTTP 服务异常退出", zap.Error(err))
			stop()
		}
	}()

	// 8. 优雅关停(预算来自 server.shutdown_timeout:HTTP → MQ → Redis → MySQL 依次关闭)
	<-ctx.Done()
	logger.Info(ctx, "收到退出信号,开始优雅关停")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error(ctx, "HTTP 关停异常", zap.Error(err))
	}
	mqClient.Close()
	if err := rdb.Close(); err != nil {
		logger.Warn(ctx, "Redis 关闭异常", zap.Error(err))
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	logger.Sync()
	logger.Info(context.Background(), "shutdown: 进程已优雅退出")
}

// syncStockToRedis 启动时把 MySQL 库存灌入 Redis(仅当 key 不存在)。
func syncStockToRedis(ctx context.Context, productRepo *repository.ProductRepo, stockRepo *repository.CacheStock) {
	skus, err := productRepo.AllSKUs()
	if err != nil {
		logger.Error(ctx, "读取 SKU 失败,跳过库存同步", zap.Error(err))
		return
	}
	items := make([]repository.StockItem, 0, len(skus))
	for _, sk := range skus {
		items = append(items, repository.StockItem{ProductID: sk.ProductID, Size: sk.Size, Quantity: sk.Stock})
	}
	if err := stockRepo.SyncFromMySQL(ctx, items); err != nil {
		logger.Error(ctx, "库存同步到 Redis 失败", zap.Error(err))
		return
	}
	logger.Info(ctx, "库存已同步到 Redis(SETNX 语义)", zap.Int("skus", len(items)))
}
