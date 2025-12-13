package embed

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofrs/uuid"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

const queueTopicPrefix = "$q/"

// messageQueueHook 实现消息队列功能
type messageQueueHook struct {
	mqtt.HookBase
	server  *mqtt.Server
	storage QueueStorage

	// 队列注册表：queueName => queueTopic（用于匹配）
	queueRegistry sync.Map // map[string]string

	// 消费者管理：queueName => consumers
	consumers sync.Map // map[string]*queueConsumers
}

type messageQueue struct {
	topic    string
	messages chan *queuedMessage
	mu       sync.RWMutex
}

type queuedMessage struct {
	MessageID string    `json:"messageId"` // 消息唯一 ID
	Topic     string    `json:"topic"`
	Payload   []byte    `json:"payload"`
	Qos       byte      `json:"qos"`
	CreatedAt time.Time `json:"createdAt"`
}

type queueConsumers struct {
	queueName string
	consumers []*consumer
	mu        sync.RWMutex
	nextIndex int64 // 用于 round-robin（原子操作）
}

type consumer struct {
	clientId      string
	client        *mqtt.Client
	queueName     string
	ctx           context.Context
	cancel        context.CancelFunc
	inFlightCount int64    // 未确认消息数（原子操作）
	maxPrefetch   int64    // 最大预取数（默认 10）
	inFlightMsgs  sync.Map // map[packetID]messageID，跟踪未确认的消息
}

// consumerType 类型别名，用于在闭包中避免变量名遮蔽
type consumerType = consumer

type messageQueueConfig struct {
	StorageType string
	StoragePath string
}

func newMessageQueueHook(server *mqtt.Server, cfg messageQueueConfig) (*messageQueueHook, error) {
	var storage QueueStorage
	var err error

	switch cfg.StorageType {
	case "badger":
		if cfg.StoragePath == "" {
			cfg.StoragePath = "./message-queue.db"
		}
		slog.Info("Initializing message queue with BadgerDB storage", "path", cfg.StoragePath)
		storage, err = NewBadgerStorage(cfg.StoragePath)
		if err != nil {
			slog.Error("Failed to initialize BadgerDB storage", "path", cfg.StoragePath, "error", err)
			return nil, err
		}
		slog.Info("Message queue using BadgerDB storage", "path", cfg.StoragePath)
	default:
		storage = NewMemoryStorage()
		slog.Info("Message queue using memory storage")
	}

	return &messageQueueHook{
		server:  server,
		storage: storage,
	}, nil
}

func (h *messageQueueHook) ID() string {
	return "message-queue"
}

func (h *messageQueueHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnPublish,
		mqtt.OnSubscribed,
		mqtt.OnUnsubscribed,
		mqtt.OnDisconnect,
		mqtt.OnPacketRead, // 用于拦截 PUBACK
	}, []byte{b})
}

// Init 初始化 hook
func (h *messageQueueHook) Init(config any) error {
	return nil
}

// Stop 停止 hook，关闭存储
func (h *messageQueueHook) Stop() error {
	if h.storage != nil {
		return h.storage.Close()
	}
	return nil
}

// OnPublish 拦截发布消息，如果主题匹配队列则入队
func (h *messageQueueHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	// 检查是否有队列匹配该主题
	h.queueRegistry.Range(func(key, value interface{}) bool {
		queueName := key.(string)
		queueTopic := value.(string)

		// 简单的主题匹配（支持通配符）
		if matchTopic(queueTopic, pk.TopicName) {
			// 生成消息 ID
			messageID, err := uuid.NewV4()
			if err != nil {
				slog.Error("Failed to generate message ID",
					"queue", queueName,
					"topic", pk.TopicName,
					"error", err)
				return true // 继续处理其他队列
			}

			// 消息入队
			msg := &queuedMessage{
				MessageID: messageID.String(),
				Topic:     pk.TopicName,
				Payload:   pk.Payload,
				Qos:       pk.FixedHeader.Qos,
				CreatedAt: time.Now(),
			}

			ctx := context.Background()
			err = h.storage.Enqueue(ctx, queueName, msg)
			if err != nil {
				if err == ErrQueueFull {
					slog.Warn("Queue full, message dropped",
						"queue", queueName,
						"topic", pk.TopicName)
				} else {
					slog.Error("Failed to enqueue message",
						"queue", queueName,
						"topic", pk.TopicName,
						"error", err)
				}
			} else {
				slog.Info("Message enqueued",
					"queue", queueName,
					"topic", pk.TopicName,
					"packetID", pk.PacketID,
					"clientId", cl.ID)
			}
		}
		return true
	})

	return pk, nil
}

