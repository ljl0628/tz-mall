package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fawn-mall/internal/middleware"
	"fawn-mall/internal/model"
	"fawn-mall/internal/service"
	"fawn-mall/pkg/resp"
	"fawn-mall/pkg/storage"
)

// AdminHandler 管理端接口(需管理员角色)。
// uploader 为 nil 表示未配置对象存储,上传接口返回明确业务错误,其余功能不受影响。
type AdminHandler struct {
	admin    *service.AdminService
	uploader storage.Uploader
}

// NewAdminHandler 构造。
func NewAdminHandler(a *service.AdminService, up storage.Uploader) *AdminHandler {
	return &AdminHandler{admin: a, uploader: up}
}

// ListProducts GET /admin/products
func (h *AdminHandler) ListProducts(c *gin.Context) {
	var f model.ProductFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "查询参数错误"))
		return
	}
	out, err := h.admin.AdminListProducts(c.Request.Context(), f)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// CreateProduct POST /admin/products
func (h *AdminHandler) CreateProduct(c *gin.Context) {
	var in service.ProductInput
	if err := c.ShouldBindJSON(&in); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:"+bindErrMsg(err)))
		return
	}
	out, err := h.admin.CreateProduct(c.Request.Context(), in)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// UpdateProduct PUT /admin/products/:id
func (h *AdminHandler) UpdateProduct(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "商品 ID 非法"))
		return
	}
	var in service.ProductInput
	if err := c.ShouldBindJSON(&in); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:"+bindErrMsg(err)))
		return
	}
	if err := h.admin.UpdateProduct(c.Request.Context(), id, in); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}

// DeleteProduct DELETE /admin/products/:id
func (h *AdminHandler) DeleteProduct(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "商品 ID 非法"))
		return
	}
	if err := h.admin.DeleteProduct(c.Request.Context(), id); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"deleted": true})
}

// UpdateSKUStock PUT /admin/skus/:id  body: {stock}
func (h *AdminHandler) UpdateSKUStock(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "SKU ID 非法"))
		return
	}
	var req struct {
		Stock int `json:"stock" binding:"gte=0,lte=1000000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "库存必须为 0-1000000"))
		return
	}
	if err := h.admin.UpdateSKUStock(c.Request.Context(), id, req.Stock); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}

// ListCategories GET /admin/categories
func (h *AdminHandler) ListCategories(c *gin.Context) {
	// 复用 C 端接口亦可,这里保持管理端完整性
	out, err := h.admin.ListCategories(c.Request.Context())
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// CreateCategory POST /admin/categories
func (h *AdminHandler) CreateCategory(c *gin.Context) {
	var in service.CategoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:"+bindErrMsg(err)))
		return
	}
	out, err := h.admin.CreateCategory(c.Request.Context(), in)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// UpdateCategory PUT /admin/categories/:id
func (h *AdminHandler) UpdateCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "分类 ID 非法"))
		return
	}
	var in service.CategoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:"+bindErrMsg(err)))
		return
	}
	if err := h.admin.UpdateCategory(c.Request.Context(), id, in); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}

// DeleteCategory DELETE /admin/categories/:id
func (h *AdminHandler) DeleteCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "分类 ID 非法"))
		return
	}
	if err := h.admin.DeleteCategory(c.Request.Context(), id); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"deleted": true})
}

// ListOrders GET /admin/orders?status&page&page_size
func (h *AdminHandler) ListOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	status := c.Query("status")
	if status != "" {
		switch status {
		case model.OrderStatusPendingPayment, model.OrderStatusPaid,
			model.OrderStatusShipped, model.OrderStatusCompleted, model.OrderStatusCanceled:
		default:
			resp.Fail(c, resp.NewBiz(resp.CodeParamError, "订单状态非法"))
			return
		}
	}
	out, err := h.admin.ListOrders(c.Request.Context(), status, page, pageSize)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Ship PUT /admin/orders/:orderNo/ship
func (h *AdminHandler) Ship(c *gin.Context) {
	orderNo := c.Param("orderNo")
	if err := h.admin.Ship(c.Request.Context(), orderNo); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"shipped": true})
}

// Me GET /admin/me —— 管理端信息探针(前端用于校验管理员身份)
func (h *AdminHandler) Me(c *gin.Context) {
	claims := middleware.GetClaims(c)
	resp.OK(c, gin.H{"username": claims.Username, "role": claims.Role})
}
