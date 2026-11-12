package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fawn-mall/internal/model"
	"fawn-mall/internal/service"
	"fawn-mall/pkg/resp"
)

// ProductHandler 商品接口(C 端)。
type ProductHandler struct {
	products *service.ProductService
}

// NewProductHandler 构造。
func NewProductHandler(p *service.ProductService) *ProductHandler {
	return &ProductHandler{products: p}
}

// List GET /products?page&page_size&category_id&age_group&keyword
func (h *ProductHandler) List(c *gin.Context) {
	var f model.ProductFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "查询参数错误"))
		return
	}
	out, err := h.products.List(c.Request.Context(), f)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Detail GET /products/:id(Redis 缓存优先)
func (h *ProductHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "商品 ID 非法"))
		return
	}
	out, err := h.products.Detail(c.Request.Context(), id)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Home GET /home(聚合缓存)
func (h *ProductHandler) Home(c *gin.Context) {
	out, err := h.products.Home(c.Request.Context())
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Categories GET /categories
func (h *ProductHandler) Categories(c *gin.Context) {
	out, err := h.products.Categories(c.Request.Context())
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}
