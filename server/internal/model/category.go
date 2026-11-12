package model

// Category 商品分类(童装品类:连体衣/套装/上衣/裤装/裙装/外套/配饰)。
type Category struct {
	ID   uint64 `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:32;uniqueIndex;not null" json:"name"`
	Sort int    `gorm:"not null;default:0" json:"sort"` // 越小越靠前
}
