package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"fawn-mall/internal/model"
	"fawn-mall/internal/mq"
	"fawn-mall/internal/repository"
	"fawn-mall/pkg/apperr"
	"fawn-mall/pkg/logger"
	"fawn-mall/pkg/resp"
)

// 取消原因常量。
const (
	CancelReasonUser    = "用户取消"
	CancelReasonTimeout = "超时未支付,自动取消"
	CancelReasonStock   = "库存不足,自动取消"
)

// OrderService 订单业务:下单(Redis 预扣 + MQ 异步落库)、支付、取消、查询。
type OrderService struct {
	orders   *repository.OrderRepo
	cart     *repository.CartRepo
	products *repository.ProductRepo
	stock    *repository.CacheStock
	pub      Publisher
}

// Publisher MQ 发布抽象(便于单测与解耦,实现方为 *mq.Client)。
type Publisher interface {
	Publish(ctx context.Context, queue string, v any) error
}

// NewOrderService 构造。
func NewOrderService(o *repository.OrderRepo, cart *repository.CartRepo,
	p *repository.ProductRepo, stock *repository.CacheStock, pub Publisher) *OrderService {
	return &OrderService{orders: o, cart: cart, products: p, stock: stock, pub: pub}
}

// ---------- 金额计算(纯函数,含单测) ----------

// 单订单金额上限:100 万元(分),防溢出与异常订单。
const maxOrderAmount int64 = 100_000_000

// CalcOrderAmount 计算订单总额 = Σ(单价 × 数量),单位分。
// 校验:单价/数量必须为正,单项与累计均不得超过单订单上限。
func CalcOrderAmount(items []model.OrderItem) (int64, error) {
	var total int64
	for _, it := range items {
		if it.Price <= 0 {
			return 0, fmt.Errorf("商品「%s」价格异常", it.ProductName)
		}
		if it.Quantity <= 0 {
			return 0, fmt.Errorf("商品「%s」数量必须大于 0", it.ProductName)
		}
		sub := it.Price * int64(it.Quantity)
		if sub <= 0 || sub > maxOrderAmount {
			return 0, fmt.Errorf("商品「%s」金额超出上限", it.ProductName)
		}
		total += sub
		if total > maxOrderAmount {
			return 0, errors.New("订单总金额超出上限")
		}
	}
	if total <= 0 {
		return 0, errors.New("订单金额必须大于 0")
	}
	return total, nil
}

// ---------- 下单 ----------

// PlaceReq 下单请求(收货信息,商品取自购物车勾选项)。
type PlaceReq struct {
	Receiver string `json:"receiver" binding:"required,min=2,max=32"`
	Phone    string `json:"phone" binding:"required,len=11,numeric"`
	Address  string `json:"address" binding:"required,min=2,max=255"`
	Remark   string `json:"remark" binding:"max=200"`
}

// PlaceResp 下单响应。
type PlaceResp struct {
	OrderNo     string `json:"order_no"`
	TotalAmount int64  `json:"total_amount"`
}

