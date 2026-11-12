package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fawn-mall/internal/middleware"
	"fawn-mall/internal/service"
	"fawn-mall/pkg/resp"
)

// ---------- 用户管理接口(管理员) ----------

// ListUsers GET /admin/users
func (h *AdminHandler) ListUsers(c *gin.Context) {
	var f service.UserFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "查询参数错误"))
		return
	}
	out, err := h.admin.AdminListUsers(c.Request.Context(), f)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// SetUserStatus PUT /admin/users/:id/status
func (h *AdminHandler) SetUserStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "用户 ID 非法"))
		return
	}
	var in service.SetStatusReq
	if err := c.ShouldBindJSON(&in); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:status 仅支持 0(禁用)/ 1(正常)"))
		return
	}
	claims := middleware.GetClaims(c)
	if err := h.admin.SetUserStatus(c.Request.Context(), claims.UserID, id, in.Status); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}

// SetUserRole PUT /admin/users/:id/role
func (h *AdminHandler) SetUserRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "用户 ID 非法"))
		return
	}
	var in service.SetRoleReq
	if err := c.ShouldBindJSON(&in); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:role 仅支持 user / admin"))
		return
	}
	claims := middleware.GetClaims(c)
	if err := h.admin.SetUserRole(c.Request.Context(), claims.UserID, id, in.Role); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}

// ResetUserPassword PUT /admin/users/:id/password
func (h *AdminHandler) ResetUserPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "用户 ID 非法"))
		return
	}
	var in service.ResetPasswordReq
	if err := c.ShouldBindJSON(&in); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:密码需 6-64 位"))
		return
	}
	if err := h.admin.ResetUserPassword(c.Request.Context(), id, in.Password); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"reset": true})
}
