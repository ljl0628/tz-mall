package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go.uber.org/zap"

	"fawn-mall/internal/model"
	"fawn-mall/internal/mq"
	"fawn-mall/internal/repository"
	"fawn-mall/pkg/logger"
	"fawn-mall/pkg/resp"
)

// newConsumeCtx 每条消息一个带超时的上下文(防止单条消息长时间占用消费者)。
func newConsumeCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// HandleOrderCreate 消费 order.create 队列:
//  1. 幂等检查(订单号已存在直接确认)
//  2. MySQL 事务:逐项条件扣库存(防超卖最后防线)→ 创建订单及明细
//  3. 成功后投递延迟取消消息(15 分钟 TTL → 死信 → order.cancel)
//  4. MySQL 侧库存不足:落「已取消」订单供用户感知 + 回补 Redis 预扣 → 业务终态确认
//  5. 基础设施错误:返回错误交给 MQ 重试机制(重试超限进 parking 队列)
func (s *OrderService) HandleOrderCreate(body []byte) error {
	var msg mq.OrderCreateMsg
	if err := json.Unmarshal(body, &msg); err != nil {
		logger.Error(context.Background(), "order.create 消息解析失败,业务终态确认",
			zap.Error(err), zap.String("body", string(body)))
		return mq.ErrAck
	}

	// 幂等:重试消息可能重复投递
	if exist, err := s.orders.GetByOrderNo(msg.OrderNo, false); err == nil && exist != nil {
		logger.Info(context.Background(), "订单已存在,跳过重复消费", zap.String("order_no", msg.OrderNo))
		return nil
	}

	order := &model.Order{
		OrderNo:     msg.OrderNo,
		UserID:      msg.UserID,
		Status:      model.OrderStatusPendingPayment,
		TotalAmount: msg.TotalAmount,
		PayAmount:   msg.TotalAmount,
		Receiver:    msg.Receiver,
		Phone:       msg.Phone,
		Address:     msg.Address,
		Remark:      msg.Remark,
	}
	items := make([]model.OrderItem, 0, len(msg.Items))
	for _, it := range msg.Items {
		items = append(items, model.OrderItem{
			ProductID: it.ProductID, ProductName: it.ProductName, Image: it.Image,
			Size: it.Size, Price: it.Price, Quantity: it.Quantity,
		})
	}

	if err := s.orders.CreateOrderWithDeduct(order, items); err != nil {
		var be *resp.BizError
		if errors.As(err, &be) && be.Code == resp.CodeStockNotEnough {
			// MySQL 侧库存不足(Redis 与 MySQL 短暂不一致兜底):
			// 落「已取消」订单让用户可见,回补 Redis 预扣,消息业务终态确认
			s.saveInsufficientOrder(order, items, be.Msg)
			return mq.ErrAck
		}
		return err // 基础设施错误 → MQ 重试
	}

	logger.Info(context.Background(), "MQ 消费:订单已落库并扣减 MySQL 库存",
		zap.String("order_no", msg.OrderNo), zap.Uint64("user_id", msg.UserID),
		zap.String("phone", logger.MaskPhone(msg.Phone)),
		zap.Int64("amount", msg.TotalAmount), zap.Int("items", len(items)))

	// 投递延迟取消消息:进入 TTL 延迟队列,15 分钟后死信到取消队列
	ctx, cancel := newConsumeCtx()
	defer cancel()
	if err := s.pub.Publish(ctx, mq.QueueOrderDelay, mq.OrderCancelMsg{
		OrderNo:   msg.OrderNo,
		Reason:    CancelReasonTimeout,
		CreatedAt: time.Now(),
	}); err != nil {
		// 延迟消息发布失败不阻断订单(订单已有效);极端情况下超时取消缺失,
		// 用户仍可主动取消。记录错误日志供人工补偿。
		logger.Error(ctx, "延迟取消消息投递失败,请人工核对",
			zap.String("order_no", msg.OrderNo), zap.Error(err))
	}
	return nil
}

// saveInsufficientOrder MySQL 库存不足时落「已取消」订单并回补 Redis 预扣。
func (s *OrderService) saveInsufficientOrder(order *model.Order, items []model.OrderItem, why string) {
	ctx, cancel := newConsumeCtx()
	defer cancel()

	now := time.Now()
	order.Status = model.OrderStatusCanceled
	order.CancelTime = &now
	order.CancelReason = CancelReasonStock
	if err := s.orders.SaveCanceledOrder(order, items); err != nil {
		logger.Error(ctx, "保存库存不足取消订单失败",
			zap.String("order_no", order.OrderNo), zap.Error(err))
	}
	stockItems := make([]repository.StockItem, 0, len(items))
	for _, it := range items {
		stockItems = append(stockItems, repository.StockItem{
			ProductID: it.ProductID, Size: it.Size, Quantity: it.Quantity,
		})
	}
	if err := s.stock.RefundBatch(ctx, stockItems); err != nil {
		logger.Error(ctx, "库存不足回补 Redis 失败,请人工核对",
			zap.String("order_no", order.OrderNo), zap.Error(err))
	}
	logger.Warn(ctx, "MQ 消费:MySQL 库存不足,订单自动取消并回补 Redis",
		zap.String("order_no", order.OrderNo), zap.String("reason", why))
}

// HandleOrderCancel 消费 order.cancel 队列(延迟队列 TTL 到期死信而来):
// 订单仍为待支付则取消并回补库存;已支付/已取消/不存在均视为业务终态直接确认。
func (s *OrderService) HandleOrderCancel(body []byte) error {
	var msg mq.OrderCancelMsg
	if err := json.Unmarshal(body, &msg); err != nil {
		logger.Error(context.Background(), "order.cancel 消息解析失败,业务终态确认",
			zap.Error(err), zap.String("body", string(body)))
		return mq.ErrAck
	}
	ctx, cancel := newConsumeCtx()
	defer cancel()

	order, err := s.orders.GetByOrderNo(msg.OrderNo, true)
	if err != nil {
		return err // 基础设施错误 → 重试
	}
	if order == nil {
		logger.Info(ctx, "超时取消:订单不存在(下单未落库),确认", zap.String("order_no", msg.OrderNo))
		return mq.ErrAck
	}
	if order.Status != model.OrderStatusPendingPayment {
		logger.Info(ctx, "超时取消:订单已非待支付,确认",
			zap.String("order_no", msg.OrderNo), zap.String("status", order.Status))
		return mq.ErrAck
	}

	ok, err := s.CancelIfUnpaid(ctx, msg.OrderNo, msg.Reason)
	if err != nil {
		return err
	}
	if !ok {
		logger.Info(ctx, "超时取消:订单状态已变更(并发支付),确认", zap.String("order_no", msg.OrderNo))
	}
	return nil
}
