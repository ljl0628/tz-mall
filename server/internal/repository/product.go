package repository

import (
	"fawn-mall/internal/model"

	"gorm.io/gorm"
)

// ProductRepo 商品与 SKU 数据访问。
type ProductRepo struct{ db *gorm.DB }

// NewProductRepo 构造。
func NewProductRepo(db *gorm.DB) *ProductRepo { return &ProductRepo{db: db} }

// List 分页查询商品列表,支持分类/年龄段/关键词/状态筛选。
func (r *ProductRepo) List(f model.ProductFilter) ([]model.Product, int64, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 50 {
		f.PageSize = 10
	}
	q := r.db.Model(&model.Product{})
	if f.CategoryID > 0 {
		q = q.Where("category_id = ?", f.CategoryID)
	}
	if f.AgeGroup != "" {
		q = q.Where("age_group = ?", f.AgeGroup)
	}
	if f.Keyword != "" {
		q = q.Where("name LIKE ?", "%"+f.Keyword+"%")
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	} else if f.OnShelfOnly {
		q = q.Where("status = ?", model.ProductStatusOn) // C 端默认只看上架
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.Product
	err := q.Order("id DESC").Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Find(&list).Error
	return list, total, err
}

// Recommended 推荐商品(首页)。
func (r *ProductRepo) Recommended(limit int) ([]model.Product, error) {
	if limit <= 0 {
		limit = 8
	}
	var list []model.Product
	err := r.db.Where("status = ? AND recommended = ?", model.ProductStatusOn, 1).
		Order("id DESC").Limit(limit).Find(&list).Error
	return list, err
}

// GetByID 按 ID 查询商品(不限状态,业务层判断是否上架)。
func (r *ProductRepo) GetByID(id uint64) (*model.Product, error) {
	var p model.Product
	if err := r.db.First(&p, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

// GetSKUs 查询商品全部 SKU。
func (r *ProductRepo) GetSKUs(productID uint64) ([]model.SKU, error) {
	var list []model.SKU
	err := r.db.Where("product_id = ?", productID).Order("id ASC").Find(&list).Error
	return list, err
}

// AllSKUs 全量 SKU(启动时同步 Redis 库存用)。
func (r *ProductRepo) AllSKUs() ([]model.SKU, error) {
	var list []model.SKU
	err := r.db.Find(&list).Error
	return list, err
}

// GetSKUByProductAndSize 查询单个 SKU。
func (r *ProductRepo) GetSKUByProductAndSize(productID uint64, size string) (*model.SKU, error) {
	var s model.SKU
	err := r.db.Where("product_id = ? AND size = ?", productID, size).First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// CreateWithSKUs 事务创建商品及 SKU。
func (r *ProductRepo) CreateWithSKUs(p *model.Product, skus []model.SKU) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(p).Error; err != nil {
			return err
		}
		for i := range skus {
			skus[i].ProductID = p.ID
		}
		if len(skus) > 0 {
			if err := tx.Create(&skus).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Update 更新商品基础字段。
func (r *ProductRepo) Update(p *model.Product) error {
	return r.db.Model(&model.Product{}).Where("id = ?", p.ID).Updates(map[string]any{
		"category_id": p.CategoryID,
		"name":        p.Name,
		"age_group":   p.AgeGroup,
		"material":    p.Material,
		"season":      p.Season,
		"description": p.Description,
		"image":       p.Image,
		"price":       p.Price,
		"status":      p.Status,
		"recommended": p.Recommended,
	}).Error
}

// DeleteWithSKUs 事务删除商品及其 SKU。
func (r *ProductRepo) DeleteWithSKUs(id uint64) ([]model.SKU, error) {
	var skus []model.SKU
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("product_id = ?", id).Find(&skus).Error; err != nil {
			return err
		}
		if err := tx.Where("product_id = ?", id).Delete(&model.SKU{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Product{}, id).Error
	})
	if err != nil {
		return nil, err
	}
	return skus, nil
}

// ReplaceSKUs 事务内全量替换商品 SKU(管理端编辑商品时使用)。
// 对仍存在的尺码保留现有库存,新增尺码使用传入库存,删除多余尺码。
func (r *ProductRepo) ReplaceSKUs(productID uint64, skus []model.SKU) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var old []model.SKU
		if err := tx.Where("product_id = ?", productID).Find(&old).Error; err != nil {
			return err
		}
		oldStock := make(map[string]int, len(old))
		for _, s := range old {
			oldStock[s.Size] = s.Stock
		}
		if err := tx.Where("product_id = ?", productID).Delete(&model.SKU{}).Error; err != nil {
			return err
		}
		for i := range skus {
			skus[i].ProductID = productID
			skus[i].ID = 0
			if st, ok := oldStock[skus[i].Size]; ok {
				skus[i].Stock = st // 同尺码保留库存
			}
		}
		if len(skus) > 0 {
			return tx.Create(&skus).Error
		}
		return nil
	})
}

// DeductStockTx MySQL 侧条件扣减库存(防超卖核心):
// UPDATE skus SET stock = stock - qty WHERE product_id=? AND size=? AND stock >= qty
// 返回是否扣减成功;事务回滚由调用方统一处理。
func DeductStockTx(tx *gorm.DB, productID uint64, size string, qty int) (bool, error) {
	res := tx.Model(&model.SKU{}).
		Where("product_id = ? AND size = ? AND stock >= ?", productID, size, qty).
		UpdateColumn("stock", gorm.Expr("stock - ?", qty))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// RefundStockTx MySQL 侧回补库存。
func RefundStockTx(tx *gorm.DB, productID uint64, size string, qty int) error {
	return tx.Model(&model.SKU{}).
		Where("product_id = ? AND size = ?", productID, size).
		UpdateColumn("stock", gorm.Expr("stock + ?", qty)).Error
}

// UpdateSKUStock 管理端直接设置某 SKU 库存。
func (r *ProductRepo) UpdateSKUStock(skuID uint64, stock int) (*model.SKU, error) {
	var s model.SKU
	if err := r.db.First(&s, skuID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	if err := r.db.Model(&model.SKU{}).Where("id = ?", skuID).
		UpdateColumn("stock", stock).Error; err != nil {
		return nil, err
	}
	s.Stock = stock
	return &s, nil
}

// Count 商品总数。
func (r *ProductRepo) Count() (int64, error) {
	var n int64
	err := r.db.Model(&model.Product{}).Count(&n).Error
	return n, err
}
