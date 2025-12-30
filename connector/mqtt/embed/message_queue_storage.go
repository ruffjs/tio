package embed

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const DEFAULT_QUEUE_SIZE = 10000

type QueueConfig struct {
	MaxSize    int           // queue max size
	ExpireTime time.Duration // message expire time
}

type QueueStorage interface {
	Enqueue(ctx context.Context, queueName string, msg *queuedMessage) error
	Dequeue(ctx context.Context, queueName string) *queuedMessage
	Size(ctx context.Context, queueName string) int64
	SetConfig(queueName string, config QueueConfig) error
	GetConfig(queueName string) QueueConfig
	Close() error
}

type queueInfo struct {
	ch     chan *queuedMessage
	config QueueConfig
	mu     sync.RWMutex
}

type MemoryStorage struct {
	queues sync.Map // map[string]*queueInfo
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{}
}

func (m *MemoryStorage) getQueue(name string) *queueInfo {
	qi, _ := m.queues.LoadOrStore(name, &queueInfo{
		ch:     make(chan *queuedMessage, DEFAULT_QUEUE_SIZE),
		config: QueueConfig{MaxSize: DEFAULT_QUEUE_SIZE},
	})
	return qi.(*queueInfo)
}

func (m *MemoryStorage) Enqueue(ctx context.Context, queueName string, msg *queuedMessage) error {
	q := m.getQueue(queueName)
	ch := q.ch

	q.mu.RLock()
	expireTime := q.config.ExpireTime
	q.mu.RUnlock()
	if msg.ExpiredAt.IsZero() && expireTime > 0 {
		msg.ExpiredAt = time.Now().Add(expireTime)
	}

	for {
		select {
		case ch <- msg:
			return nil
		default:
			// queue is full, try to pop an old message
			select {
			case <-ch:
				// pop success, try to enqueue new message in loop
				slog.Info("Queue drop msg caused by full", "queue", queueName, "msgId", msg.MsgID)
			default:
				// if pop failed, someone took the message at this moment
				// try again in loop
			}
		}
	}
}

func (m *MemoryStorage) Dequeue(ctx context.Context, queueName string) *queuedMessage {
	qi, ok := m.queues.Load(queueName)
	if !ok {
		return nil
	}
	q := qi.(*queueInfo)

	for {
		ch := q.ch
		select {
		case msg := <-ch:
			// check if message is expired
			if !msg.ExpiredAt.IsZero() && time.Now().After(msg.ExpiredAt) {
				continue // message expired, try next
			}
			return msg
		default:
			return nil
		}
	}
}

func (m *MemoryStorage) Size(ctx context.Context, queueName string) int64 {
	if qi, ok := m.queues.Load(queueName); ok {
		q := qi.(*queueInfo)
		return int64(len(q.ch))
	}
	return 0
}

func (m *MemoryStorage) SetConfig(queueName string, config QueueConfig) error {
	if config.MaxSize <= 0 {
		config.MaxSize = DEFAULT_QUEUE_SIZE
	}

	qi, loaded := m.queues.LoadOrStore(queueName, &queueInfo{
		ch:     make(chan *queuedMessage, config.MaxSize),
		config: config,
	})

	if loaded {
		q := qi.(*queueInfo)
		q.mu.Lock()
		defer q.mu.Unlock()
		slog.Info("Queue storage already exists", "queueName", queueName)
		if q.config.MaxSize != config.MaxSize {
			slog.Warn("Queue storage: Changing MaxSize of an existing queue is not supported to avoid message loss",
				"queueName", queueName, "current", q.config.MaxSize, "requested", config.MaxSize)
		}
		q.config.ExpireTime = config.ExpireTime
	} else {
		slog.Info("Queue storage set config success", "queueName", queueName, "config", config)
	}

	return nil
}

func (m *MemoryStorage) GetConfig(queueName string) QueueConfig {
	q := m.getQueue(queueName)
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.config
}

func (m *MemoryStorage) Close() error {
	return nil
}

var (
	ErrQueueFull = &queueError{msg: "queue is full"}
)

type queueError struct {
	msg string
}

func (e *queueError) Error() string {
	return e.msg
}
