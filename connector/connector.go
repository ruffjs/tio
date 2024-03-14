package connector

import (
	"context"
	"strings"
	"time"
)

// presence topic
const (
	// for event
	TopicEventPresenceTmpl = "$iothub/events/things/{thingId}/presence"
	TopicEventPresenceAll  = "$iothub/events/things/+/presence"

	// for retain message
	TopicPresenceTmpl = "$iothub/things/{thingId}/presence"
	TopicPresenceAll  = "$iothub/things/+/presence"
)

// event type
const (
	EventConnected    = "connected"
	EventDisconnected = "disconnected"
)

type ClientInfo struct {
	ClientId         string     `json:"clientId"`
	Username         string     `json:"username"`
	Connected        bool       `json:"connected"`
	ConnectedAt      *time.Time `json:"connectedAt"`
	DisconnectedAt   *time.Time `json:"disconnectedAt"`
	DisconnectReason string     `json:"disconnectReason"`
	RemoteAddr       string     `json:"remoteAddr"`
}

type Connector interface {
	Connectivity
	PubSub
}

type PublishData struct {
	QoS      uint // optional
	Retained bool // optional, work for MQTT
	Payload  []byte
}

type Message interface {
	Qos() byte
	Retained() bool
	Topic() string
	MessageID() uint16
	Payload() []byte
	Ack()
}

type PubSub interface {
	Subscriber
	Publisher
}

type Publisher interface {
	Publish(topic string, qos byte, retained bool, payload []byte) error
}

type Subscriber interface {
	Subscribe(ctx context.Context, topic string, qos byte, callback func(msg Message)) error
}

type Connectivity interface {
	ConnectChecker

	Start(ctx context.Context) error
	Close(thingId string) error
	Remove(thingId string) error
}

type ConnectChecker interface {
	IsConnected(thingId string) (bool, error)
	OnConnect() <-chan PresenceEvent
	ClientInfo(thingId string) (ClientInfo, error)
	AllClientInfo() ([]ClientInfo, error)
}

type PresenceEvent struct {
	Timestamp        int64  `json:"timestamp"`
	EventType        string `json:"eventType"`
	ThingId          string `json:"thingId"`
	ClientId         string `json:"clientId"`
	RemoteAddr       string `json:"remoteAddr"`
	DisconnectReason string `json:"disconnectReason,omitempty"`
}

func TopicPresence(thingId string) string {
	return strings.ReplaceAll(TopicPresenceTmpl, "{thingId}", thingId)
}

func TopicPresenceEvent(thingId string) string {
	return strings.ReplaceAll(TopicEventPresenceTmpl, "{thingId}", thingId)
}
