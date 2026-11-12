package repository

import (
	"fawn-mall/internal/model"
	"fawn-mall/pkg/resp"

	"gorm.io/gorm"
)

// CategoryRepo 分类数据访问。
type CategoryRepo struct{ db *gorm.DB }

// NewCategoryRepo 构造。
func NewCategoryRepo(db *gorm.DB) *CategoryRepo { return &CategoryRepo{db: db} }

// List 全量分类,按 Sort、ID 排序。
func (r *CategoryRepo) List() ([]model.Category, error) {
	var list []model.Category
	err := r.db.Order("sort ASC, id ASC").Find(&list).Error
	return list, err
}

// GetByID 按 ID 查询。
func (r *CategoryRepo) GetByID(id uint64) (*model.Category, error) {
	var c model.Category
	if err := r.db.First(&c, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, resp.ErrCategory
		}
		return nil, err
	}
	return &c, nil
}

// Create 新建分类,重名返回业务错误。
func (r *CategoryRepo) Create(c *model.Category) error {
	var n int64
	if err := r.db.Model(&model.Category{}).Where("name = ?", c.Name).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return resp.ErrCategoryDup
	}
	return r.db.Create(c).Error
}

// Update 更新分类。
func (r *CategoryRepo) Update(c *model.Category) error {
	return r.db.Model(&model.Category{}).Where("id = ?", c.ID).
		Updates(map[string]any{"name": c.Name, "sort": c.Sort}).Error
}

// Delete 删除分类;分类下仍有商品时禁止删除。
func (r *CategoryRepo) Delete(id uint64) error {
	var n int64
	if err := r.db.Model(&model.Product{}).Where("category_id = ?", id).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return resp.NewBiz(resp.CodeCategoryNotFound, "该分类下仍有商品,无法删除")
	}
	res := r.db.Delete(&model.Category{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return resp.ErrCategory
	}
	return nil
}

// Count 分类总数。
func (r *CategoryRepo) Count() (int64, error) {
	var n int64
	err := r.db.Model(&model.Category{}).Count(&n).Error
	return n, err
}

// NamesByID 批量取分类名(id → name)。
func (r *CategoryRepo) NamesByID(ids []uint64) (map[uint64]string, error) {
	list, err := r.List()
	if err != nil {
		return nil, err
	}
	all := make(map[uint64]string, len(list))
	for _, c := range list {
		all[c.ID] = c.Name
	}
	m := make(map[uint64]string, len(ids))
	for _, id := range ids {
		if name, ok := all[id]; ok {
			m[id] = name
		}
	}
	return m, nil
}
