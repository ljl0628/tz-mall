package repository

import (
	"fawn-mall/internal/model"
	"time"

	"gorm.io/gorm"
)

// OrderRepo 订单数据访问。
type OrderRepo struct{ db *gorm.DB }

// NewOrderRepo 构造。
func NewOrderRepo(db *gorm.DB) *OrderRepo { return &OrderRepo{db: db} }

// CreateOrderWithDeduct 事务:逐项 MySQL 条件扣库存 → 创建订单与订单项。
// 任一尺码库存不足则整体回滚,返回 resp.ErrStockNotEnough。
// 这是「MySQL 侧再校验 + 事务防超卖」的最后一道防线。
func (r *OrderRepo) CreateOrderWithDeduct(order *model.Order, items []model.OrderItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, it := range items {
			ok, err := DeductStockTx(tx, it.ProductID, it.Size, it.Quantity)
			if err != nil {
				return err
			}
			if !ok {
				return respStockNotEnough(it)
			}
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].ID = 0
			items[i].OrderID = order.ID
		}
		order.Items = items
		return tx.Create(&items).Error
	})
}

// SaveCanceledOrder 消费侧库存不足时,落一条「已取消」订单供用户感知(不扣库存)。
func (r *OrderRepo) SaveCanceledOrder(order *model.Order, items []model.OrderItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].ID = 0
			items[i].OrderID = order.ID
		}
		order.Items = items
		return tx.Create(&items).Error
	})
}

// GetByOrderNo 按订单号查询(withItems 控制是否预加载明细)。
func (r *OrderRepo) GetByOrderNo(orderNo string, withItems bool) (*model.Order, error) {
	q := r.db.Where("order_no = ?", orderNo)
	if withItems {
		q = q.Preload("Items")
	}
	var o model.Order
	if err := q.First(&o).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &o, nil
}

// ListByUser 用户订单分页(status 为空查全部),附带明细。
func (r *OrderRepo) ListByUser(userID uint64, status string, page, pageSize int) ([]model.Order, int64, error) {
	return r.list(userID, status, page, pageSize, true)
}

// ListAll 管理端订单分页。
func (r *OrderRepo) ListAll(status string, page, pageSize int) ([]model.Order, int64, error) {
	return r.list(0, status, page, pageSize, false)
}

func (r *OrderRepo) list(userID uint64, status string, page, pageSize int, withUser bool) ([]model.Order, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}
	q := r.db.Model(&model.Order{})
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.Order
	err := q.Preload("Items").Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// markStatusCond 条件更新订单状态(乐观并发:仅 from 状态可迁移),返回是否更新成功。
func (r *OrderRepo) markStatusCond(orderNo, from, to string, sets map[string]any) (bool, error) {
	if sets == nil {
		sets = map[string]any{}
	}
	sets["status"] = to
	res := r.db.Model(&model.Order{}).
		Where("order_no = ? AND status = ?", orderNo, from).
		Updates(sets)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// MarkPaidWithFlow 事务:待支付 → 已支付,并在同事务写入进账流水,
// 保证支付状态与流水账实一致(要么同时落库,要么都不落)。
// 条件更新保证与超时取消并发时只有一方成功;未更新成功返回 false,不写流水。
func (r *OrderRepo) MarkPaidWithFlow(orderNo string, flow *model.PayFlow) (bool, error) {
	var paid bool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Order{}).
			Where("order_no = ? AND status = ?", orderNo, model.OrderStatusPendingPayment).
			Updates(map[string]any{"status": model.OrderStatusPaid, "pay_time": time.Now()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // 已支付过 / 已取消(可能刚好被延迟任务取消)
		}
		if err := tx.Create(flow).Error; err != nil {
			return err
		}
		paid = true
		return nil
	})
	return paid, err
}

// MarkShipped 已支付 → 已发货。
func (r *OrderRepo) MarkShipped(orderNo string) (bool, error) {
	return r.markStatusCond(orderNo, model.OrderStatusPaid, model.OrderStatusShipped,
		map[string]any{"ship_time": time.Now()})
}

// MarkCompleted 已发货 → 已完成(确认收货)。
func (r *OrderRepo) MarkCompleted(orderNo string) (bool, error) {
	return r.markStatusCond(orderNo, model.OrderStatusShipped, model.OrderStatusCompleted,
		map[string]any{"finish_time": time.Now()})
}

// MarkCanceled 待支付 → 已取消(用户取消 / 超时取消)。并发安全:
// 与支付(MarkPaidWithFlow)竞争时只有一方能更新成功。
func (r *OrderRepo) MarkCanceled(orderNo, reason string) (bool, error) {
	return r.markStatusCond(orderNo, model.OrderStatusPendingPayment, model.OrderStatusCanceled,
		map[string]any{"cancel_time": time.Now(), "cancel_reason": reason})
}

// CancelAndRefund 事务:条件更新订单为已取消,成功则同事务回补 MySQL 库存。
// 取消成功返回 true;订单已支付/已取消返回 false(不回补,防重复)。
func (r *OrderRepo) CancelAndRefund(orderNo, reason string, items []model.OrderItem) (bool, error) {
	var refunded bool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Order{}).
			Where("order_no = ? AND status = ?", orderNo, model.OrderStatusPendingPayment).
			Updates(map[string]any{
				"status":        model.OrderStatusCanceled,
				"cancel_time":   time.Now(),
				"cancel_reason": reason,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // 已被支付或已取消:不回补
		}
		for _, it := range items {
			if err := RefundStockTx(tx, it.ProductID, it.Size, it.Quantity); err != nil {
				return err
			}
		}
		refunded = true
		return nil
	})
	return refunded, err
}
