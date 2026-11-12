package handler

import (
	"github.com/gin-gonic/gin"

	"fawn-mall/internal/middleware"
	"fawn-mall/pkg/config"
	"fawn-mall/pkg/jwtx"
)

// RouterDeps 路由依赖。
type RouterDeps struct {
	Cfg   *config.Config
	JWT   *jwtx.Manager
	Black middleware.TokenBlacklist
	Guard middleware.UserGuard

	Auth    *AuthHandler
	Product *ProductHandler
	Cart    *CartHandler
	Order   *OrderHandler
	Address *AddressHandler
	Admin   *AdminHandler
	Finance *FinanceHandler
}

// NewRouter 组装全部路由。
func NewRouter(d RouterDeps) *gin.Engine {
	if d.Cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	// Recovery 最外层兜 panic;RequestID → AccessLog 顺序保证访问日志携带 request_id
	r.Use(middleware.Recovery(), middleware.RequestID(), middleware.AccessLog(), middleware.CORS())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 调试端点:仅 debug 模式注册,用于验收 Recovery 中间件
	// (统一 50001 + zap.Stack 错误日志 + 进程不退出),release 模式不存在。
	if d.Cfg.Server.Mode == "debug" {
		r.GET("/debug/panic", func(c *gin.Context) {
			panic("手动触发 panic(调试端点,验证 Recovery 中间件)")
		})
	}

	api := r.Group("/api/v1")

	// 公开接口
	auth := api.Group("/auth")
	{
		auth.POST("/register", d.Auth.Register)
		auth.POST("/login", d.Auth.Login)
	}
	api.GET("/home", d.Product.Home)
	api.GET("/categories", d.Product.Categories)
	api.GET("/products", d.Product.List)
	api.GET("/products/:id", d.Product.Detail)

	// 登录用户接口
	user := api.Group("", middleware.JWTAuth(d.JWT, d.Black, d.Guard))
	{
		user.POST("/auth/logout", d.Auth.Logout)
		user.GET("/user/me", d.Auth.Me)

		user.GET("/cart", d.Cart.Get)
		user.POST("/cart/items", d.Cart.Add)
		user.PUT("/cart/items/:field", d.Cart.Update)
		user.DELETE("/cart/items/:field", d.Cart.Remove)
		user.PUT("/cart/select-all", d.Cart.SelectAll)

		user.POST("/orders", d.Order.Place)
		user.GET("/orders", d.Order.List)
		user.GET("/orders/:orderNo", d.Order.Detail)
		user.POST("/orders/:orderNo/pay", d.Order.Pay)
		user.POST("/orders/:orderNo/cancel", d.Order.Cancel)
		user.POST("/orders/:orderNo/confirm", d.Order.Confirm)
		user.POST("/orders/:orderNo/refund-apply", d.Order.RefundApply)

		// 收货地址簿(仅本人数据)
		user.GET("/user/addresses", d.Address.List)
		user.POST("/user/addresses", d.Address.Create)
		user.PUT("/user/addresses/:id", d.Address.Update)
		user.PUT("/user/addresses/:id/default", d.Address.SetDefault)
		user.DELETE("/user/addresses/:id", d.Address.Delete)
	}

	// 管理端接口(管理员角色)
	admin := api.Group("/admin", middleware.JWTAuth(d.JWT, d.Black, d.Guard), middleware.AdminRequired())
	{
		admin.GET("/me", d.Admin.Me)
		admin.POST("/upload/image", d.Admin.UploadImage)
		admin.GET("/products", d.Admin.ListProducts)
		admin.POST("/products", d.Admin.CreateProduct)
		admin.PUT("/products/:id", d.Admin.UpdateProduct)
		admin.DELETE("/products/:id", d.Admin.DeleteProduct)
		admin.PUT("/skus/:id", d.Admin.UpdateSKUStock)

		admin.GET("/categories", d.Admin.ListCategories)
		admin.POST("/categories", d.Admin.CreateCategory)
		admin.PUT("/categories/:id", d.Admin.UpdateCategory)
		admin.DELETE("/categories/:id", d.Admin.DeleteCategory)

		admin.GET("/orders", d.Admin.ListOrders)
		admin.PUT("/orders/:orderNo/ship", d.Admin.Ship)

		admin.GET("/users", d.Admin.ListUsers)
		admin.PUT("/users/:id/status", d.Admin.SetUserStatus)
		admin.PUT("/users/:id/role", d.Admin.SetUserRole)
		admin.PUT("/users/:id/password", d.Admin.ResetUserPassword)

		// 财务中心:看板 / 流水 / 退款售后 / 对账
		fin := admin.Group("/finance")
		{
			fin.GET("/dashboard", d.Finance.Dashboard)
			fin.GET("/flows", d.Finance.Flows)
			fin.GET("/trend", d.Finance.Trend)
			fin.GET("/refunds", d.Finance.RefundList)
			fin.POST("/refunds", d.Finance.AdminRefund)
			fin.POST("/refunds/:refundNo/process", d.Finance.ProcessRefund)
			fin.GET("/statements", d.Finance.Statements)
			fin.POST("/statements/run", d.Finance.RunStatements)
			fin.POST("/reconcile", d.Finance.Reconcile)
		}
	}

	return r
}
