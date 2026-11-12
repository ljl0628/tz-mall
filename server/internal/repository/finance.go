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
