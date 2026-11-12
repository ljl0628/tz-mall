package repository

import (
	"gorm.io/gorm"

	"fawn-mall/internal/model"
)

// AddressRepo 收货地址数据访问。
type AddressRepo struct{ db *gorm.DB }

// NewAddressRepo 构造。
func NewAddressRepo(db *gorm.DB) *AddressRepo { return &AddressRepo{db: db} }

// ListByUser 用户的地址列表:默认地址置顶,其余按创建时间倒序。
func (r *AddressRepo) ListByUser(userID uint64) ([]model.Address, error) {
	var list []model.Address
	err := r.db.Where("user_id = ?", userID).
		Order("is_default DESC, id DESC").Find(&list).Error
	return list, err
}

// GetByID 按 ID 查询(nil = 不存在)。
func (r *AddressRepo) GetByID(id uint64) (*model.Address, error) {
	var a model.Address
	if err := r.db.First(&a, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// Create 新建地址。
func (r *AddressRepo) Create(a *model.Address) error { return r.db.Create(a).Error }

// Update 更新地址(按 id + user_id 定位,越权时 RowsAffected=0)。
func (r *AddressRepo) Update(a *model.Address) error {
	return r.db.Model(&model.Address{}).
		Where("id = ? AND user_id = ?", a.ID, a.UserID).
		Updates(map[string]any{
			"receiver": a.Receiver, "phone": a.Phone,
			"province": a.Province, "city": a.City, "district": a.District,
			"detail": a.Detail,
		}).Error
}

// Delete 删除地址(仅本人的)。
func (r *AddressRepo) Delete(userID, id uint64) error {
	return r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Address{}).Error
}

// CountByUser 用户地址数量(上限控制)。
func (r *AddressRepo) CountByUser(userID uint64) (int64, error) {
	var n int64
	err := r.db.Model(&model.Address{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}

// ClearDefault 清除用户全部默认标记(设默认前调用)。
func (r *AddressRepo) ClearDefault(userID uint64) error {
	return r.db.Model(&model.Address{}).
		Where("user_id = ? AND is_default = true", userID).
		Update("is_default", false).Error
}

// SetDefault 事务:清除旧默认 → 标记目标地址(仅本人的)。
func (r *AddressRepo) SetDefault(userID, id uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Address{}).
			Where("user_id = ?", userID).Update("is_default", false).Error; err != nil {
			return err
		}
		res := tx.Model(&model.Address{}).
			Where("id = ? AND user_id = ?", id, userID).Update("is_default", true)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
