package mq

import (
	"context"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"fawn-mall/internal/testutil"
)

// TestDelayQueue_DeadLetter 端到端验证「TTL + 死信队列」延迟链路(真实 RabbitMQ):
// 消息进入 1 秒 TTL 的延迟队列 → 到期死信到 cancel 队列 → 消费者收到。
// 生产环境的 15 分钟超时取消复用同一机制,仅 TTL 不同。
func TestDelayQueue_DeadLetter(t *testing.T) {
	conn, err := amqp.Dial(testutil.RabbitMQURL())
	if err != nil {
		t.Fatalf("连接真实 RabbitMQ 失败(先 task infra): %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("创建 channel: %v", err)
	}
	defer ch.Close()

	const (
		testDLX    = "fawn.test.dlx"
		testDelay  = "fawn.test.delay"
		testCancel = "fawn.test.cancel"
	)
	ctx := context.Background()

	// 清理上次残留
	ch.ExchangeDelete(testDLX, false, false)
	ch.QueueDelete(testDelay, false, false, false)
	ch.QueueDelete(testCancel, false, false, false)

	// 声明与生产一致的拓扑结构(TTL 缩短为 1 秒)
	if err := ch.ExchangeDeclare(testDLX, "direct", false, true, false, false, nil); err != nil {
		t.Fatalf("声明死信交换机: %v", err)
	}
	if _, err := ch.QueueDeclare(testDelay, false, true, false, false, amqp.Table{
		"x-message-ttl":             int32(1000),
		"x-dead-letter-exchange":    testDLX,
		"x-dead-letter-routing-key": "cancel",
	}); err != nil {
		t.Fatalf("声明延迟队列: %v", err)
	}
	if _, err := ch.QueueDeclare(testCancel, false, true, false, false, nil); err != nil {
		t.Fatalf("声明取消队列: %v", err)
	}
	if err := ch.QueueBind(testCancel, "cancel", testDLX, false, nil); err != nil {
		t.Fatalf("绑定取消队列: %v", err)
	}
	t.Cleanup(func() {
		ch.ExchangeDelete(testDLX, false, false)
		ch.QueueDelete(testDelay, false, false, false)
		ch.QueueDelete(testCancel, false, false, false)
	})

	// 开启 confirm 模式后再发布
	if err := ch.Confirm(false); err != nil {
		t.Fatalf("开启 confirm: %v", err)
	}
	confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx, "", testDelay, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        []byte(`{"order_no":"FKTEST-DELAY-001"}`),
		Timestamp:   time.Now(),
	})
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	if _, err := confirm.WaitContext(ctx); err != nil {
		t.Fatalf("等待确认失败: %v", err)
	}
	publishedAt := time.Now()

	// 消费取消队列:必须约 1 秒后收到(死信),且消息体不丢
	msgs, err := ch.Consume(testCancel, "", true, false, false, false, nil)
	if err != nil {
		t.Fatalf("消费失败: %v", err)
	}
	select {
	case d := <-msgs:
		if string(d.Body) != `{"order_no":"FKTEST-DELAY-001"}` {
			t.Errorf("消息体不符: %s", string(d.Body))
		}
		delay := time.Since(publishedAt)
		if delay < 900*time.Millisecond {
			t.Errorf("消息提前到达(%v),未经过 TTL 延迟", delay)
		}
		t.Logf("延迟消息 %.2f 秒后经死信队列到达", delay.Seconds())
	case <-time.After(5 * time.Second):
		t.Fatal("5 秒内未收到死信消息,延迟链路不通")
	}
}
