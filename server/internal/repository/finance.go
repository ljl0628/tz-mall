package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"fawn-mall/internal/model"
)

// 退款链路哨兵错误(service 翻译为业务提示)。
var (
	// ErrOrderNotFound 订单不存在(事务内锁定订单时未命中)。
	ErrOrderNotFound = errors.New("order not found")
	// ErrRefundNotFound 退款售后单不存在。
	ErrRefundNotFound = errors.New("refund order not found")
	// ErrRefundNotPending 售后单不是待处理状态(重复操作或状态异常)。
	ErrRefundNotPending = errors.New("refund order not pending")
	// ErrOrderNotPaid 订单当前不是已支付状态。
	ErrOrderNotPaid = errors.New("order not paid")
	// ErrRefundApplyExists 该订单已存在进行中的售后单(待处理或已退款)。
	ErrRefundApplyExists = errors.New("refund apply exists")
)

// FinanceRepo 财务中心数据访问:退款事务(防并发)、售后单。
type FinanceRepo struct{ db *gorm.DB }

// NewFinanceRepo 构造。
func NewFinanceRepo(db *gorm.DB) *FinanceRepo { return &FinanceRepo{db: db} }

// refundCore 退款事务内公共部分:
// 条件更新 orders(paid → refunded)→ 插入出账流水 → 回补 MySQL 库存。
// done=false 表示订单当前不是已支付(重复操作或状态异常),事务内到此为止、无任何写入,
// 从机制上杜绝超额/重复退款。
func refundCore(tx *gorm.DB, orderNo string, items []model.OrderItem, flow *model.PayFlow) (bool, error) {
	res := tx.Model(&model.Order{}).
		Where("order_no = ? AND status = ?", orderNo, model.OrderStatusPaid).
		Updates(map[string]any{
			"status":      model.OrderStatusRefunded,
			"refund_time": time.Now(),
		})
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected == 0 {
		return false, nil // 已退款/已发货/已取消:绝不重复退款
	}
	if err := tx.Create(flow).Error; err != nil {
		return false, err
	}
	for _, it := range items {
		if err := RefundStockTx(tx, it.ProductID, it.Size, it.Quantity); err != nil {
			return false, err
		}
	}
	return true, nil
}

// AdminRefund 管理员直接退款事务:refundCore + 落一条「已退款」售后单;
// 同订单此前若有用户待处理申请,由本次退款一并完成,避免悬挂。
// 返回 false 表示订单非已支付(重复操作或状态异常)。
func (r *FinanceRepo) AdminRefund(order *model.Order, flow *model.PayFlow, refund *model.RefundOrder) (bool, error) {
	var refunded bool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		done, err := refundCore(tx, order.OrderNo, order.Items, flow)
		if err != nil {
			return err
		}
		if !done {
			return nil
		}
		if err := tx.Create(refund).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.RefundOrder{}).
			Where("order_no = ? AND status = ?", order.OrderNo, model.RefundStatusPending).
			Updates(map[string]any{
				"status":       model.RefundStatusRefunded,
				"operator_id":  refund.OperatorID,
				"processed_at": time.Now(),
			}).Error; err != nil {
			return err
		}
		refunded = true
		return nil
	})
	return refunded, err
}

// CreateRefundApply 用户申请退款事务:
// 锁定订单行(与支付/发货/退款串行化)→ 校验仍为已支付 → 校验无进行中售后单 → 插入待处理售后单。
// Amount 以锁定后的订单实付金额为准。
func (r *FinanceRepo) CreateRefundApply(refund *model.RefundOrder) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var order model.Order
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_no = ?", refund.OrderNo).First(&order).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrOrderNotFound
			}
			return err
		}
		if order.Status != model.OrderStatusPaid {
			return ErrOrderNotPaid
		}
		var n int64
		if err := tx.Model(&model.RefundOrder{}).
			Where("order_no = ? AND status IN ?", refund.OrderNo,
				[]string{model.RefundStatusPending, model.RefundStatusRefunded}).
			Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return ErrRefundApplyExists
		}
		refund.Amount = order.PayAmount
		return tx.Create(refund).Error
	})
}

