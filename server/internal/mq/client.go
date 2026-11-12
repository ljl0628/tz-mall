// Package mq RabbitMQ 客户端:拓扑声明、发布(带确认与重试)、
// 消费(手动 ack、失败重试、超限进入 parking 队列)、断线自动重连。
package mq

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"fawn-mall/pkg/logger"
)

// 队列与交换机拓扑。
const (
	QueueOrderCreate = "fawn.order.create"  // 下单落库队列(异步创建订单 + MySQL 扣库存)
	QueueOrderDelay  = "fawn.order.delay"   // 延迟队列:TTL 到期后死信到 cancel 队列
	QueueOrderCancel = "fawn.order.cancel"  // 超时取消队列(回补库存)
	QueueParking     = "fawn.order.parking" // 重试超限的停车队列(人工排查)
	ExchangeDLX      = "fawn.order.dlx"     // 死信交换机
	RoutingCancel    = "fawn.cancel"

	publishRetryWait = time.Second // 发布失败重连后的等待
	publishAttempts  = 3
)

// ErrAck 业务终态错误:消息应答(不重试),仅记录日志。
// 典型场景:延迟取消消息到达但订单已支付——正常业务路径,不应重试。
var ErrAck = errors.New("业务终态,直接确认")

// Handler 消费处理器:返回 nil 或 ErrAck 表示可 ack;其他错误进入重试逻辑。
type Handler func(body []byte) error

// Client RabbitMQ 客户端。
type Client struct {
	url               string
	delayTTL          int // 延迟队列消息 TTL(毫秒)
	maxRetry          int
	reconnectInterval time.Duration
	opTimeout         time.Duration

	mu       sync.Mutex
	conn     *amqp.Connection
	ch       *amqp.Channel
	healthy  bool
	handlers map[string]Handler
	closed   bool
}

// New 创建客户端(尚未连接,调用 Run 后自动连接与重连)。
// 超时参数均来自配置(rabbitmq.reconnect_interval / op_timeout)。
func New(url string, payTimeoutSeconds, maxRetry int, reconnectInterval, opTimeout time.Duration) *Client {
	return &Client{
		url:               url,
		delayTTL:          payTimeoutSeconds * 1000,
		maxRetry:          maxRetry,
		reconnectInterval: reconnectInterval,
		opTimeout:         opTimeout,
		handlers:          make(map[string]Handler),
	}
}

// RegisterHandler 注册某队列的消费处理器(须在 Run 之前调用)。
func (c *Client) RegisterHandler(queue string, h Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[queue] = h
}

// Run 阻塞运行:建立连接 → 声明拓扑 → 启动消费者;连接断开后自动重连。
// 由调用方以 goroutine 方式启动,ctx 取消时退出。
func (c *Client) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if err := c.connectOnce(); err != nil {
			logger.Error(ctx, "RabbitMQ 连接失败,稍后重试", zap.Error(err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(c.reconnectInterval):
			}
			continue
		}
		logger.Info(ctx, "RabbitMQ 已连接", zap.Int("delay_ttl_ms", c.delayTTL))

		// 启动全部已注册消费者
		c.mu.Lock()
		subs := make(map[string]Handler, len(c.handlers))
		for q, h := range c.handlers {
			subs[q] = h
		}
		c.mu.Unlock()
		var wg sync.WaitGroup
		for q, h := range subs {
			wg.Add(1)
			go func(queue string, h Handler) {
				defer wg.Done()
				c.consume(ctx, queue, h)
			}(q, h)
		}

		// 等待连接或通道关闭(消费者随之退出),然后重连
		connClose := c.conn.NotifyClose(make(chan *amqp.Error, 1))
		select {
		case <-ctx.Done():
			c.Close()
			wg.Wait()
			return
		case <-connClose:
			logger.Warn(ctx, "RabbitMQ 连接断开,准备重连")
			c.setHealthy(false)
			wg.Wait()
			select {
			case <-ctx.Done():
				return
			case <-time.After(c.reconnectInterval):
			}
		}
	}
}

