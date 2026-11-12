package model

import "time"

// 资金流水方向(商户视角)。
const (
	FlowDirectionIn  int8 = 1 // 进账:支付成功
	FlowDirectionOut int8 = 2 // 出账:退款成功
)

// 流水业务类型。
const (
	FlowBizPay    = "pay"    // 支付
	FlowBizRefund = "refund" // 退款
)

// FlowStatusSuccess 流水状态:模拟支付场景即时成功,预留字段便于接真实渠道。
const FlowStatusSuccess = "success"

// PayFlow 资金流水(insert-only 账本):进账/出账都是追加记录,
// 退款绝不修改或删除原支付流水,保证对账可追溯。
type PayFlow struct {
	ID        uint64    `gorm:"primaryKey" json:"-"`
	FlowNo    string    `gorm:"size:32;uniqueIndex;not null" json:"flow_no"`
	OrderNo   string    `gorm:"size:32;index;not null" json:"order_no"`
	UserID    uint64    `gorm:"not null" json:"user_id"`
	Direction int8      `gorm:"not null" json:"direction"`        // 1=进账 2=出账
	BizType   string    `gorm:"size:16;not null" json:"biz_type"` // pay / refund
	Amount    int64     `gorm:"not null" json:"amount"`           // 金额(分),恒为正
	Status    string    `gorm:"size:16;not null;default:success" json:"status"`
	Remark    string    `gorm:"size:255" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 显式表名。
func (PayFlow) TableName() string { return "pay_flows" }

// 退款售后单状态。
const (
	RefundStatusPending  = "pending"  // 待处理(用户申请,等管理员审核)
	RefundStatusRefunded = "refunded" // 已退款(管理员直接退款 / 审核通过执行退款)
	RefundStatusRejected = "rejected" // 已拒绝(管理员驳回申请)
)

// RefundOrder 退款售后单:一笔订单至多一条有效(已退款)售后单,
// 防重复由订单状态条件更新兜底,这里只做记录与展示。
type RefundOrder struct {
	ID          uint64     `gorm:"primaryKey" json:"-"`
	RefundNo    string     `gorm:"size:32;uniqueIndex;not null" json:"refund_no"`
	OrderNo     string     `gorm:"size:32;index;not null" json:"order_no"`
	UserID      uint64     `gorm:"not null" json:"user_id"`
	Amount      int64      `gorm:"not null" json:"amount"` // 退款金额(分),整单全额退
	Reason      string     `gorm:"size:255" json:"reason"`
	Status      string     `gorm:"size:16;not null;index;default:pending" json:"status"`
	OperatorID  uint64     `json:"operator_id"` // 处理人(管理员用户 ID,0=尚未处理)
	CreatedAt   time.Time  `json:"created_at"`
	ProcessedAt *time.Time `json:"processed_at"`
}

// TableName 显式表名。
func (RefundOrder) TableName() string { return "refund_orders" }

// DailyStatement 日结账单:每天一条,定时任务(或手动触发)聚合当日流水,
// 按 stat_date 幂等 upsert;看板/趋势图优先读本表,避免实时扫订单表。
type DailyStatement struct {
	ID           uint64    `gorm:"primaryKey" json:"-"`
	StatDate     string    `gorm:"type:date;uniqueIndex;not null" json:"stat_date"` // YYYY-MM-DD
	PayAmount    int64     `gorm:"not null;default:0" json:"pay_amount"`            // 当日进账(分)
	RefundAmount int64     `gorm:"not null;default:0" json:"refund_amount"`         // 当日出账(分)
	NetAmount    int64     `gorm:"not null;default:0" json:"net_amount"`            // 净收入 = 进账 - 出账(分)
	PayCount     int       `gorm:"not null;default:0" json:"pay_count"`
	RefundCount  int       `gorm:"not null;default:0" json:"refund_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 显式表名。
func (DailyStatement) TableName() string { return "daily_statements" }