// ProcessRefundApply 管理员处理用户退款申请事务:
//
//	拒绝:售后单 → rejected(订单不动);
//	同意:refundCore(条件更新 paid → refunded + 出账流水 + 回补库存)
//	      → 售后单 → refunded,同订单其余待处理申请一并完成。
//
// 返回订单明细(供事务外回补 Redis 库存)。
func (r *FinanceRepo) ProcessRefundApply(refundNo string, approve bool, operatorID uint64, flowNo string) ([]model.OrderItem, error) {
	var items []model.OrderItem
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var row model.RefundOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("refund_no = ?", refundNo).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRefundNotFound
			}
			return err
		}
		if row.Status != model.RefundStatusPending {
			return ErrRefundNotPending // 重复操作或状态异常
		}

		if !approve {
			return tx.Model(&row).Updates(map[string]any{
				"status":       model.RefundStatusRejected,
				"operator_id":  operatorID,
				"processed_at": time.Now(),
			}).Error
		}

		var order model.Order
		if err := tx.Where("order_no = ?", row.OrderNo).First(&order).Error; err != nil {
			return err
		}
		if err := tx.Where("order_id = ?", order.ID).Find(&items).Error; err != nil {
			return err
		}
		flow := &model.PayFlow{
			FlowNo:    flowNo,
			OrderNo:   row.OrderNo,
			UserID:    row.UserID,
			Direction: model.FlowDirectionOut,
			BizType:   model.FlowBizRefund,
			Amount:    row.Amount,
			Status:    model.FlowStatusSuccess,
			Remark:    "退款(审核通过)",
		}
		done, err := refundCore(tx, row.OrderNo, items, flow)
		if err != nil {
			return err
		}
		if !done {
			return ErrOrderNotPaid // 下单后订单已发货/已退款等,申请作废回滚
		}
		now := time.Now()
		if err := tx.Model(&row).Updates(map[string]any{
			"status":       model.RefundStatusRefunded,
			"operator_id":  operatorID,
			"processed_at": now,
		}).Error; err != nil {
			return err
		}
		// 同订单其余待处理申请一并完成,避免悬挂
		return tx.Model(&model.RefundOrder{}).
			Where("order_no = ? AND status = ? AND id <> ?", row.OrderNo, model.RefundStatusPending, row.ID).
			Updates(map[string]any{
				"status":       model.RefundStatusRefunded,
				"operator_id":  operatorID,
				"processed_at": now,
			}).Error
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ---------- 查询:流水分页 / 售后单 / 对账 / 日结 ----------

// FlowQuery 资金流水分页查询条件,时间范围为闭开区间 [StartAt, EndAt)。
type FlowQuery struct {
	StartAt   time.Time
	EndAt     time.Time
	OrderNo   string
	FlowNo    string
	Direction int8 // 0=全部 1=进账 2=出账
	Page      int
	PageSize  int
}

// ListFlows 资金流水分页(新流水在前)。
func (r *FinanceRepo) ListFlows(q FlowQuery) ([]model.PayFlow, int64, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 50 {
		q.PageSize = 10
	}
	tx := r.db.Model(&model.PayFlow{})
	if !q.StartAt.IsZero() && !q.EndAt.IsZero() {
		tx = tx.Where("created_at >= ? AND created_at < ?", q.StartAt, q.EndAt)
	}
	if q.OrderNo != "" {
		tx = tx.Where("order_no = ?", q.OrderNo)
	}
	if q.FlowNo != "" {
		tx = tx.Where("flow_no = ?", q.FlowNo)
	}
	if q.Direction != 0 {
		tx = tx.Where("direction = ?", q.Direction)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.PayFlow
	err := tx.Order("id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&list).Error
	return list, total, err
}

// FlowSum 区间成功流水汇总(进账/出账双口径)。
type FlowSum struct {
	PayAmount    int64
	RefundAmount int64
	PayCount     int64
	RefundCount  int64
}

// SumFlows 汇总 [start, end) 的进账/出账总额与笔数(日结与看板"今日"的数据源)。
func (r *FinanceRepo) SumFlows(start, end time.Time) (FlowSum, error) {
	var s FlowSum
	err := r.db.Model(&model.PayFlow{}).
		Select(`COALESCE(SUM(CASE WHEN direction = ? THEN amount ELSE 0 END), 0) AS pay_amount,
			COALESCE(SUM(CASE WHEN direction = ? THEN amount ELSE 0 END), 0) AS refund_amount,
			COALESCE(SUM(CASE WHEN direction = ? THEN 1 ELSE 0 END), 0) AS pay_count,
			COALESCE(SUM(CASE WHEN direction = ? THEN 1 ELSE 0 END), 0) AS refund_count`,
			model.FlowDirectionIn, model.FlowDirectionOut,
			model.FlowDirectionIn, model.FlowDirectionOut).
		Where("status = ? AND created_at >= ? AND created_at < ?",
			model.FlowStatusSuccess, start, end).
		Scan(&s).Error
	return s, err
}

// ListRefunds 售后单分页(status 为空查全部,新单在前)。
func (r *FinanceRepo) ListRefunds(status string, page, pageSize int) ([]model.RefundOrder, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}
	tx := r.db.Model(&model.RefundOrder{})
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.RefundOrder
	err := tx.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// CountPendingRefunds 待处理退款申请数(看板指标)。
func (r *FinanceRepo) CountPendingRefunds() (int64, error) {
	var n int64
	err := r.db.Model(&model.RefundOrder{}).
		Where("status = ?", model.RefundStatusPending).Count(&n).Error
	return n, err
}

// UpsertStatement 按 stat_date 幂等写入日结(重复执行覆盖为最新聚合值)。
func (r *FinanceRepo) UpsertStatement(st *model.DailyStatement) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "stat_date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"pay_amount", "refund_amount", "net_amount", "pay_count", "refund_count", "updated_at",
		}),
	}).Create(st).Error
}

