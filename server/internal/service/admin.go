package service

import (
	"context"

	"fawn-mall/internal/model"
	"fawn-mall/internal/repository"
	"fawn-mall/pkg/resp"
)

// AdminService 管理端业务:商品/分类增删改查、库存管理、订单查看与发货、用户管理。
type AdminService struct {
	products   *repository.ProductRepo
	categories *repository.CategoryRepo
	orders     *repository.OrderRepo
	cache      *repository.Cache
	stock      *repository.CacheStock
	users      *repository.UserRepo
	ustate     *repository.UserState
}

// NewAdminService 构造。
func NewAdminService(p *repository.ProductRepo, c *repository.CategoryRepo,
	o *repository.OrderRepo, cache *repository.Cache, stock *repository.CacheStock,
	users *repository.UserRepo, ustate *repository.UserState) *AdminService {
	return &AdminService{products: p, categories: c, orders: o, cache: cache, stock: stock,
		users: users, ustate: ustate}
}

// SKUInput 商品 SKU 入参。
type SKUInput struct {
	Size  string `json:"size" binding:"required,min=1,max=16"`
	Stock int    `json:"stock" binding:"gte=0,lte=1000000"`
}

// ProductInput 商品创建/更新入参。
type ProductInput struct {
	CategoryID  uint64     `json:"category_id" binding:"required,gt=0"`
	Name        string     `json:"name" binding:"required,min=2,max=128"`
	AgeGroup    string     `json:"age_group" binding:"required,oneof=baby little middle big"`
	Material    string     `json:"material" binding:"required,max=64"`
	Season      string     `json:"season" binding:"required,oneof=spring_summer autumn_winter"`
	Description string     `json:"description" binding:"max=2000"`
	Image       string     `json:"image" binding:"max=512"`
	Price       int64      `json:"price" binding:"required,gt=0,lt=100000000"` // 分,< 100 万元
	Status      int8       `json:"status" binding:"oneof=0 1"`
	Recommended int8       `json:"recommended" binding:"oneof=0 1"`
	SKUs        []SKUInput `json:"skus" binding:"required,min=1,max=16,dive"`
}

// CreateProduct 新建商品(含 SKU),同步库存到 Redis,失效首页缓存。
func (s *AdminService) CreateProduct(ctx context.Context, in ProductInput) (*model.ProductDetail, error) {
	if _, err := s.categories.GetByID(in.CategoryID); err != nil {
		return nil, err
	}
	p := &model.Product{
		CategoryID: in.CategoryID, Name: in.Name, AgeGroup: in.AgeGroup,
		Material: in.Material, Season: in.Season, Description: in.Description,
		Image: in.Image, Price: in.Price, Status: in.Status, Recommended: in.Recommended,
	}
	if p.Image == "" {
		p.Image = defaultImage(in.Name)
	}
	skus := make([]model.SKU, 0, len(in.SKUs))
	for _, sk := range in.SKUs {
		skus = append(skus, model.SKU{Size: sk.Size, Stock: sk.Stock})
	}
	if err := s.products.CreateWithSKUs(p, skus); err != nil {
		return nil, err
	}
	items := make([]repository.StockItem, 0, len(skus))
	for _, sk := range skus {
		items = append(items, repository.StockItem{ProductID: p.ID, Size: sk.Size, Quantity: sk.Stock})
	}
	if err := s.stock.SyncFromMySQL(ctx, items); err != nil {
		return nil, err
	}
	_ = s.cache.InvalidateHome(ctx)
	return s.detail(ctx, p.ID)
}

// UpdateProduct 更新商品(基础字段 + SKU 全量替换;同尺码保留库存)。
func (s *AdminService) UpdateProduct(ctx context.Context, id uint64, in ProductInput) error {
	p, err := s.products.GetByID(id)
	if err != nil {
		return err
	}
	if p == nil {
		return resp.ErrProductNotFound
	}
	if _, err := s.categories.GetByID(in.CategoryID); err != nil {
		return err
	}
	p.CategoryID, p.Name, p.AgeGroup = in.CategoryID, in.Name, in.AgeGroup
	p.Material, p.Season, p.Description = in.Material, in.Season, in.Description
	p.Image, p.Price, p.Status, p.Recommended = in.Image, in.Price, in.Status, in.Recommended
	if p.Image == "" {
		p.Image = defaultImage(in.Name)
	}
	if err := s.products.Update(p); err != nil {
		return err
	}
	skus := make([]model.SKU, 0, len(in.SKUs))
	for _, sk := range in.SKUs {
		skus = append(skus, model.SKU{Size: sk.Size, Stock: sk.Stock})
	}
	if err := s.products.ReplaceSKUs(id, skus); err != nil {
		return err
	}
	// 新尺码库存写入 Redis(SETNX,不覆盖既有 key 的预扣差额)
	items := make([]repository.StockItem, 0, len(skus))
	for _, sk := range skus {
		items = append(items, repository.StockItem{ProductID: id, Size: sk.Size, Quantity: sk.Stock})
	}
	if err := s.stock.SyncFromMySQL(ctx, items); err != nil {
		return err
	}
	// 主动失效详情与首页缓存
	if err := s.cache.InvalidateProduct(ctx, id); err != nil {
		return err
	}
	return s.cache.InvalidateHome(ctx)
}

