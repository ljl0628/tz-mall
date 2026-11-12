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

	"github.com/redis/go-redis/v9"

	"fawn-mall/internal/handler"
	"fawn-mall/internal/model"
	"fawn-mall/internal/mq"
	"fawn-mall/internal/repository"
	"fawn-mall/internal/seed"
	"fawn-mall/internal/service"
	"fawn-mall/pkg/config"
	"fawn-mall/pkg/jwtx"
	"fawn-mall/pkg/logger"
)

func main() {
	// 1. 配置:viper 加载(环境变量 > config.{env}.yaml > config.yaml > 默认值),启动即强校验
	cfg := config.MustLoad()
	logger.Init(cfg.Log.Level)
	logger.Info("配置加载完成(密码已脱敏)", "effective", cfg.MaskedSummary())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 2. MySQL:连接 + AutoMigrate + 种子数据
	db, err := repository.NewDB(cfg)
	if err != nil {
		logger.Error("MySQL 初始化失败", "err", err)
		os.Exit(1)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Category{}, &model.Product{}, &model.SKU{},
		&model.Order{}, &model.OrderItem{},
	); err != nil {
		logger.Error("AutoMigrate 失败", "err", err)
		os.Exit(1)
	}
	logger.Info("MySQL 已连接,表结构已同步")

	userRepo := repository.NewUserRepo(db)
	if err := seed.Seed(db, userRepo); err != nil {
		logger.Error("种子数据写入失败", "err", err)
		os.Exit(1)
	}

	// 3. Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("Redis 连接失败", "err", err)
		os.Exit(1)
	}
	logger.Info("Redis 已连接", "addr", cfg.Redis.Addr)

	// 4. 仓储与服务组装
	categoryRepo := repository.NewCategoryRepo(db)
	productRepo := repository.NewProductRepo(db)
	orderRepo := repository.NewOrderRepo(db)
	cartRepo := repository.NewCartRepo(rdb)
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
	orderSvc := service.NewOrderService(orderRepo, cartRepo, productRepo, stockRepo, mqClient)
	adminSvc := service.NewAdminService(productRepo, categoryRepo, orderRepo, cacheRepo, stockRepo, userRepo, userState)

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
		Admin:   handler.NewAdminHandler(adminSvc),
	})
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}
	go func() {
		logger.Info("HTTP 服务启动", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP 服务异常退出", "err", err)
			stop()
		}
	}()

	// 8. 优雅关停(预算来自 server.shutdown_timeout:HTTP → MQ → Redis → MySQL 依次关闭)
	<-ctx.Done()
	logger.Info("收到退出信号,开始优雅关停")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP 关停异常", "err", err)
	}
	mqClient.Close()
	if err := rdb.Close(); err != nil {
		logger.Warn("Redis 关闭异常", "err", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	logger.Info("已退出")
}

// syncStockToRedis 启动时把 MySQL 库存灌入 Redis(仅当 key 不存在)。
func syncStockToRedis(ctx context.Context, productRepo *repository.ProductRepo, stockRepo *repository.CacheStock) {
	skus, err := productRepo.AllSKUs()
	if err != nil {
		logger.Error("读取 SKU 失败,跳过库存同步", "err", err)
		return
	}
	items := make([]repository.StockItem, 0, len(skus))
	for _, sk := range skus {
		items = append(items, repository.StockItem{ProductID: sk.ProductID, Size: sk.Size, Quantity: sk.Stock})
	}
	if err := stockRepo.SyncFromMySQL(ctx, items); err != nil {
		logger.Error("库存同步到 Redis 失败", "err", err)
		return
	}
	logger.Info("库存已同步到 Redis(SETNX 语义)", "skus", len(items))
}
