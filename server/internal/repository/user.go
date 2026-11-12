package repository

import (
	"fawn-mall/internal/model"

	"gorm.io/gorm"
)

// UserRepo 用户数据访问。
type UserRepo struct{ db *gorm.DB }

// NewUserRepo 构造。
func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

// GetByUsername 按用户名查询。
func (r *UserRepo) GetByUsername(username string) (*model.User, error) {
	var u model.User
	if err := r.db.Where("username = ?", username).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// GetByID 按 ID 查询。
func (r *UserRepo) GetByID(id uint64) (*model.User, error) {
	var u model.User
	if err := r.db.First(&u, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// Create 新建用户。
func (r *UserRepo) Create(u *model.User) error { return r.db.Create(u).Error }

// CountByRole 统计某角色用户数(种子数据幂等判断用)。
func (r *UserRepo) CountByRole(role string) (int64, error) {
	var n int64
	err := r.db.Model(&model.User{}).Where("role = ?", role).Count(&n).Error
	return n, err
}
