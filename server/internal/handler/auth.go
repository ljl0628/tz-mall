package handler

import (
	"github.com/gin-gonic/gin"

	"fawn-mall/internal/middleware"
	"fawn-mall/internal/service"
	"fawn-mall/pkg/resp"
)

// AuthHandler 认证相关接口。
type AuthHandler struct {
	auth *service.AuthService
}

// NewAuthHandler 构造。
func NewAuthHandler(auth *service.AuthService) *AuthHandler { return &AuthHandler{auth: auth} }

// Register POST /auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req service.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:"+bindErrMsg(err)))
		return
	}
	out, err := h.auth.Register(c.Request.Context(), req)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Login POST /auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "用户名和密码不能为空"))
		return
	}
	out, err := h.auth.Login(c.Request.Context(), req)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Logout POST /auth/logout(需登录)
func (h *AuthHandler) Logout(c *gin.Context) {
	claims := middleware.GetClaims(c)
	token := middleware.GetToken(c)
	if claims == nil {
		resp.Fail(c, resp.NewBiz(resp.CodeUnauthorized, "请先登录"))
		return
	}
	if err := h.auth.Logout(c.Request.Context(), token, claims.ExpiresAt.Time); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"logged_out": true})
}

// Me GET /user/me(需登录)
func (h *AuthHandler) Me(c *gin.Context) {
	claims := middleware.GetClaims(c)
	u, err := h.auth.Me(c.Request.Context(), claims.UserID)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, u)
}
