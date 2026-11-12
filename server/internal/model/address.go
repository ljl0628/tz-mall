package model

import "time"

// Address 用户收货地址簿:下单时快速选择,下单后订单内仅存组合快照,两者互不影响。
type Address struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	UserID    uint64    `gorm:"index;not null" json:"user_id"`
	Receiver  string    `gorm:"size:32;not null" json:"receiver"`
	Phone     string    `gorm:"size:32;not null" json:"phone"`
	Province  string    `gorm:"size:32;not null" json:"province"` // 省(快照文本)
	City      string    `gorm:"size:32;not null" json:"city"`     // 市
	District  string    `gorm:"size:32;not null" json:"district"` // 区/县
	Detail    string    `gorm:"size:255;not null" json:"detail"`  // 详细地址(街道门牌)
	IsDefault bool      `gorm:"not null;default:false" json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 显式表名。
func (Address) TableName() string { return "addresses" }
