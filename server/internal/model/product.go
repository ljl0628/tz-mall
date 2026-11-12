package model

import "time"

// 年龄段常量(童装域模型)。
const (
	AgeBaby   = "baby"   // 婴幼儿 0-3 岁
	AgeLittle = "little" // 小童 3-6 岁
	AgeMiddle = "middle" // 中童 6-9 岁
	AgeBig    = "big"    // 大童 9-12 岁
)

// AgeGroups 有序的年龄段列表(展示顺序)。
var AgeGroups = []struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}{
	{AgeBaby, "婴幼儿 0-3岁"},
	{AgeLittle, "小童 3-6岁"},
	{AgeMiddle, "中童 6-9岁"},
	{AgeBig, "大童 9-12岁"},
}

// AgeGroupName 返回年龄段中文名。
func AgeGroupName(key string) string {
	for _, g := range AgeGroups {
		if g.Key == key {
			return g.Name
		}
	}
	return key
}

// 季节常量。
const (
	SeasonSpringSummer = "spring_summer"
	SeasonAutumnWinter = "autumn_winter"
)

// 商品上下架状态。
const (
	ProductStatusOff = 0
	ProductStatusOn  = 1
)

// Product 商品(童装)。
type Product struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	CategoryID  uint64    `gorm:"index;not null" json:"category_id"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	AgeGroup    string    `gorm:"size:16;not null;index" json:"age_group"`
	Material    string    `gorm:"size:64;not null" json:"material"`            // 面料材质:纯棉/莫代尔/棉麻/摇粒绒…
	Season      string    `gorm:"size:16;not null" json:"season"`              // 适用季节
	Description string    `gorm:"type:text" json:"description"`                // 图文描述
	Image       string    `gorm:"size:512" json:"image"`                       // 主图 URL(picsum 占位)
	Price       int64     `gorm:"not null" json:"price"`                       // 单位:分
	Status      int8      `gorm:"not null;default:1;index" json:"status"`      // 1 上架 / 0 下架
	Recommended int8      `gorm:"not null;default:0;index" json:"recommended"` // 1 首页推荐
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 显式表名。
func (Product) TableName() string { return "products" }

// ProductDetail 商品详情(含 SKU 列表与分类名,供缓存/接口使用)。
type ProductDetail struct {
	Product
	CategoryName string `json:"category_name"`
	SKUs         []SKU  `json:"skus"` // 按 ID 升序
}

// SKU 商品尺码库存:库存按「商品 + 尺码」维度管理。
type SKU struct {
	ID        uint64 `gorm:"primaryKey" json:"id"`
	ProductID uint64 `gorm:"not null;uniqueIndex:uk_product_size" json:"product_id"`
	Size      string `gorm:"size:16;not null;uniqueIndex:uk_product_size" json:"size"` // 90/100/110/…/160
	Stock     int    `gorm:"not null;default:0" json:"stock"`
}

// TableName 显式表名。
func (SKU) TableName() string { return "skus" }

// ProductFilter 商品列表查询条件。
type ProductFilter struct {
	Page        int    `form:"page"`
	PageSize    int    `form:"page_size"`
	CategoryID  uint64 `form:"category_id"`
	AgeGroup    string `form:"age_group"`
	Keyword     string `form:"keyword"`
	Status      *int8  `form:"status"` // 显式按状态过滤(管理端用)
	OnShelfOnly bool   `form:"-"`      // C 端固定只查上架商品
}