// Healthy 当前是否处于已连接状态。
func (c *Client) Healthy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.healthy
}

func (c *Client) setHealthy(v bool) {
	c.mu.Lock()
	c.healthy = v
	c.mu.Unlock()
}

// connectOnce 建立连接与通道并声明拓扑。
func (c *Client) connectOnce() error {
	conn, err := amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("amqp 拨号: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("创建 channel: %w", err)
	}
	// 开启 publisher confirm,发布时可等待 broker 确认
	if err := ch.Confirm(false); err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("开启 confirm 模式: %w", err)
	}
	if err := declareTopology(ch, c.delayTTL); err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("声明拓扑: %w", err)
	}
	c.mu.Lock()
	c.conn, c.ch = conn, ch
	c.healthy = true
	c.mu.Unlock()
	return nil
}

// declareTopology 声明交换机与队列(幂等)。
func declareTopology(ch *amqp.Channel, delayTTLMS int) error {
	if err := ch.ExchangeDeclare(ExchangeDLX, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明死信交换机: %w", err)
	}
	// 延迟队列:消息 TTL 到期后经死信交换机转入取消队列
	if _, err := ch.QueueDeclare(QueueOrderDelay, true, false, false, false, amqp.Table{
		"x-message-ttl":             int32(delayTTLMS), // 未支付超时时间
		"x-dead-letter-exchange":    ExchangeDLX,
		"x-dead-letter-routing-key": RoutingCancel,
	}); err != nil {
		return fmt.Errorf("声明延迟队列: %w", err)
	}
	for _, q := range []string{QueueOrderCreate, QueueOrderCancel, QueueParking} {
		if _, err := ch.QueueDeclare(q, true, false, false, false, nil); err != nil {
			return fmt.Errorf("声明队列 %s: %w", q, err)
		}
	}
	return ch.QueueBind(QueueOrderCancel, RoutingCancel, ExchangeDLX, false, nil)
}

// Publish 向指定队列发布 JSON 消息(publisher confirm + 失败重连重试)。
func (c *Client) Publish(ctx context.Context, queue string, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.PublishRaw(ctx, queue, body, nil)
}

// PublishRaw 发布原始消息体;headers 用于重试计数。
func (c *Client) PublishRaw(ctx context.Context, queue string, body []byte, headers amqp.Table) error {
	var lastErr error
	for attempt := 0; attempt < publishAttempts; attempt++ {
		err := c.publishOnce(ctx, queue, body, headers)
		if err == nil {
			return nil
		}
		lastErr = err
		logger.Warn(ctx, "MQ 发布失败,尝试重连后重发",
			zap.String("queue", queue), zap.Int("attempt", attempt+1), zap.Error(err))
		c.forceReconnect()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("发布到 %s 失败(已重试 %d 次): %w", queue, publishAttempts, lastErr)
}

func (c *Client) publishOnce(ctx context.Context, queue string, body []byte, headers amqp.Table) error {
	c.mu.Lock()
	ch := c.ch
	c.mu.Unlock()
	if ch == nil {
		return errors.New("channel 未就绪")
	}
	confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx, "",
		queue, false, false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent, // 持久化消息
			ContentType:  "application/json",
			Body:         body,
			Headers:      headers,
			MessageId:    randomID(),
			Timestamp:    time.Now(),
		})
	if err != nil {
		return err
	}
	if confirm == nil {
		return errors.New("confirm 为空")
	}
	if _, err := confirm.WaitContext(ctx); err != nil {
		return fmt.Errorf("等待 broker 确认: %w", err)
	}
	return nil
}

// forceReconnect 立即重建连接(发布/消费失败时调用)。
func (c *Client) forceReconnect() {
	c.mu.Lock()
	old := c.conn
	c.conn, c.ch = nil, nil
	c.healthy = false
	c.mu.Unlock()
	if old != nil {
		old.Close() // 触发 NotifyClose,Run 主循环将重建连接与消费者
	}
}