// Place 下单主链路:
//  1. 从购物车取勾选商品(服务端价格快照,防篡改)
//  2. Redis Lua 批量预扣库存(原子:任一不足则全不扣)
//  3. 订单消息投递 RabbitMQ,消费者落库 MySQL 并再校验扣库存(防超卖)
//  4. 清理购物车已结算项,立即返回订单号
func (s *OrderService) Place(ctx context.Context, userID uint64, req PlaceReq) (*PlaceResp, error) {
	cartItems, err := s.cart.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	selected := make([]repository.CartItem, 0, len(cartItems))
	for _, it := range cartItems {
		if it.Selected {
			selected = append(selected, it)
		}
	}
	if len(selected) == 0 {
		return nil, resp.ErrCartEmpty
	}

	// 服务端取实时商品信息做价格快照
	items := make([]model.OrderItem, 0, len(selected))
	stockItems := make([]repository.StockItem, 0, len(selected))
	nameOf := make(map[string]string, len(selected)) // "{pid}:{size}" → 商品名(错误提示用)
	for _, c := range selected {
		p, err := s.products.GetByID(c.ProductID)
		if err != nil {
			return nil, err
		}
		if p == nil || p.Status != model.ProductStatusOn {
			return nil, resp.NewBiz(resp.CodeProductNotFound,
				fmt.Sprintf("商品「%s」已下架,请重新选择", c.Name))
		}
		items = append(items, model.OrderItem{
			ProductID:   c.ProductID,
			ProductName: p.Name,
			Image:       p.Image,
			Size:        c.Size,
			Price:       p.Price,
			Quantity:    c.Quantity,
		})
		stockItems = append(stockItems, repository.StockItem{
			ProductID: c.ProductID, Size: c.Size, Quantity: c.Quantity,
		})
		nameOf[fmt.Sprintf("%d:%s", c.ProductID, c.Size)] = p.Name
	}

	total, err := CalcOrderAmount(items)
	if err != nil {
		return nil, resp.NewBiz(resp.CodeParamError, err.Error())
	}

	// Redis Lua 原子预扣库存
	if err := s.stock.DeductBatch(ctx, stockItems, func(it repository.StockItem) string {
		name := nameOf[fmt.Sprintf("%d:%s", it.ProductID, it.Size)]
		return fmt.Sprintf("商品「%s」尺码 %s", name, it.Size)
	}); err != nil {
		return nil, resp.NewBiz(resp.CodeStockNotEnough, "很抱歉,"+err.Error())
	}

	orderNo := GenOrderNo()
	msg := mq.OrderCreateMsg{
		OrderNo: orderNo, UserID: userID,
		Receiver: req.Receiver, Phone: req.Phone, Address: req.Address, Remark: req.Remark,
		Items: toMsgItems(items), TotalAmount: total, CreatedAt: time.Now(),
	}
	if err := s.pub.Publish(ctx, mq.QueueOrderCreate, msg); err != nil {
		// 发布失败:回补 Redis 预扣库存,保证不白白占用
		if rerr := s.stock.RefundBatch(ctx, stockItems); rerr != nil {
			logger.Error(ctx, "下单 MQ 发布失败且回补库存失败,请人工核对",
				zap.String("order_no", orderNo), zap.Error(rerr))
		}
		return nil, apperr.Wrap("下单发布 order.create", fmt.Errorf("下单失败,请稍后重试: %w", err))
	}

	// 清理购物车中已结算的项
	fields := make([]string, 0, len(selected))
	for _, c := range selected {
		fields = append(fields, c.Field())
	}
	if err := s.cart.RemoveFields(ctx, userID, fields); err != nil {
		logger.Warn(ctx, "下单成功但清理购物车失败",
			zap.String("order_no", orderNo), zap.Error(err))
	}

	logger.Info(ctx, "下单受理成功,等待 MQ 消费落库",
		zap.String("order_no", orderNo), zap.Uint64("user_id", userID),
		zap.Int64("total", total), zap.Int("items", len(items)))
	return &PlaceResp{OrderNo: orderNo, TotalAmount: total}, nil
}

// GenOrderNo 生成订单号:FK + yyyyMMddHHmmss + 6 位随机十六进制。
func GenOrderNo() string {
	return "FK" + time.Now().Format("20060102150405") + randomHex(3)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func toMsgItems(items []model.OrderItem) []mq.MsgItem {
	out := make([]mq.MsgItem, 0, len(items))
	for _, it := range items {
		out = append(out, mq.MsgItem{
			ProductID: it.ProductID, ProductName: it.ProductName, Image: it.Image,
			Size: it.Size, Price: it.Price, Quantity: it.Quantity,
		})
	}
	return out
}

// ---------- 支付 / 取消 / 确认收货 ----------

// Pay 模拟支付:待支付 → 已支付。与超时取消并发时,条件更新保证只有一方成功。
func (s *OrderService) Pay(ctx context.Context, userID uint64, orderNo string) (*model.Order, error) {
	order, err := s.getOwnedOrder(orderNo, userID, true)
	if err != nil {
		return nil, err
	}
	ok, err := s.orders.MarkPaid(orderNo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, resp.ErrOrderStatus // 已支付过 / 已取消(可能刚好被延迟任务取消)
	}
	logger.Info(ctx, "模拟支付成功", zap.String("order_no", orderNo),
		zap.Uint64("user_id", userID), zap.Int64("amount", order.PayAmount))
	return s.orders.GetByOrderNo(orderNo, true)
}

// Cancel 用户取消订单(仅待支付),取消后回补 Redis 与 MySQL 库存。
func (s *OrderService) Cancel(ctx context.Context, userID uint64, orderNo string) (*model.Order, error) {
	order, err := s.getOwnedOrder(orderNo, userID, true)
	if err != nil {
		return nil, err
	}
	if order.Status != model.OrderStatusPendingPayment {
		return nil, resp.ErrOrderStatus
	}
	ok, err := s.CancelIfUnpaid(ctx, orderNo, CancelReasonUser)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, resp.ErrOrderStatus
	}
	return s.orders.GetByOrderNo(orderNo, true)
}

