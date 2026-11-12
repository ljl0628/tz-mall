package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fawn-mall/internal/middleware"
	"fawn-mall/internal/service"
	"fawn-mall/pkg/resp"
)

// OrderHandler 订单接口(均需登录)。
type OrderHandler struct {
	orders *service.OrderService
}

// NewOrderHandler 构造。
func NewOrderHandler(o *service.OrderService) *OrderHandler { return &OrderHandler{orders: o} }

// Place POST /orders —— 下单(Redis 预扣 + MQ 异步落库)
func (h *OrderHandler) Place(c *gin.Context) {
	var req service.PlaceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:"+bindErrMsg(err)))
		return
	}
	out, err := h.orders.Place(c.Request.Context(), middleware.GetClaims(c).UserID, req)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// List GET /orders?page&page_size&status
func (h *OrderHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	out, err := h.orders.List(c.Request.Context(), middleware.GetClaims(c).UserID,
		c.Query("status"), page, pageSize)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Detail GET /orders/:orderNo
func (h *OrderHandler) Detail(c *gin.Context) {
	out, err := h.orders.Detail(c.Request.Context(), middleware.GetClaims(c).UserID, c.Param("orderNo"))
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Pay POST /orders/:orderNo/pay —— 模拟支付
func (h *OrderHandler) Pay(c *gin.Context) {
	out, err := h.orders.Pay(c.Request.Context(), middleware.GetClaims(c).UserID, c.Param("orderNo"))
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Cancel POST /orders/:orderNo/cancel —— 用户取消
func (h *OrderHandler) Cancel(c *gin.Context) {
	out, err := h.orders.Cancel(c.Request.Context(), middleware.GetClaims(c).UserID, c.Param("orderNo"))
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Confirm POST /orders/:orderNo/confirm —— 确认收货
func (h *OrderHandler) Confirm(c *gin.Context) {
	out, err := h.orders.Confirm(c.Request.Context(), middleware.GetClaims(c).UserID, c.Param("orderNo"))
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}