// DeleteProduct 删除商品,清理其 Redis 库存 key 与缓存。
func (s *AdminService) DeleteProduct(ctx context.Context, id uint64) error {
	skus, err := s.products.DeleteWithSKUs(id)
	if err != nil {
		return err
	}
	if len(skus) == 0 {
		return resp.ErrProductNotFound
	}
	sizes := make([]string, 0, len(skus))
	for _, sk := range skus {
		sizes = append(sizes, sk.Size)
	}
	if err := s.stock.Delete(ctx, id, sizes); err != nil {
		return err
	}
	_ = s.cache.InvalidateProduct(ctx, id)
	return s.cache.InvalidateHome(ctx)
}

// AdminListProducts 管理端商品列表(默认含下架商品;status 显式传入时按其过滤)。
func (s *AdminService) AdminListProducts(ctx context.Context, f model.ProductFilter) (*PageList, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 50 {
		f.PageSize = 10
	}
	list, total, err := s.products.List(f)
	if err != nil {
		return nil, err
	}
	names := map[uint64]string{}
	if cats, cerr := s.categories.List(); cerr == nil {
		for _, c := range cats {
			names[c.ID] = c.Name
		}
	}
	type productView struct {
		model.Product
		CategoryName string `json:"category_name"`
	}
	views := make([]productView, 0, len(list))
	for _, p := range list {
		views = append(views, productView{Product: p, CategoryName: names[p.CategoryID]})
	}
	return &PageList{List: views, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// UpdateSKUStock 直接设置某 SKU 库存:MySQL 为准,Redis 强制覆盖,失效详情缓存。
func (s *AdminService) UpdateSKUStock(ctx context.Context, skuID uint64, stock int) error {
	sk, err := s.products.UpdateSKUStock(skuID, stock)
	if err != nil {
		return err
	}
	if sk == nil {
		return resp.NewBiz(resp.CodeNotFound, "SKU 不存在")
	}
	if err := s.stock.SetForcibly(ctx, sk.ProductID, sk.Size, stock); err != nil {
		return err
	}
	return s.cache.InvalidateProduct(ctx, sk.ProductID)
}

// ListCategories 分类列表。
func (s *AdminService) ListCategories(ctx context.Context) ([]model.Category, error) {
	return s.categories.List()
}

// CategoryInput 分类入参。
type CategoryInput struct {
	Name string `json:"name" binding:"required,min=1,max=32"`
	Sort int    `json:"sort" binding:"gte=0,lte=9999"`
}

// CreateCategory 新建分类。
func (s *AdminService) CreateCategory(ctx context.Context, in CategoryInput) (*model.Category, error) {
	c := &model.Category{Name: in.Name, Sort: in.Sort}
	if err := s.categories.Create(c); err != nil {
		return nil, err
	}
	_ = s.cache.InvalidateHome(ctx)
	return c, nil
}

// UpdateCategory 更新分类。
func (s *AdminService) UpdateCategory(ctx context.Context, id uint64, in CategoryInput) error {
	c := &model.Category{ID: id, Name: in.Name, Sort: in.Sort}
	if err := s.categories.Update(c); err != nil {
		return err
	}
	return s.cache.InvalidateHome(ctx)
}

// DeleteCategory 删除分类。
func (s *AdminService) DeleteCategory(ctx context.Context, id uint64) error {
	if err := s.categories.Delete(id); err != nil {
		return err
	}
	return s.cache.InvalidateHome(ctx)
}

// ListOrders 管理端订单列表。
func (s *AdminService) ListOrders(ctx context.Context, status string, page, pageSize int) (*PageList, error) {
	list, total, err := s.orders.ListAll(status, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &PageList{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

// Ship 发货:已支付 → 已发货。
func (s *AdminService) Ship(ctx context.Context, orderNo string) error {
	ok, err := s.orders.MarkShipped(orderNo)
	if err != nil {
		return err
	}
	if !ok {
		return resp.ErrOrderStatus
	}
	return nil
}

func (s *AdminService) detail(ctx context.Context, id uint64) (*model.ProductDetail, error) {
	d, err := NewProductService(s.products, s.categories, s.cache).Detail(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.cache.InvalidateProduct(ctx, id) // 新建后立即可见最新数据
	return d, nil
}

// defaultImage 未传图时生成 picsum 占位图(按名称确定性 seed)。
func defaultImage(name string) string {
	return "https://picsum.photos/seed/" + urlEscapeSeed(name) + "/600/600"
}

func urlEscapeSeed(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "fawn"
	}
	return string(out)
}
