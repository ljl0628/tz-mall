package model

import "time"

// 订单状态机:
//
//	pending_payment(待支付) → paid(已支付) → shipped(已发货) → completed(已完成)
//	pending_payment → canceled(已取消:用户取消 / 超时未支付)
const (
	OrderStatusPendingPayment = "pending_payment"
	OrderStatusPaid           = "paid"
	OrderStatusShipped        = "shipped"
	OrderStatusCompleted      = "completed"
	OrderStatusCanceled       = "canceled"
)

// OrderStatusName 订单状态中文名(前端时间轴展示备用)。
var OrderStatusName = map[string]string{
	OrderStatusPendingPayment: "待支付",
	OrderStatusPaid:           "已支付",
	OrderStatusShipped:        "已发货",
	OrderStatusCompleted:      "已完成",
	OrderStatusCanceled:       "已取消",
}

// Order 订单主表。
type Order struct {
	ID           uint64      `gorm:"primaryKey" json:"-"`
	OrderNo      string      `gorm:"size:32;uniqueIndex;not null" json:"order_no"`
	UserID       uint64      `gorm:"index;not null" json:"user_id"`
	Status       string      `gorm:"size:24;not null;index" json:"status"`
	TotalAmount  int64       `gorm:"not null" json:"total_amount"`     // 商品总额(分)
	PayAmount    int64       `gorm:"not null" json:"pay_amount"`       // 应付金额(分,本项目无运费=总额)
	Receiver     string      `gorm:"size:64;not null" json:"receiver"` // 收货人快照
	Phone        string      `gorm:"size:32;not null" json:"phone"`    // 手机号快照
	Address      string      `gorm:"size:255;not null" json:"address"` // 地址快照
	Remark       string      `gorm:"size:255" json:"remark"`
	PayTime      *time.Time  `json:"pay_time"`
	ShipTime     *time.Time  `json:"ship_time"`
	FinishTime   *time.Time  `json:"finish_time"`
	CancelTime   *time.Time  `json:"cancel_time"`
	CancelReason string      `gorm:"size:128" json:"cancel_reason"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
	Items        []OrderItem `json:"items,omitempty"`

	// PayRemainingSeconds Remaining payment seconds (computed field, not stored in DB):
	// service layer computes the value for pending-payment orders based on created_at + pay_timeout,
	// omitempty hides it for other statuses or when timeout is reached (0).
	PayRemainingSeconds int `gorm:"-" json:"pay_remaining_seconds,omitempty"`
}

// TableName 显式表名。
func (Order) TableName() string { return "orders" }

// OrderItem 订单商品项(下单时快照,不依赖商品后续修改)。
type OrderItem struct {
	ID          uint64 `gorm:"primaryKey" json:"-"`
	OrderID     uint64 `gorm:"index:idx_order;not null" json:"-"`
	ProductID   uint64 `json:"product_id"`
	ProductName string `gorm:"size:128" json:"product_name"`
	Image       string `gorm:"size:512" json:"image"`
	Size        string `gorm:"size:16" json:"size"`
	Price       int64  `json:"price"` // 成交单价快照(分)
	Quantity    int    `json:"quantity"`
}

// TableName 显式表名。
func (OrderItem) TableName() string { return "order_items" }
