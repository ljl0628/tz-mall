package service

import (
	"testing"

	"fawn-mall/internal/model"
)

// TestCalcOrderAmount 常规多商品合计。
func TestCalcOrderAmount(t *testing.T) {
	items := []model.OrderItem{
		{ProductName: "条纹T恤", Price: 4900, Quantity: 2},
		{ProductName: "背带裤", Price: 9900, Quantity: 1},
		{ProductName: "棒球帽", Price: 3500, Quantity: 3},
	}
	// 4900*2 + 9900 + 3500*3 = 9800 + 9900 + 10500 = 30200
	got, err := CalcOrderAmount(items)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}
	if got != 30200 {
		t.Errorf("金额 = %d,期望 30200", got)
	}
}

// TestCalcOrderAmount_RejectsBadInput 非法单价/数量必须报错。
func TestCalcOrderAmount_RejectsBadInput(t *testing.T) {
	cases := []struct {
		name  string
		items []model.OrderItem
	}{
		{"零价格", []model.OrderItem{{Price: 0, Quantity: 1}}},
		{"负价格", []model.OrderItem{{Price: -100, Quantity: 1}}},
		{"零数量", []model.OrderItem{{Price: 1000, Quantity: 0}}},
		{"负数量", []model.OrderItem{{Price: 1000, Quantity: -2}}},
		{"单项超额", []model.OrderItem{{Price: maxOrderAmount, Quantity: 2}}},
		{"累计超额", []model.OrderItem{
			{Price: 60_000_000, Quantity: 1}, {Price: 60_000_000, Quantity: 1},
		}},
		{"空清单", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := CalcOrderAmount(c.items); err == nil {
				t.Errorf("%s 应返回错误", c.name)
			}
		})
	}
}

// TestGenOrderNo 订单号格式与前缀。
func TestGenOrderNo(t *testing.T) {
	no := GenOrderNo()
	if len(no) != 2+14+6 {
		t.Errorf("订单号长度 = %d(%s),期望 22", len(no), no)
	}
	if no[:2] != "FK" {
		t.Errorf("订单号前缀 = %s,期望 FK", no[:2])
	}
	if GenOrderNo() == no {
		t.Error("连续生成的订单号不应重复")
	}
}