// ListStatements 日结账单区间查询[from, to](YYYY-MM-DD 含端点),日期倒序。
func (r *FinanceRepo) ListStatements(from, to string) ([]model.DailyStatement, error) {
	var list []model.DailyStatement
	err := r.db.Where("stat_date >= ? AND stat_date <= ?", from, to).
		Order("stat_date DESC").Find(&list).Error
	return list, err
}

// ListStatementsPage 日结账单分页(新在前,对账管理页展示)。
func (r *FinanceRepo) ListStatementsPage(page, pageSize int) ([]model.DailyStatement, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	if err := r.db.Model(&model.DailyStatement{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.DailyStatement
	err := r.db.Order("stat_date DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// ReconcileSums 对账双口径汇总:订单侧 vs 流水侧。
// 进账口径:"曾经支付过"以 pay_time IS NOT NULL 为准(已退款订单仍计入历史进账);
// 退款口径:订单 status='refunded' vs 出账流水。
type ReconcileSums struct {
	OrderPay       int64
	FlowPay        int64
	OrderRefund    int64
	FlowRefund     int64
	OrderPayCnt    int64
	FlowPayCnt     int64
	OrderRefundCnt int64
	FlowRefundCnt  int64
}

// ReconcileSums 单条 SQL 汇总双口径(模拟支付数据量小,全表聚合可接受)。
func (r *FinanceRepo) ReconcileSums() (ReconcileSums, error) {
	var s ReconcileSums
	err := r.db.Raw(`
		SELECT
		  (SELECT COALESCE(SUM(pay_amount), 0) FROM orders WHERE pay_time IS NOT NULL) AS order_pay,
		  (SELECT COUNT(*) FROM orders WHERE pay_time IS NOT NULL)                     AS order_pay_cnt,
		  (SELECT COALESCE(SUM(amount), 0) FROM pay_flows WHERE biz_type = 'pay' AND status = 'success')    AS flow_pay,
		  (SELECT COUNT(*) FROM pay_flows WHERE biz_type = 'pay' AND status = 'success')                    AS flow_pay_cnt,
		  (SELECT COALESCE(SUM(pay_amount), 0) FROM orders WHERE status = 'refunded')  AS order_refund,
		  (SELECT COUNT(*) FROM orders WHERE status = 'refunded')                      AS order_refund_cnt,
		  (SELECT COALESCE(SUM(amount), 0) FROM pay_flows WHERE biz_type = 'refund' AND status = 'success') AS flow_refund,
		  (SELECT COUNT(*) FROM pay_flows WHERE biz_type = 'refund' AND status = 'success')                 AS flow_refund_cnt
	`).Scan(&s).Error
	return s, err
}

// ReconcileMismatch 对账差异明细(进账口径)。
type ReconcileMismatch struct {
	OrderNo     string `json:"order_no"`
	OrderAmount int64  `json:"order_amount"`
	FlowAmount  int64  `json:"flow_amount"`
	Reason      string `json:"reason"`
}

// MismatchList 进账口径差异明细(最多 limit 条):
// 已支付订单流水缺失/金额不符 + 有流水但订单未支付的孤儿流水。
func (r *FinanceRepo) MismatchList(limit int) ([]ReconcileMismatch, error) {
	var out []ReconcileMismatch
	err := r.db.Raw(`
		SELECT o.order_no, o.pay_amount AS order_amount, COALESCE(f.total, 0) AS flow_amount,
		       '已支付订单缺少进账流水或金额不符' AS reason
		FROM orders o
		LEFT JOIN (SELECT order_no, SUM(amount) AS total FROM pay_flows
		           WHERE biz_type = 'pay' AND status = 'success' GROUP BY order_no) f
		  ON f.order_no = o.order_no
		WHERE o.pay_time IS NOT NULL AND (f.order_no IS NULL OR f.total <> o.pay_amount)
		LIMIT ?`, limit).Scan(&out).Error
	if err != nil {
		return nil, err
	}
	var orphan []ReconcileMismatch
	err = r.db.Raw(`
		SELECT f.order_no, 0 AS order_amount, f.total AS flow_amount,
		       '存在进账流水但订单未支付' AS reason
		FROM (SELECT order_no, SUM(amount) AS total FROM pay_flows
		      WHERE biz_type = 'pay' AND status = 'success' GROUP BY order_no) f
		LEFT JOIN orders o ON o.order_no = f.order_no
		WHERE o.id IS NULL OR o.pay_time IS NULL
		LIMIT ?`, limit).Scan(&orphan).Error
	if err != nil {
		return nil, err
	}
	return append(out, orphan...), nil
}
