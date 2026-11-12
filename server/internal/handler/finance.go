package handler

import (
	"github.com/gin-gonic/gin"

	"fawn-mall/internal/middleware"
	"fawn-mall/internal/service"
	"fawn-mall/pkg/resp"
)

// FinanceHandler 财务中心接口(管理员):退款售后;流水/看板/对账见查询接口。
type FinanceHandler struct {
	finance *service.FinanceService
}

// NewFinanceHandler 构造。
func NewFinanceHandler(f *service.FinanceService) *FinanceHandler { return &FinanceHandler{finance: f} }

// AdminRefund POST /admin/finance/refunds —— 管理员对已支付订单直接退款
func (h *FinanceHandler) AdminRefund(c *gin.Context) {
	var req service.AdminRefundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:"+bindErrMsg(err)))
		return
	}
	out, err := h.finance.AdminRefund(c.Request.Context(), middleware.GetClaims(c).UserID, req.OrderNo, req.Reason)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// ProcessRefund POST /admin/finance/refunds/:refundNo/process —— 处理用户退款申请
func (h *FinanceHandler) ProcessRefund(c *gin.Context) {
	var req service.ProcessRefundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:"+bindErrMsg(err)))
		return
	}
	if err := h.finance.ProcessRefund(c.Request.Context(), middleware.GetClaims(c).UserID,
		c.Param("refundNo"), req.Approve); err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"refund_no": c.Param("refundNo")})
}
