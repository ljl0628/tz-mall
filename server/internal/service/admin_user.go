package service

import (
	"context"

	"fawn-mall/internal/model"
	"fawn-mall/internal/repository"
	"fawn-mall/pkg/password"
	"fawn-mall/pkg/resp"
)

// ---------- 用户管理(管理端) ----------
// 封禁与角色调整除更新 MySQL 外,同步写 Redis 覆盖键(repository.UserState),
// 让操作对已签发的 token 即时生效,而不是等最长 72 小时自然过期。

// UserPage 用户列表分页结果。
type UserPage struct {
	List     []model.User `json:"list"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

// UserFilter 用户列表查询条件(别名:handler 绑定 query 用,实现委托给 repository)。
type UserFilter = repository.UserFilter

// SetStatusReq 封禁/解封入参。注意不能加 required:validator 把零值视为缺失,
// status=0(封禁)会被误拦,oneof 已足够约束取值范围。
type SetStatusReq struct {
	Status int8 `json:"status" binding:"oneof=0 1"`
}

// SetRoleReq 角色调整入参。
type SetRoleReq struct {
	Role string `json:"role" binding:"required,oneof=user admin"`
}

// ResetPasswordReq 重置密码入参。
type ResetPasswordReq struct {
	Password string `json:"password" binding:"required,min=6,max=64"`
}

// AdminListUsers 用户列表(用户名模糊 / 角色 / 状态筛选 + 分页)。
func (s *AdminService) AdminListUsers(ctx context.Context, f repository.UserFilter) (*UserPage, error) {
	list, total, err := s.users.List(f)
	if err != nil {
		return nil, err
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 10
	}
	return &UserPage{List: list, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// SetUserStatus 封禁/解封用户:更新 MySQL + Redis 覆盖键(即时生效)。
// operatorID 为当前操作的管理员,不允许操作自己的账号(防止把自己封到登录不进来)。
func (s *AdminService) SetUserStatus(ctx context.Context, operatorID, userID uint64, status int8) error {
	if userID == operatorID {
		return resp.ErrSelfOp
	}
	u, err := s.users.GetByID(userID)
	if err != nil {
		return err
	}
	if u == nil {
		return resp.ErrUserNotFound
	}
	if err := s.users.UpdateStatus(userID, status); err != nil {
		return err
	}
	if status == model.UserStatusDisabled {
		return s.ustate.SetDisabled(ctx, userID)
	}
	return s.ustate.ClearDisabled(ctx, userID)
}

// SetUserRole 调整角色(升级管理员/降为普通用户):
// MySQL 更新 + Redis 角色覆盖键,降权管理员的存量 token 立即失去管理权限。
func (s *AdminService) SetUserRole(ctx context.Context, operatorID, userID uint64, role string) error {
	if userID == operatorID {
		return resp.ErrSelfOp
	}
	u, err := s.users.GetByID(userID)
	if err != nil {
		return err
	}
	if u == nil {
		return resp.ErrUserNotFound
	}
	if u.Role == role {
		return nil // 目标角色与当前一致,幂等返回
	}
	if err := s.users.UpdateRole(userID, role); err != nil {
		return err
	}
	return s.ustate.SetRole(ctx, userID, role)
}

// ResetUserPassword 重置密码(bcrypt 重新加密)。
// 允许管理员重置自己的密码;token 无状态且不逐设备跟踪,已登录设备在 token
// 过期前仍有效,新密码在下次登录时生效。
func (s *AdminService) ResetUserPassword(ctx context.Context, userID uint64, newPassword string) error {
	u, err := s.users.GetByID(userID)
	if err != nil {
		return err
	}
	if u == nil {
		return resp.ErrUserNotFound
	}
	hash, err := password.Hash(newPassword)
	if err != nil {
		return err
	}
	return s.users.UpdatePasswordHash(userID, hash)
}
