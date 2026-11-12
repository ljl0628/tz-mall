package handler

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/go-playground/validator/v10"
)

// fieldLabels 校验字段的中文名;未收录的字段回退英文名,
// 保证提示可读的同时不再把 validator 内部错误(Key: 'X.Y' failed on tag)暴露给调用方。
var fieldLabels = map[string]string{
	"Username":    "用户名",
	"Password":    "密码",
	"Nickname":    "昵称",
	"Receiver":    "收货人",
	"Phone":       "手机号",
	"Province":    "省份",
	"City":        "城市",
	"District":    "区县",
	"Detail":      "详细地址",
	"Remark":      "订单备注",
	"Name":        "名称",
	"CategoryID":  "分类",
	"AgeGroup":    "适用年龄",
	"Material":    "面料",
	"Season":      "季节",
	"Description": "描述",
	"Image":       "商品图",
	"Price":       "价格",
	"Status":      "状态",
	"Recommended": "推荐标记",
	"SKUs":        "SKU 列表",
	"Size":        "尺码",
	"Stock":       "库存",
	"ProductID":   "商品",
	"Quantity":    "数量",
	"Sort":        "排序值",
	"Role":        "角色",
}

// bindErrMsg 将 gin 绑定/校验错误翻译为用户可读的中文提示。
// 非 validator 错误(JSON 解析失败/类型不符)给出通用提示。
func bindErrMsg(err error) string {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return "请求体格式不正确"
	}
	seen := make(map[string]struct{}, len(verrs))
	msgs := make([]string, 0, len(verrs))
	for _, fe := range verrs {
		m := fieldMsg(fe)
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		msgs = append(msgs, m)
	}
	sort.Strings(msgs) // 排序保证同一输入的提示稳定,便于测试与排查
	return strings.Join(msgs, ";")
}

func fieldMsg(fe validator.FieldError) string {
	name, ok := fieldLabels[fe.Field()]
	if !ok {
		name = fe.Field()
	}
	switch fe.Tag() {
	case "required":
		return name + "不能为空"
	case "min":
		return fmt.Sprintf("%s至少为 %s", name, fe.Param())
	case "max":
		return fmt.Sprintf("%s不能超过 %s", name, fe.Param())
	case "len":
		return fmt.Sprintf("%s必须为 %s 位", name, fe.Param())
	case "alphanum":
		return name + "仅支持字母或数字"
	case "numeric", "number":
		return name + "必须是数字"
	case "oneof":
		return fmt.Sprintf("%s仅支持 %s", name, strings.ReplaceAll(fe.Param(), " ", " / "))
	case "gt":
		return fmt.Sprintf("%s必须大于 %s", name, fe.Param())
	case "gte":
		return fmt.Sprintf("%s不能小于 %s", name, fe.Param())
	case "lt":
		return fmt.Sprintf("%s必须小于 %s", name, fe.Param())
	case "lte":
		return fmt.Sprintf("%s不能大于 %s", name, fe.Param())
	default:
		return name + "格式不正确(" + fe.Tag() + ")"
	}
}
