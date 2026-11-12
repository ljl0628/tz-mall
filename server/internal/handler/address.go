package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fawn-mall/internal/middleware"
	"fawn-mall/internal/service"
	"fawn-mall/pkg/resp"
)

// AddressHandler 收货地址簿接口(均需登录,只操作本人数据)。
type AddressHandler struct {
	addrs *service.AddressService
}

// NewAddressHandler 构造。
func NewAddressHandler(a *service.AddressService) *AddressHandler { return &AddressHandler{addrs: a} }

// List GET /user/addresses
func (h *AddressHandler) List(c *gin.Context) {
	list, err := h.addrs.List(c.Request.Context(), middleware.GetClaims(c).UserID)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, list)
}

// Create POST /user/addresses
func (h *AddressHandler) Create(c *gin.Context) {
	var in service.AddressInput
	if err := c.ShouldBindJSON(&in); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:收货人 2-32 字、手机号 11 位、省市区与详细地址必填"))
		return
	}
	out, err := h.addrs.Create(c.Request.Context(), middleware.GetClaims(c).UserID, in)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Update PUT /user/addresses/:id
func (h *AddressHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "地址 ID 非法"))
		return
	}
	var in service.AddressInput
	if err := c.ShouldBindJSON(&in); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:收货人 2-32 字、手机号 11 位、省市区与详细地址必填"))
		return
	}
	out, err := h.addrs.Update(c.Request.Context(), middleware.GetClaims(c).UserID, id, in)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Delete DELETE /user/addresses/:id
func (h *AddressHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "地址 ID 非法"))
		return
	}
	if err := h.addrs.Delete(c.Request.Context(), middleware.GetClaims(c).UserID, id); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"deleted": true})
}

// SetDefault PUT /user/addresses/:id/default
func (h *AddressHandler) SetDefault(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "地址 ID 非法"))
		return
	}
	if err := h.addrs.SetDefault(c.Request.Context(), middleware.GetClaims(c).UserID, id); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}
