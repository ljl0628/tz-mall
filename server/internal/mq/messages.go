package mq

import "time"

// OrderCreateMsg 下单消息:下单接口完成 Redis 预扣后投递,
// 消费者负责订单落库 + MySQL 扣库存 + 投递延迟取消消息。
type OrderCreateMsg struct {
	OrderNo     string    `json:"order_no"`
	UserID      uint64    `json:"user_id"`
	Receiver    string    `json:"receiver"`
	Phone       string    `json:"phone"`
	Address     string    `json:"address"`
	Remark      string    `json:"remark"`
	Items       []MsgItem `json:"items"`
	TotalAmount int64     `json:"total_amount"` // 分
	CreatedAt   time.Time `json:"created_at"`
}

// MsgItem 下单消息中的商品项(含价格快照)。
type MsgItem struct {
	ProductID   uint64 `json:"product_id"`
	ProductName string `json:"product_name"`
	Image       string `json:"image"`
	Size        string `json:"size"`
	Price       int64  `json:"price"` // 分
	Quantity    int    `json:"quantity"`
}

// OrderCancelMsg 延迟取消消息:延迟队列 TTL 到期死信后由取消消费者处理。
type OrderCancelMsg struct {
	OrderNo   string    `json:"order_no"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}
