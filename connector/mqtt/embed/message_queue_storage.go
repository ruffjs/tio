package embed

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dgraph-io/badger/v4"
)

// QueueStorage 消息队列存储接口
type QueueStorage interface {
	// Enqueue 将消息入队
	Enqueue(ctx context.Context, queueName string, msg *queuedMessage) error
	// Dequeue 从队列中取出消息（FIFO）
	Dequeue(ctx context.Context, queueName string) (*queuedMessage, error)
	// Peek 查看队列中的下一个消息但不移除
	Peek(ctx context.Context, queueName string) (*queuedMessage, error)
	// Size 返回队列中消息数量
	Size(ctx context.Context, queueName string) (int64, error)
	// Close 关闭存储
	Close() error
}

// MemoryStorage 内存存储实现
type MemoryStorage struct {
	queues sync.Map // map[string]*memoryQueue
	mu     sync.RWMutex
}

type memoryQueue struct {
	messages []*queuedMessage
	mu       sync.RWMutex
	maxSize  int
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{}
}

func (m *MemoryStorage) Enqueue(ctx context.Context, queueName string, msg *queuedMessage) error {
	queue, _ := m.queues.LoadOrStore(queueName, &memoryQueue{
		messages: make([]*queuedMessage, 0),
		maxSize:  10000, // 默认最大 10000 条消息
	})

	q := queue.(*memoryQueue)
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.messages) >= q.maxSize {
		return ErrQueueFull
	}

	q.messages = append(q.messages, msg)
	return nil
}

func (m *MemoryStorage) Dequeue(ctx context.Context, queueName string) (*queuedMessage, error) {
	queue, ok := m.queues.Load(queueName)
	if !ok {
		return nil, ErrQueueEmpty
	}

	q := queue.(*memoryQueue)
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.messages) == 0 {
		return nil, ErrQueueEmpty
	}

	msg := q.messages[0]
	q.messages = q.messages[1:]
	return msg, nil
}

func (m *MemoryStorage) Peek(ctx context.Context, queueName string) (*queuedMessage, error) {
	queue, ok := m.queues.Load(queueName)
	if !ok {
		return nil, ErrQueueEmpty
	}

	q := queue.(*memoryQueue)
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.messages) == 0 {
		return nil, ErrQueueEmpty
	}

	return q.messages[0], nil
}

func (m *MemoryStorage) Size(ctx context.Context, queueName string) (int64, error) {
	queue, ok := m.queues.Load(queueName)
	if !ok {
		return 0, nil
	}

	q := queue.(*memoryQueue)
	q.mu.RLock()
	defer q.mu.RUnlock()

	return int64(len(q.messages)), nil
}

func (m *MemoryStorage) Close() error {
	return nil
}

// BadgerStorage BadgerDB 存储实现
type BadgerStorage struct {
	db     *badger.DB
	prefix []byte
}

func NewBadgerStorage(dbPath string) (*BadgerStorage, error) {
	// BadgerDB 需要目录路径，如果提供的是文件路径，使用其目录
	// 如果路径以 .db 结尾，使用其目录
	dir := dbPath
	if filepath.Ext(dbPath) == ".db" {
		dir = filepath.Dir(dbPath)
		// 如果目录是 "."，使用当前目录
		if dir == "." {
			dir = "./"
		}
	}

	// 确保目录存在
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory for BadgerDB: %w", err)
	}

	// BadgerDB 使用目录路径
	opts := badger.DefaultOptions(dir)
	opts.Logger = nil // 禁用 badger 的日志，使用我们的 slog

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB at %s: %w", dir, err)
	}

	return &BadgerStorage{
		db:     db,
		prefix: []byte("mq:queue:"),
	}, nil
}

// queueKey 生成队列的 key
func (b *BadgerStorage) queueKey(queueName string) []byte {
	return append(b.prefix, []byte(queueName)...)
}

