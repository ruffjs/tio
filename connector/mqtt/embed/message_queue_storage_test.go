package embed

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMemoryStorage_EnqueueDequeue(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()
	queueName := "test-queue"

	msg := &queuedMessage{
		MsgID:   "1",
		Topic:   "topic1",
		Payload: []byte("hello"),
	}

	// Test Enqueue
	err := storage.Enqueue(ctx, queueName, msg)
	assert.NoError(t, err)

	// Test Size
	size := storage.Size(ctx, queueName)
	assert.Equal(t, int64(1), size)

	// Test Dequeue
	dequeued := storage.Dequeue(ctx, queueName)
	assert.NotNil(t, dequeued)
	assert.Equal(t, msg.MsgID, dequeued.MsgID)

	// Test Empty Dequeue
	assert.Nil(t, storage.Dequeue(ctx, queueName))
}

func TestMemoryStorage_QueueFull(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()
	queueName := "full-queue"

	// Set a small size
	err := storage.SetConfig(queueName, QueueConfig{MaxSize: 2})
	assert.NoError(t, err)

	storage.Enqueue(ctx, queueName, &queuedMessage{MsgID: "1"})
	storage.Enqueue(ctx, queueName, &queuedMessage{MsgID: "2"})

	// Now the queue is full. Enqueueing "3" should evict "1" (Sliding Window behavior).
	err = storage.Enqueue(ctx, queueName, &queuedMessage{MsgID: "3"})
	assert.NoError(t, err)

	size := storage.Size(ctx, queueName)
	assert.Equal(t, int64(2), size)

	// Verify that "1" was evicted and the queue contains "2" and "3" in FIFO order
	m2 := storage.Dequeue(ctx, queueName)
	assert.NotNil(t, m2)
	assert.Equal(t, "2", m2.MsgID)

	m3 := storage.Dequeue(ctx, queueName)
	assert.NotNil(t, m3)
	assert.Equal(t, "3", m3.MsgID)

	assert.Nil(t, storage.Dequeue(ctx, queueName))
}

func TestMemoryStorage_Expiration(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()
	queueName := "exp-queue"

	// Message that expires immediately
	msg1 := &queuedMessage{
		MsgID:     "1",
		ExpiredAt: time.Now().Add(-1 * time.Second),
	}
	// Message that does not expire
	msg2 := &queuedMessage{
		MsgID: "2",
	}

	storage.Enqueue(ctx, queueName, msg1)
	storage.Enqueue(ctx, queueName, msg2)

	// Dequeue should skip msg1 and return msg2
	dequeued := storage.Dequeue(ctx, queueName)
	assert.NotNil(t, dequeued)
	assert.Equal(t, "2", dequeued.MsgID)
	assert.Nil(t, storage.Dequeue(ctx, queueName))
}

func TestMemoryStorage_Config(t *testing.T) {
	storage := NewMemoryStorage()
	queueName := "config-queue"

	config := QueueConfig{
		MaxSize:    500,
		ExpireTime: 10 * time.Second,
	}

	err := storage.SetConfig(queueName, config)
	assert.NoError(t, err)

	gotConfig := storage.GetConfig(queueName)
	assert.Equal(t, config.MaxSize, gotConfig.MaxSize)
	assert.Equal(t, config.ExpireTime, gotConfig.ExpireTime)

	// Update only expire time
	newConfig := QueueConfig{
		MaxSize:    500,
		ExpireTime: 20 * time.Second,
	}
	err = storage.SetConfig(queueName, newConfig)
	assert.NoError(t, err)
	assert.Equal(t, 20*time.Second, storage.GetConfig(queueName).ExpireTime)
}

func TestMemoryStorage_Concurrency(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()
	queueName := "concurrent-queue"

	const workers = 10
	const msgsPerWorker = 100

	var wg sync.WaitGroup
	wg.Add(workers * 2)

	// Concurrent Enqueuers
	for i := 0; i < workers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < msgsPerWorker; j++ {
				storage.Enqueue(ctx, queueName, &queuedMessage{
					MsgID: fmt.Sprintf("w%d-m%d", workerID, j),
				})
			}
		}(i)
	}

	// Concurrent Dequeuers
	var count int64
	var mu sync.Mutex
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for {
				msg := storage.Dequeue(ctx, queueName)
				if msg != nil {
					mu.Lock()
					count++
					mu.Unlock()
				} else {
					// Check if we should stop
					mu.Lock()
					c := count
					mu.Unlock()
					if c >= workers*msgsPerWorker {
						return
					}
					time.Sleep(1 * time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int64(workers*msgsPerWorker), count)
}