// consume 单队列消费者:手动 ack;失败消息带 x-retry 计数重发,
// 超过最大重试次数后转入 parking 队列并确认原消息,避免无限循环。
func (c *Client) consume(ctx context.Context, queue string, h Handler) {
	for {
		c.mu.Lock()
		ch := c.ch
		c.mu.Unlock()
		if ch == nil || ch.IsClosed() {
			return // 连接已断,由 Run 负责重建
		}
		// Qos 必须在 Consume 之前设置,prefetch=1 保证逐条手动 ack
		if err := ch.Qos(1, 0, false); err != nil {
			logger.Error(ctx, "MQ 设置 Qos 失败", zap.String("queue", queue), zap.Error(err))
			return
		}
		deliveries, err := ch.Consume(queue, "", false, false, false, false, nil)
		if err != nil {
			logger.Error(ctx, "MQ 消费启动失败", zap.String("queue", queue), zap.Error(err))
			return
		}
		logger.Info(ctx, "MQ 消费者启动", zap.String("queue", queue))

		for d := range deliveries {
			c.handleMessage(ctx, ch, queue, d, h)
		}
		// deliveries 关闭:连接断开,退出由 Run 重启
		logger.Warn(ctx, "MQ 消费通道关闭", zap.String("queue", queue))
		return
	}
}

// handleMessage 单条消息的分发与应答;ctx 来自消费者 goroutine(Run 的生命周期)。
func (c *Client) handleMessage(ctx context.Context, ch *amqp.Channel, queue string, d amqp.Delivery, h Handler) {
	start := time.Now()
	err := safeHandle(h, d.Body)
	if err == nil {
		_ = d.Ack(false)
		return
	}
	if errors.Is(err, ErrAck) {
		logger.Info(ctx, "MQ 消息业务终态,确认",
			zap.String("queue", queue), zap.String("msg_id", d.MessageId))
		_ = d.Ack(false)
		return
	}

	// 基础设施类失败:重试
	retry := retryCount(d.Headers) + 1
	if retry > c.maxRetry {
		logger.Error(ctx, "MQ 消费重试超限,转入停车队列", zap.String("queue", queue),
			zap.String("msg_id", d.MessageId), zap.Int("retry_count", retry),
			zap.Error(err), zap.String("body", string(d.Body)))
		parkCtx, cancel := context.WithTimeout(context.Background(), c.opTimeout)
		if perr := c.PublishRaw(parkCtx, QueueParking, d.Body, amqp.Table{
			"x-origin-queue": queue,
			"x-last-error":   err.Error(),
		}); perr != nil {
			logger.Error(ctx, "转入停车队列失败,消息将重新入队", zap.Error(perr))
			_ = d.Nack(false, true) // 宁可重复也不能丢
			cancel()
			return
		}
		cancel()
		_ = d.Ack(false)
		return
	}

	logger.Warn(ctx, "MQ 消费失败,延迟重试", zap.String("queue", queue),
		zap.String("msg_id", d.MessageId), zap.Int("retry_count", retry),
		zap.Int64("latency_ms", time.Since(start).Milliseconds()), zap.Error(err))
	// 重发一条带新计数的新消息,确认旧消息(避免 requeue 死循环且保证不丢)
	pubCtx, cancel := context.WithTimeout(context.Background(), c.opTimeout)
	if perr := c.PublishRaw(pubCtx, queue, d.Body, amqp.Table{"x-retry": int32(retry)}); perr != nil {
		logger.Error(ctx, "重发失败,消息将重新入队", zap.Error(perr))
		_ = d.Nack(false, true)
		cancel()
		return
	}
	cancel()
	_ = d.Ack(false)
}

// safeHandle 防 panic 传导导致消费者 goroutine 崩溃。
func safeHandle(h Handler, body []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("消费处理器 panic: %v", r)
		}
	}()
	return h(body)
}

func retryCount(h amqp.Table) int {
	if v, ok := h["x-retry"]; ok {
		switch n := v.(type) {
		case int32:
			return int(n)
		case int64:
			return int(n)
		case int:
			return n
		}
	}
	return 0
}

func randomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Close 关闭连接。
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	if c.ch != nil {
		c.ch.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
	c.healthy = false
}