// Confirm 确认收货:已发货 → 已完成。
func (s *OrderService) Confirm(ctx context.Context, userID uint64, orderNo string) (*model.Order, error) {
	if _, err := s.getOwnedOrder(orderNo, userID, false); err != nil {
		return nil, err
	}
	ok, err := s.orders.MarkCompleted(orderNo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, resp.ErrOrderStatus
	}
	return s.orders.GetByOrderNo(orderNo, true)
}

// CancelIfUnpaid 取消未支付订单并回补库存(超时取消/用户取消共用,幂等):
// MySQL 事务内条件更新状态 + 回补 MySQL 库存;成功后回补 Redis 库存。
// 返回是否真正执行了取消。
func (s *OrderService) CancelIfUnpaid(ctx context.Context, orderNo, reason string) (bool, error) {
	order, err := s.orders.GetByOrderNo(orderNo, true)
	if err != nil {
		return false, err
	}
	if order == nil {
		return false, resp.ErrOrderNotFound
	}

	refunded, err := s.orders.CancelAndRefund(orderNo, reason, order.Items)
	if err != nil {
		return false, err
	}
	if !refunded {
		return false, nil
	}

	// 回补 Redis 预扣库存(带重试;Redis 失败仅影响可售余量偏保守,不会超卖)
	stockItems := make([]repository.StockItem, 0, len(order.Items))
	for _, it := range order.Items {
		stockItems = append(stockItems, repository.StockItem{
			ProductID: it.ProductID, Size: it.Size, Quantity: it.Quantity,
		})
	}
	var rerr error
	for attempt := 0; attempt < 3; attempt++ {
		if rerr = s.stock.RefundBatch(ctx, stockItems); rerr == nil {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
	}
	if rerr != nil {
		logger.Error(ctx, "订单已取消但 Redis 库存回补失败,请人工核对",
			zap.String("order_no", orderNo), zap.Error(rerr))
	} else {
		logger.Info(ctx, "订单取消完成,MySQL 与 Redis 库存已回补",
			zap.String("order_no", orderNo), zap.String("reason", reason))
	}
	return true, nil
}

// ---------- 查询 ----------

// List 我的订单分页。
func (s *OrderService) List(ctx context.Context, userID uint64, status string, page, pageSize int) (*PageList, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}
	if status != "" && !validOrderStatus(status) {
		return nil, resp.NewBiz(resp.CodeParamError, "订单状态非法")
	}
	list, total, err := s.orders.ListByUser(userID, status, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &PageList{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

// Detail 订单详情(校验归属)。
func (s *OrderService) Detail(ctx context.Context, userID uint64, orderNo string) (*model.Order, error) {
	return s.getOwnedOrder(orderNo, userID, true)
}

func (s *OrderService) getOwnedOrder(orderNo string, userID uint64, withItems bool) (*model.Order, error) {
	order, err := s.orders.GetByOrderNo(orderNo, withItems)
	if err != nil {
		return nil, err
	}
	if order == nil || order.UserID != userID {
		return nil, resp.ErrOrderNotFound
	}
	return order, nil
}

func validOrderStatus(s string) bool {
	switch s {
	case model.OrderStatusPendingPayment, model.OrderStatusPaid,
		model.OrderStatusShipped, model.OrderStatusCompleted, model.OrderStatusCanceled:
		return true
	}
	return false
}
