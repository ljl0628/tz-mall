package service

import (
	"context"

	"fawn-mall/internal/model"
	"fawn-mall/internal/repository"
	"fawn-mall/pkg/resp"
)

// ProductService 商品业务:列表/详情(缓存)/首页聚合 + 管理端 CRUD。
type ProductService struct {
	products   *repository.ProductRepo
	categories *repository.CategoryRepo
	cache      *repository.Cache
}

// NewProductService 构造。
func NewProductService(p *repository.ProductRepo, c *repository.CategoryRepo, cache *repository.Cache) *ProductService {
	return &ProductService{products: p, categories: c, cache: cache}
}

// PageList 分页商品列表(附带分类名)。
type PageList struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// List 商品列表:分页 + 分类 + 年龄段 + 关键词(C 端,仅上架)。
func (s *ProductService) List(ctx context.Context, f model.ProductFilter) (*PageList, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 50 {
		f.PageSize = 10
	}
	f.OnShelfOnly = true
	list, total, err := s.products.List(f)
	if err != nil {
		return nil, err
	}
	// 附带分类名(单页商品最多 50,一次全量分类映射即可)
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

// Detail 商品详情(cache-aside:先查 Redis,未命中查 MySQL 并回填,TTL 30 分钟)。
// 管理端更新/删除商品时主动 DEL 缓存,详见 repository/cache.go 头注释。
func (s *ProductService) Detail(ctx context.Context, id uint64) (*model.ProductDetail, error) {
	if d, err := s.cache.GetProductDetail(ctx, id); err == nil && d != nil {
		return d, nil
	}
	p, err := s.products.GetByID(id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, resp.ErrProductNotFound
	}
	skus, err := s.products.GetSKUs(id)
	if err != nil {
		return nil, err
	}
	cats, _ := s.categories.List()
	catName := ""
	for _, c := range cats {
		if c.ID == p.CategoryID {
			catName = c.Name
			break
		}
	}
	d := &model.ProductDetail{Product: *p, CategoryName: catName, SKUs: skus}
	if err := s.cache.SetProductDetail(ctx, d); err != nil {
		// 缓存写失败不影响主流程
		_ = err
	}
	return d, nil
}

// HomePayload 首页聚合数据。
type HomePayload struct {
	Categories []model.Category `json:"categories"`
	AgeGroups  []struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"age_groups"`
	Recommended []model.Product `json:"recommended"`
}

// Home 首页数据(缓存 10 分钟,商品/分类变更时主动失效)。
func (s *ProductService) Home(ctx context.Context) (*HomePayload, error) {
	var out HomePayload
	if ok, err := s.cache.GetHome(ctx, &out); err == nil && ok {
		return &out, nil
	}
	cats, err := s.categories.List()
	if err != nil {
		return nil, err
	}
	rec, err := s.products.Recommended(8)
	if err != nil {
		return nil, err
	}
	out = HomePayload{Categories: cats, AgeGroups: model.AgeGroups, Recommended: rec}
	_ = s.cache.SetHome(ctx, out)
	return &out, nil
}

// Categories 全部分类。
func (s *ProductService) Categories(ctx context.Context) ([]model.Category, error) {
	return s.categories.List()
}
