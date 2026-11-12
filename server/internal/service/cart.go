package service

import (
	"context"
	"fmt"

	"fawn-mall/internal/model"
	"fawn-mall/internal/repository"
	"fawn-mall/pkg/resp"
)

// 单个购物项数量上限。
const cartMaxQty = 99

// CartService 购物车业务(Redis Hash 存储,同款不同码为不同购物项)。
type CartService struct {
	cart     *repository.CartRepo
	products *repository.ProductRepo
	cache    *repository.Cache
}

// NewCartService 构造。
func NewCartService(cart *repository.CartRepo, products *repository.ProductRepo, cache *repository.Cache) *CartService {
	return &CartService{cart: cart, products: products, cache: cache}
}

// AddReq 加购请求。
type AddReq struct {
	ProductID uint64 `json:"product_id" binding:"required,gt=0"`
	Size      string `json:"size" binding:"required,min=1,max=16"`
	Quantity  int    `json:"quantity" binding:"required,gt=0,lte=99"`
}

// Add 加购:校验商品在售与尺码存在,数量累加。
func (s *CartService) Add(ctx context.Context, userID uint64, req AddReq) error {
	p, err := s.products.GetByID(req.ProductID)
	if err != nil {
		return err
	}
	if p == nil || p.Status != model.ProductStatusOn {
		return resp.ErrProductNotFound
	}
	sku, err := s.products.GetSKUByProductAndSize(req.ProductID, req.Size)
	if err != nil {
		return err
	}
	if sku == nil {
		return resp.NewBiz(resp.CodeParamError, fmt.Sprintf("尺码 %s 不存在", req.Size))
	}
	_, err = s.cart.Add(ctx, userID, repository.CartItem{
		ProductID: req.ProductID,
		Size:      req.Size,
		Quantity:  req.Quantity,
		Selected:  true, // 新加购默认勾选
		Name:      p.Name,
		Image:     p.Image,
		Price:     p.Price,
		AgeGroup:  p.AgeGroup,
	}, cartMaxQty)
	return err
}

// CartItemView 购物车项视图(合并商品实时信息:价格可能调整、商品可能下架)。
type CartItemView struct {
	ProductID uint64 `json:"product_id"`
	Size      string `json:"size"`
	Quantity  int    `json:"quantity"`
	Selected  bool   `json:"selected"`
	Name      string `json:"name"`
	Image     string `json:"image"`
	Price     int64  `json:"price"`
	AgeGroup  string `json:"age_group"`
	Stock     int    `json:"stock"` // 当前尺码实时库存
	OffShelf  bool   `json:"off_shelf"`
}

// CartSummary 合计信息。
type CartSummary struct {
	Count            int   `json:"count"`             // 购物项数
	TotalQuantity    int   `json:"total_quantity"`    // 总件数
	SelectedAmount   int64 `json:"selected_amount"`   // 勾选商品金额(分)
	SelectedQuantity int   `json:"selected_quantity"` // 勾选件数
}

// CartView 购物车整体视图。
type CartView struct {
	Items   []CartItemView `json:"items"`
	Summary CartSummary    `json:"summary"`
}

// Get 查询购物车(合并商品实时信息)。
func (s *CartService) Get(ctx context.Context, userID uint64) (*CartView, error) {
	items, err := s.cart.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	view := &CartView{Items: make([]CartItemView, 0, len(items))}
	for _, it := range items {
		v := CartItemView{
			ProductID: it.ProductID, Size: it.Size,
			Quantity: it.Quantity, Selected: it.Selected,
			Name: it.Name, Image: it.Image, Price: it.Price, AgeGroup: it.AgeGroup,
		}
		if p, err := s.products.GetByID(it.ProductID); err == nil && p != nil {
			v.Name, v.Image, v.Price, v.AgeGroup = p.Name, p.Image, p.Price, p.AgeGroup
			v.OffShelf = p.Status != model.ProductStatusOn
			if skus, err := s.products.GetSKUs(it.ProductID); err == nil {
				for _, sk := range skus {
					if sk.Size == it.Size {
						v.Stock = sk.Stock
						break
					}
				}
			}
		} else {
			v.OffShelf = true
		}
		view.Items = append(view.Items, v)

		view.Summary.Count++
		view.Summary.TotalQuantity += it.Quantity
		if it.Selected {
			view.Summary.SelectedQuantity += it.Quantity
			view.Summary.SelectedAmount += v.Price * int64(it.Quantity)
		}
	}
	return view, nil
}

// UpdateReq 修改购物车项(数量与勾选,均可选)。
type UpdateReq struct {
	Quantity *int  `json:"quantity" binding:"omitempty,gte=1,lte=99"`
	Selected *bool `json:"selected"`
}

// Update 修改数量/勾选。field 格式为 "{productID}:{size}"。
func (s *CartService) Update(ctx context.Context, userID uint64, field string, req UpdateReq) error {
	pid, size, ok := repository.CartKeyItem(field)
	if !ok {
		return resp.NewBiz(resp.CodeParamError, "购物车项标识非法")
	}
	it, err := s.cart.Get(ctx, userID, pid, size)
	if err != nil {
		return err
	}
	if it == nil {
		return resp.ErrCartItem
	}
	if req.Quantity != nil {
		if err := s.cart.UpdateQuantity(ctx, userID, pid, size, *req.Quantity); err != nil {
			return err
		}
	}
	if req.Selected != nil {
		if err := s.cart.UpdateSelected(ctx, userID, pid, size, *req.Selected); err != nil {
			return err
		}
	}
	return nil
}

// Remove 删除购物车项。
func (s *CartService) Remove(ctx context.Context, userID uint64, field string) error {
	pid, size, ok := repository.CartKeyItem(field)
	if !ok {
		return resp.NewBiz(resp.CodeParamError, "购物车项标识非法")
	}
	return s.cart.Remove(ctx, userID, pid, size)
}

// SelectAll 全选/取消全选。
func (s *CartService) SelectAll(ctx context.Context, userID uint64, selected bool) error {
	return s.cart.UpdateAllSelected(ctx, userID, selected)
}

// Count 购物车项数(角标)。
func (s *CartService) Count(ctx context.Context, userID uint64) (int, error) {
	return s.cart.Count(ctx, userID)
}