// OnSubscribe 拦截订阅，处理 $q/ 前缀的队列订阅
func (h *messageQueueHook) OnSubscribed(cl *mqtt.Client, pk packets.Packet, reasonCodes []byte) {
	for _, sub := range pk.Filters {
		if after, ok := strings.CutPrefix(sub.Filter, queueTopicPrefix); ok {
			// 提取队列名称（去掉 $q/ 前缀）
			queueName := after

			// 确保队列存在
			h.ensureQueue(queueName)

			// 添加消费者
			h.addConsumer(cl, queueName)

			slog.Info("Queue subscription",
				"clientId", cl.ID,
				"queue", queueName,
				"filter", sub.Filter,
				"reasonCodes", reasonCodes,
			)
		}
	}

}

// OnUnsubscribe 处理取消订阅
func (h *messageQueueHook) OnUnsubscribed(cl *mqtt.Client, pk packets.Packet) {
	for _, filter := range pk.Filters {
		if after, ok := strings.CutPrefix(filter.Filter, queueTopicPrefix); ok {
			queueName := after
			h.removeConsumer(cl.ID, queueName)

			slog.Info("Queue unsubscription",
				"clientId", cl.ID,
				"queue", queueName)
		}
	}
}

// OnDisconnect 客户端断开时清理消费者
func (h *messageQueueHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	// 清理该客户端的所有消费者
	h.consumers.Range(func(key, value interface{}) bool {
		consumers := value.(*queueConsumers)
		h.removeConsumer(cl.ID, consumers.queueName)
		return true
	})
}

// OnPacketRead 拦截客户端发送的数据包，用于捕获 PUBACK
func (h *messageQueueHook) OnPacketRead(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	// 检查是否是 PUBACK 包
	if pk.FixedHeader.Type == packets.Puback {
		slog.Info("OnPacketRead: PUBACK received", "clientId", cl.ID, "packetID", pk.PacketID)
		// 处理 PUBACK，减少 inFlightCount
		h.handlePuback(cl, pk.PacketID)
	}
	return pk, nil
}

// handlePuback 处理收到的 PUBACK
func (h *messageQueueHook) handlePuback(cl *mqtt.Client, packetID uint16) {
	// 遍历所有队列，找到对应的 consumer
	h.consumers.Range(func(key, value interface{}) bool {
		queueName := key.(string)
		consumers := value.(*queueConsumers)

		consumers.mu.RLock()
		for _, consumer := range consumers.consumers {
			if consumer.clientId == cl.ID {
				// 检查这个 packetID 是否在我们的跟踪列表中
				if messageID, ok := consumer.inFlightMsgs.LoadAndDelete(packetID); ok {
					// 减少未确认计数
					newCount := atomic.AddInt64(&consumer.inFlightCount, -1)

					slog.Info("Message acknowledged",
						"clientId", cl.ID,
						"queue", queueName,
						"packetID", packetID,
						"messageID", messageID,
						"inFlightCount", newCount)

					consumers.mu.RUnlock()
					return false // 找到后停止遍历
				}
			}
		}
		consumers.mu.RUnlock()
		return true
	})
}

// ensureQueue 确保队列存在（注册队列，队列名称就是主题）
func (h *messageQueueHook) ensureQueue(queueName string) {
	// 队列名称就是主题，直接注册
	if _, exists := h.queueRegistry.Load(queueName); !exists {
		h.queueRegistry.Store(queueName, queueName)
		slog.Info("Queue registered", "queue", queueName)
	}
}

// addConsumer 添加消费者
func (h *messageQueueHook) addConsumer(cl *mqtt.Client, queueName string) {
	consumers, _ := h.consumers.LoadOrStore(queueName, &queueConsumers{
		queueName: queueName,
		consumers: make([]*consumer, 0),
	})

	qc := consumers.(*queueConsumers)
	qc.mu.Lock()
	defer qc.mu.Unlock()

	// 检查是否已存在
	for _, c := range qc.consumers {
		if c.clientId == cl.ID {
			return
		}
	}

	// 创建消费者
	ctx, cancel := context.WithCancel(context.Background())
	consumer := &consumer{
		clientId:      cl.ID,
		client:        cl,
		queueName:     queueName,
		ctx:           ctx,
		cancel:        cancel,
		inFlightCount: 0,
		maxPrefetch:   10, // 默认最大预取 10 条消息
	}

	qc.consumers = append(qc.consumers, consumer)

	// 启动消费者协程
	go h.consumeMessages(consumer)

	slog.Info("Consumer added",
		"clientId", cl.ID,
		"queue", queueName,
		"totalConsumers", len(qc.consumers))
}

