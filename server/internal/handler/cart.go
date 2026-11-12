package handler

import (
	"github.com/gin-gonic/gin"

	"fawn-mall/internal/middleware"
	"fawn-mall/internal/service"
	"fawn-mall/pkg/resp"
)

// CartHandler 购物车接口(均需登录)。
type CartHandler struct {
	cart *service.CartService
}

// NewCartHandler 构造。
func NewCartHandler(cart *service.CartService) *CartHandler { return &CartHandler{cart: cart} }

// Get GET /cart
func (h *CartHandler) Get(c *gin.Context) {
	userID := middleware.GetClaims(c).UserID
	out, err := h.cart.Get(c.Request.Context(), userID)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Add POST /cart/items
func (h *CartHandler) Add(c *gin.Context) {
	var req service.AddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:"+bindErrMsg(err)))
		return
	}
	if err := h.cart.Add(c.Request.Context(), middleware.GetClaims(c).UserID, req); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"added": true})
}

// Update PUT /cart/items/:field  body: {quantity?, selected?}
func (h *CartHandler) Update(c *gin.Context) {
	var req service.UpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:"+bindErrMsg(err)))
		return
	}
	if err := h.cart.Update(c.Request.Context(), middleware.GetClaims(c).UserID, c.Param("field"), req); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}

// Remove DELETE /cart/items/:field
func (h *CartHandler) Remove(c *gin.Context) {
	if err := h.cart.Remove(c.Request.Context(), middleware.GetClaims(c).UserID, c.Param("field")); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"removed": true})
}

// SelectAll PUT /cart/select-all  body: {selected: bool}
func (h *CartHandler) SelectAll(c *gin.Context) {
	var req struct {
		Selected bool `json:"selected"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误"))
		return
	}
	if err := h.cart.SelectAll(c.Request.Context(), middleware.GetClaims(c).UserID, req.Selected); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"selected": req.Selected})
}
