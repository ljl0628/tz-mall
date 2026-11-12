package repository

import (
	"strings"

	"fawn-mall/internal/model"

	"gorm.io/gorm"
)

// UserRepo 用户数据访问。
type UserRepo struct{ db *gorm.DB }

// NewUserRepo 构造。
func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

// UserFilter 用户列表筛选(管理端)。
type UserFilter struct {
	Page     int    `form:"page"` // 从 1 开始
	PageSize int    `form:"page_size"`
	Keyword  string `form:"keyword"` // 用户名模糊匹配
	Role     string `form:"role"`    // user | admin,空为全部
	Status   *int8  `form:"status"`  // 1 正常 0 禁用,nil 为全部
}

// List 分页查询用户(用户名模糊 / 角色 / 状态筛选,按 ID 倒序)。
func (r *UserRepo) List(f UserFilter) ([]model.User, int64, error) {
	q := r.db.Model(&model.User{})
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		q = q.Where("username LIKE ?", "%"+kw+"%")
	}
	if f.Role != "" {
		q = q.Where("role = ?", f.Role)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 10
	}
	var list []model.User
	if err := q.Order("id DESC").Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateStatus 更新用户状态(封禁/解封)。
func (r *UserRepo) UpdateStatus(id uint64, status int8) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Update("status", status).Error
}

// UpdateRole 更新用户角色。
func (r *UserRepo) UpdateRole(id uint64, role string) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Update("role", role).Error
}

// UpdatePasswordHash 重置密码(bcrypt 摘要)。
func (r *UserRepo) UpdatePasswordHash(id uint64, hash string) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Update("password_hash", hash).Error
}

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
