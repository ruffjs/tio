package embed

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// MessageQueueHook implements Pull mode for message queue functionality
type MessageQueueHook struct {
	mqtt.HookBase
	server  *mqtt.Server
	storage QueueStorage

	// queueRegistry: queueName => queueTopic, used for matching in OnPublish
	queueRegistry sync.Map // map[string]string
}

type queuedMessage struct {
	MsgID     string    `json:"msgId"`
	Topic     string    `json:"topic"`
	Payload   []byte    `json:"payload"`
	Qos       byte      `json:"qos"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiredAt time.Time `json:"expiredAt,omitempty"`
}

type wsRequest struct {
	Action string `json:"action"` // "pull"
	Count  int    `json:"count"`  // specify the number of messages to pull
}

type messageQueueConfig struct {
	StorageType string
	StoragePath string
}

func newMessageQueueHook(server *mqtt.Server, cfg messageQueueConfig) (*MessageQueueHook, error) {
	var storage QueueStorage

	switch cfg.StorageType {
	default:
		storage = NewMemoryStorage()
		slog.Info("Message queue using memory storage")
	}

	return &MessageQueueHook{
		server:  server,
		storage: storage,
	}, nil
}

func (h *MessageQueueHook) ID() string {
	return "message-queue"
}

func (h *MessageQueueHook) Provides(b byte) bool {
	return b == mqtt.OnPublish
}

func (h *MessageQueueHook) Init(config any) error {
	return nil
}

func (h *MessageQueueHook) Stop() error {
	if h.storage != nil {
		return h.storage.Close()
	}
	return nil
}

// OnPublish intercepts published messages and enqueues them if the topic matches a queue
func (h *MessageQueueHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	h.queueRegistry.Range(func(key, value any) bool {
		queueName := key.(string)
		queueTopic := value.(string)

		if matchTopic(queueTopic, pk.TopicName) {
			slog.Info("match topic", "queue", queueName, "topic", pk.TopicName)
			messageID, _ := uuid.NewV4()

			// Must copy Payload to avoid data corruption as the broker might reuse the buffer
			payloadCopy := make([]byte, len(pk.Payload))
			copy(payloadCopy, pk.Payload)

			msg := &queuedMessage{
				MsgID:     messageID.String(),
				Topic:     pk.TopicName,
				Payload:   payloadCopy,
				Qos:       pk.FixedHeader.Qos,
				CreatedAt: time.Now(),
			}

			if err := h.storage.Enqueue(context.Background(), queueName, msg); err != nil {
				slog.Error("Failed to enqueue message", "queue", queueName, "error", err)
			} else {
				slog.Debug("Enqueue message success", "queue", queueName, "msgId", msg.MsgID)
			}
		} else {
			slog.Info("no match topic", "queue", queueName, "queueTopic", queueTopic, "topic", pk.TopicName)
		}
		return true
	})
	return pk, nil
}

// --- WebSocket Support ---

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WsHandler handles clients' Pull requests
func (h *MessageQueueHook) WsHandler(w http.ResponseWriter, r *http.Request) {
	queueName := r.URL.Query().Get("queue")
	if queueName == "" {
		http.Error(w, "queue parameter is required", http.StatusBadRequest)
		return
	}

	clientId := r.URL.Query().Get("clientId")
	if clientId == "" {
		clientId = "ws-" + uuid.Must(uuid.NewV4()).String()
	}

	// register queue topic (simple approach: queue name is the topic)
	h.queueRegistry.Store(queueName, queueName)

	// get queue configuration parameters
	queueSizeStr := r.URL.Query().Get("queueSize")
	expireTimeStr := r.URL.Query().Get("expireTime")

	// parse queue size
	maxSize := 10000 // default size
	if queueSizeStr != "" {
		if size, err := strconv.Atoi(queueSizeStr); err == nil && size > 0 {
			maxSize = size
		}
	}

	// parse expire time (seconds)
	expireTime := time.Duration(0) // default no expiration
	if expireTimeStr != "" {
		if seconds, err := strconv.ParseInt(expireTimeStr, 10, 64); err == nil && seconds > 0 {
			expireTime = time.Duration(seconds) * time.Second
		}
	}

	// set queue configuration
	config := QueueConfig{
		MaxSize:    maxSize,
		ExpireTime: expireTime,
	}
	_ = h.storage.SetConfig(queueName, config)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	slog.Info("WS Consumer connected (Pull mode)", "clientId", clientId, "queue", queueName)

	ctx := r.Context()
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var req wsRequest
		if err := json.Unmarshal(message, &req); err != nil {
			continue
		}

		if req.Action == "pull" {
			pullCount := req.Count
			if pullCount <= 0 {
				pullCount = 1
			}
			if pullCount > 500 {
				pullCount = 500 // safety limit
			}

			delivered := 0
			for i := 0; i < pullCount; i++ {
				msg := h.storage.Dequeue(ctx, queueName)
				if msg == nil {
					break
				}

				_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				if err := conn.WriteJSON(msg); err != nil {
					slog.Warn("WS delivery failed, re-enqueueing", "clientId", clientId, "error", err)
					_ = h.storage.Enqueue(ctx, queueName, msg)
					return // exit and close connection on write error
				}
				slog.Info("WS delivery success", "clientId", clientId, "queue", queueName, "msgId", msg.MsgID)
				delivered++
			}
			if delivered > 0 {
				slog.Debug("Messages pulled", "clientId", clientId, "queue", queueName, "count", delivered)
			}
		}
	}
}

func matchTopic(filter, topic string) bool {
	f := strings.Split(filter, "/")
	t := strings.Split(topic, "/")
	i := 0
	for i < len(f) && i < len(t) {
		if f[i] == "#" {
			return true
		}
		if f[i] != "+" && f[i] != t[i] {
			return false
		}
		i++
	}
	return i == len(f) && i == len(t)
}