// removeConsumer 移除消费者
func (h *messageQueueHook) removeConsumer(clientId, queueName string) {
	consumers, ok := h.consumers.Load(queueName)
	if !ok {
		return
	}

	qc := consumers.(*queueConsumers)
	qc.mu.Lock()
	defer qc.mu.Unlock()

	for i, c := range qc.consumers {
		if c.clientId == clientId {
			// 取消消费者
			c.cancel()

			// 从列表中移除
			qc.consumers = append(qc.consumers[:i], qc.consumers[i+1:]...)

			slog.Info("Consumer removed",
				"clientId", clientId,
				"queue", queueName,
				"remainingConsumers", len(qc.consumers))

			// 如果没有消费者了，可以选择清理队列
			if len(qc.consumers) == 0 {
				h.consumers.Delete(queueName)
			}
			break
		}
	}
}

// consumeMessages 消费消息（使用 round-robin 负载均衡）
func (h *messageQueueHook) consumeMessages(consumer *consumer) {
	targetTopic := queueTopicPrefix + consumer.queueName

	slog.Info("Consumer started",
		"clientId", consumer.clientId,
		"queue", consumer.queueName)

	ticker := time.NewTicker(100 * time.Millisecond) // 轮询间隔（仅在队列为空时使用）
	defer ticker.Stop()

	// 尝试消费一条消息，返回是否成功消费
	tryConsume := func() bool {
		// 检查客户端是否还连接
		if consumer.client.Closed() {
			slog.Debug("Client disconnected, stop consuming",
				"clientId", consumer.clientId)
			return false
		}

		// 检查预取限制
		if atomic.LoadInt64(&consumer.inFlightCount) >= consumer.maxPrefetch {
			// 已达到预取限制，跳过
			return false
		}

		// 先快速检查队列是否有消息，避免无意义的 round-robin
		ctx := context.Background()

		// 使用 Peek 检查队列是否有消息（比 Size 更准确且更快）
		tmpMsg, err := h.storage.Peek(ctx, consumer.queueName)
		if err != nil {
			if err == ErrQueueEmpty {
				// 队列为空，直接返回
				return false
			}
			slog.Error("Failed to peek queue",
				"clientId", consumer.clientId,
				"queue", consumer.queueName,
				"error", err)
			return false
		}
		slog.Debug("Peek queue", "tmpMsg", tmpMsg)

		// 使用 round-robin 选择消费者
		consumers, ok := h.consumers.Load(consumer.queueName)
		if !ok {
			return false
		}

		qc := consumers.(*queueConsumers)
		qc.mu.RLock()
		if len(qc.consumers) == 0 {
			qc.mu.RUnlock()
			return false
		}

		// 过滤出未达到预取限制的消费者
		availableConsumers := make([]*consumerType, 0)
		for _, c := range qc.consumers {
			if atomic.LoadInt64(&c.inFlightCount) < c.maxPrefetch {
				availableConsumers = append(availableConsumers, c)
			}
		}

		if len(availableConsumers) == 0 {
			// 所有消费者都达到预取限制
			qc.mu.RUnlock()
			return false
		}

		// Round-robin: 从可用消费者中选择下一个
		// 原子性地增加索引并获取当前应该处理的消费者
		currentIdx := atomic.AddInt64(&qc.nextIndex, 1) - 1
		idx := int(currentIdx) % len(availableConsumers)
		selectedConsumer := availableConsumers[idx]
		qc.mu.RUnlock()

		// 只处理分配给当前消费者的消息
		if selectedConsumer.clientId != consumer.clientId {
			// 不是自己的轮次，静默返回（不打印日志，避免日志过多）
			return false
		}

		// 从存储中取出消息（只有轮到的消费者才会执行到这里）
		// 注意：不先 Peek，直接 Dequeue，因为 Dequeue 是原子的
		// 如果队列为空，Dequeue 会返回 ErrQueueEmpty
		msg, err := h.storage.Dequeue(ctx, consumer.queueName)
		if err != nil {
			if err == ErrQueueEmpty {
				// 队列已空，可能是其他消费者已经取走了（竞态条件）
				// 这种情况是正常的，返回 false 让下一个消费者尝试
				return false
			}
			slog.Error("Failed to dequeue message",
				"clientId", consumer.clientId,
				"queue", consumer.queueName,
				"error", err)
			return false
		}

		// 验证消息是否有效
		if msg == nil {
			slog.Error("Dequeued message is nil",
				"clientId", consumer.clientId,
				"queue", consumer.queueName)
			return false
		}

		// 成功取出消息，投递
		pkId, err := consumer.client.NextPacketID()
		if err != nil {
			slog.Error("Failed to get next packet ID",
				"clientId", consumer.clientId,
				"queue", consumer.queueName,
				"error", err)
			// 获取 packet ID 失败，重新入队到队列末尾
			if enqErr := h.storage.Enqueue(ctx, consumer.queueName, msg); enqErr != nil {
				slog.Error("Failed to re-enqueue message after packet ID error",
					"clientId", consumer.clientId,
					"queue", consumer.queueName,
					"error", enqErr)
			}
			return false
		}

		// 投递消息（QoS 1）
		pk := packets.Packet{
			FixedHeader: packets.FixedHeader{
				Type:   packets.Publish,
				Qos:    1, // 队列消息统一使用 QoS 1
				Retain: false,
			},
			PacketID:  uint16(pkId),
			TopicName: targetTopic,
			Payload:   msg.Payload,
		}

		// 记录 packetID -> messageID 映射（在收到 PUBACK 时使用）
		consumer.inFlightMsgs.Store(pk.PacketID, msg.MessageID)

		err = consumer.client.WritePacket(pk)
		if err != nil {
			slog.Error("Failed to deliver queued message",
				"clientId", consumer.clientId,
				"queue", consumer.queueName,
				"error", err)
			// 投递失败，删除映射并重新入队到队列末尾
			consumer.inFlightMsgs.Delete(pk.PacketID)
			if enqErr := h.storage.Enqueue(ctx, consumer.queueName, msg); enqErr != nil {
				slog.Error("Failed to re-enqueue message after delivery error",
					"clientId", consumer.clientId,
					"queue", consumer.queueName,
					"error", enqErr)
			}
			return false
		}

		// 手动增加 inFlightCount（因为 WritePacket 不会触发 OnQosPublish hook）
		newCount := atomic.AddInt64(&consumer.inFlightCount, 1)
		slog.Debug("Message in-flight",
			"clientId", consumer.clientId,
			"queue", consumer.queueName,
			"packetID", pk.PacketID,
			"messageID", msg.MessageID,
			"inFlightCount", newCount)

		slog.Info("Queued message delivered",
			"clientId", consumer.clientId,
			"queue", consumer.queueName,
			"topic", msg.Topic,
			"messageID", msg.MessageID,
			"packetID", pk.PacketID)
		return true
	}

	// 主循环：快速消费模式
	for {
		select {
		case <-consumer.ctx.Done():
			slog.Info("Consumer stopped",
				"clientId", consumer.clientId,
				"queue", consumer.queueName)
			return

		case <-ticker.C:
			// ticker 触发时尝试消费（用于队列为空后的轮询）
			tryConsume()

		default:
			// 快速消费模式：连续尝试消费，直到队列为空或不是自己的轮次
			consumed := tryConsume()
			if !consumed {
				// 消费失败（可能是队列为空，或者不是自己的轮次）
				// 短暂休眠避免 CPU 空转，然后继续尝试
				// 使用更短的 ticker 或者直接 sleep
				time.Sleep(10 * time.Millisecond)
			}
			// 如果成功消费，继续循环立即尝试下一条（不等待 ticker）
		}
	}
}

// matchTopic 简单的主题匹配（支持 + 和 # 通配符）
func matchTopic(filter, topic string) bool {
	filterParts := strings.Split(filter, "/")
	topicParts := strings.Split(topic, "/")

	filterIdx := 0
	topicIdx := 0

	for filterIdx < len(filterParts) && topicIdx < len(topicParts) {
		if filterParts[filterIdx] == "#" {
			return true // # 匹配所有后续层级
		}
		if filterParts[filterIdx] == "+" || filterParts[filterIdx] == topicParts[topicIdx] {
			filterIdx++
			topicIdx++
			continue
		}
		return false
	}

	return filterIdx == len(filterParts) && topicIdx == len(topicParts)
}
