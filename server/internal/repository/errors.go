package repository

import (
	"fmt"

	"fawn-mall/internal/model"
	"fawn-mall/pkg/resp"
)

// respStockNotEnough 构造带商品明细的库存不足业务错误。
func respStockNotEnough(it model.OrderItem) error {
	return resp.NewBiz(resp.CodeStockNotEnough,
		fmt.Sprintf("商品「%s」尺码 %s 库存不足", it.ProductName, it.Size))
}
