package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fawn-mall/internal/middleware"
	"fawn-mall/internal/repository"
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

// RefundList GET /admin/finance/refunds?status&page&page_size —— 售后单分页
func (h *FinanceHandler) RefundList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	out, err := h.finance.ListRefunds(c.Request.Context(), c.Query("status"), page, pageSize)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Flows GET /admin/finance/flows?start_date&end_date&order_no&flow_no&direction&page
func (h *FinanceHandler) Flows(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	dir, _ := strconv.Atoi(c.DefaultQuery("direction", "0"))
	q := repository.FlowQuery{
		OrderNo:   c.Query("order_no"),
		FlowNo:    c.Query("flow_no"),
		Direction: int8(dir),
		Page:      page,
		PageSize:  pageSize,
	}
	if d := c.Query("start_date"); d != "" {
		start, _, err := service.DayRange(d)
		if err != nil {
			resp.Fail(c, resp.NewBiz(resp.CodeParamError, "开始日期格式应为 YYYY-MM-DD"))
			return
		}
		q.StartAt = start
	}
	if d := c.Query("end_date"); d != "" {
		_, end, err := service.DayRange(d)
		if err != nil {
			resp.Fail(c, resp.NewBiz(resp.CodeParamError, "结束日期格式应为 YYYY-MM-DD"))
			return
		}
		q.EndAt = end // [start, end):end_date 当天全天包含
	}
	out, err := h.finance.ListFlows(c.Request.Context(), q)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Dashboard GET /admin/finance/dashboard —— 经营看板指标
func (h *FinanceHandler) Dashboard(c *gin.Context) {
	out, err := h.finance.Dashboard(c.Request.Context())
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Trend GET /admin/finance/trend?days=7 —— 收入趋势图数据
func (h *FinanceHandler) Trend(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	out, err := h.finance.Trend(c.Request.Context(), days)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Reconcile POST /admin/finance/reconcile —— 一键对账
func (h *FinanceHandler) Reconcile(c *gin.Context) {
	out, err := h.finance.Reconcile(c.Request.Context())
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// Statements GET /admin/finance/statements?page —— 日结账单列表
func (h *FinanceHandler) Statements(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	out, err := h.finance.ListStatements(c.Request.Context(), page, pageSize)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, out)
}

// RunStatements POST /admin/finance/statements/run —— 手动日结(单日/区间回填)
func (h *FinanceHandler) RunStatements(c *gin.Context) {
	var req service.StatementRunReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, resp.NewBiz(resp.CodeParamError, "参数错误:"+bindErrMsg(err)))
		return
	}
	n, err := h.finance.RunStatements(c.Request.Context(), req)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, gin.H{"done_days": n})
}