// messageKey 生成消息的 key: prefix + queueName + seq
func (b *BadgerStorage) messageKey(queueName string, seq uint64) []byte {
	key := b.queueKey(queueName)
	key = append(key, ':')
	seqBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seqBytes, seq)
	return append(key, seqBytes...)
}

// 注意：getNextSeq, getFirstSeq, setFirstSeq 已不再使用，逻辑已合并到 Enqueue/Dequeue 中
// 保留这些函数用于向后兼容（如果需要），但实际不再调用

func (b *BadgerStorage) Enqueue(ctx context.Context, queueName string, msg *queuedMessage) error {
	// 序列化消息
	msgData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	seqKey := append(b.queueKey(queueName), []byte(":seq")...)
	firstKey := append(b.queueKey(queueName), []byte(":first")...)

	err = b.db.Update(func(txn *badger.Txn) error {
		// 获取并增加序列号（在同一事务中）
		var seq uint64
		seqItem, err := txn.Get(seqKey)
		if err == badger.ErrKeyNotFound {
			seq = 0
		} else if err != nil {
			return fmt.Errorf("failed to get seq key: %w", err)
		} else {
			err = seqItem.Value(func(val []byte) error {
				seq = binary.BigEndian.Uint64(val)
				return nil
			})
			if err != nil {
				return fmt.Errorf("failed to read seq value: %w", err)
			}
		}

		seq++
		seqBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(seqBytes, seq)
		if err := txn.Set(seqKey, seqBytes); err != nil {
			return fmt.Errorf("failed to set seq key: %w", err)
		}

		// 如果是第一条消息，设置 first seq（在同一事务中）
		if seq == 1 {
			firstSeqBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(firstSeqBytes, 1)
			if err := txn.Set(firstKey, firstSeqBytes); err != nil {
				return fmt.Errorf("failed to set first key: %w", err)
			}
		}

		// 存储消息
		key := b.messageKey(queueName, seq)
		if err := txn.Set(key, msgData); err != nil {
			return fmt.Errorf("failed to set message key: %w", err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("badger transaction failed for queue %s: %w", queueName, err)
	}

	return nil
}

func (b *BadgerStorage) Dequeue(ctx context.Context, queueName string) (*queuedMessage, error) {
	var msg *queuedMessage

	firstKey := append(b.queueKey(queueName), []byte(":first")...)
	seqKey := append(b.queueKey(queueName), []byte(":seq")...)

	err := b.db.Update(func(txn *badger.Txn) error {
		// 获取第一个消息序列号
		firstItem, err := txn.Get(firstKey)
		if err == badger.ErrKeyNotFound {
			return ErrQueueEmpty
		}
		if err != nil {
			return fmt.Errorf("failed to get first key: %w", err)
		}

		var firstSeq uint64
		err = firstItem.Value(func(val []byte) error {
			firstSeq = binary.BigEndian.Uint64(val)
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to read first seq: %w", err)
		}

		// 读取消息
		msgKey := b.messageKey(queueName, firstSeq)
		msgItem, err := txn.Get(msgKey)
		if err == badger.ErrKeyNotFound {
			// 消息不存在，可能已被删除，返回空队列
			return ErrQueueEmpty
		}
		if err != nil {
			return fmt.Errorf("failed to get message key: %w", err)
		}

		err = msgItem.Value(func(val []byte) error {
			msg = &queuedMessage{}
			if err := json.Unmarshal(val, msg); err != nil {
				return fmt.Errorf("failed to unmarshal message: %w", err)
			}
			// 验证消息字段
			if msg.Topic == "" && len(msg.Payload) > 0 {
				// 可能是旧格式，尝试修复
				return nil
			}
			return nil
		})
		if err != nil {
			return err
		}

		// 删除消息
		if err := txn.Delete(msgKey); err != nil {
			return fmt.Errorf("failed to delete message: %w", err)
		}

		// 获取最大序列号
		seqItem, err := txn.Get(seqKey)
		if err == badger.ErrKeyNotFound {
			// 没有 seq key，说明队列异常，删除 first key
			if err := txn.Delete(firstKey); err != nil {
				return fmt.Errorf("failed to delete first key: %w", err)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to get seq key: %w", err)
		}

		var maxSeq uint64
		err = seqItem.Value(func(val []byte) error {
			maxSeq = binary.BigEndian.Uint64(val)
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to read max seq: %w", err)
		}

		// 更新 first seq
		nextSeq := firstSeq + 1
		if nextSeq > maxSeq {
			// 队列为空，删除 first seq
			if err := txn.Delete(firstKey); err != nil {
				return fmt.Errorf("failed to delete first key when empty: %w", err)
			}
		} else {
			// 更新 first seq 为下一个
			nextSeqBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(nextSeqBytes, nextSeq)
			if err := txn.Set(firstKey, nextSeqBytes); err != nil {
				return fmt.Errorf("failed to update first key: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		if err == ErrQueueEmpty {
			return nil, err
		}
		return nil, fmt.Errorf("badger transaction failed for queue %s: %w", queueName, err)
	}

	return msg, nil
}

func (b *BadgerStorage) Peek(ctx context.Context, queueName string) (*queuedMessage, error) {
	var msg *queuedMessage

	firstKey := append(b.queueKey(queueName), []byte(":first")...)

	err := b.db.View(func(txn *badger.Txn) error {
		// 获取第一个消息序列号
		firstItem, err := txn.Get(firstKey)
		if err == badger.ErrKeyNotFound {
			return ErrQueueEmpty
		}
		if err != nil {
			return err
		}

		var firstSeq uint64
		err = firstItem.Value(func(val []byte) error {
			firstSeq = binary.BigEndian.Uint64(val)
			return nil
		})
		if err != nil {
			return err
		}

		// 读取消息
		msgKey := b.messageKey(queueName, firstSeq)
		msgItem, err := txn.Get(msgKey)
		if err == badger.ErrKeyNotFound {
			// 消息不存在，队列可能不一致，返回空队列
			return ErrQueueEmpty
		}
		if err != nil {
			return err
		}

		return msgItem.Value(func(val []byte) error {
			msg = &queuedMessage{}
			return json.Unmarshal(val, msg)
		})
	})

	return msg, err
}

func (b *BadgerStorage) Size(ctx context.Context, queueName string) (int64, error) {
	var size int64

	firstKey := append(b.queueKey(queueName), []byte(":first")...)
	seqKey := append(b.queueKey(queueName), []byte(":seq")...)

	err := b.db.View(func(txn *badger.Txn) error {
		firstItem, err := txn.Get(firstKey)
		if err == badger.ErrKeyNotFound {
			// 没有 first key，队列为空
			size = 0
			return nil
		}
		if err != nil {
			return err
		}

		seqItem, err := txn.Get(seqKey)
		if err == badger.ErrKeyNotFound {
			// 没有 seq key，队列为空
			size = 0
			return nil
		}
		if err != nil {
			return err
		}

		var firstSeq, maxSeq uint64
		err = firstItem.Value(func(val []byte) error {
			firstSeq = binary.BigEndian.Uint64(val)
			return nil
		})
		if err != nil {
			return err
		}

		err = seqItem.Value(func(val []byte) error {
			maxSeq = binary.BigEndian.Uint64(val)
			return nil
		})
		if err != nil {
			return err
		}

		// 计算队列大小：maxSeq - firstSeq + 1
		// 简化计算，直接使用差值（因为消息是连续存储的，不应该有空洞）
		if firstSeq > maxSeq {
			size = 0
		} else {
			size = int64(maxSeq - firstSeq + 1)
		}

		return nil
	})

	return size, err
}

func (b *BadgerStorage) Close() error {
	return b.db.Close()
}

var (
	ErrQueueFull  = &queueError{msg: "queue is full"}
	ErrQueueEmpty = &queueError{msg: "queue is empty"}
)

type queueError struct {
	msg string
}

func (e *queueError) Error() string {
	return e.msg
}
