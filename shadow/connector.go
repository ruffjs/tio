package shadow

import (
	"context"
	"strings"
	"time"

	"ruff.io/tio/ntp"
)

// presence topic
const (
	// TopicPresenceTmpl = "$iothub/events/presence/{clientId}"
	// TopicPresenceAll  = "$iothub/events/presence/+"
	TopicPresenceTmpl = "$iothub/things/{thingId}/presence"
	TopicPresenceAll  = "$iothub/things/+/presence"
)

// event type
const (
	EventConnected    = "connected"
	EventDisconnected = "disconnected"
)

type ClientInfo struct {
	ClientId       string     `json:"clientId"`
	Username       string     `json:"username"`
	Connected      bool       `json:"connected"`
	ConnectedAt    *time.Time `json:"connectedAt"`
	DisconnectedAt *time.Time `json:"disconnectedAt"`
	RemoteAddr     string     `json:"remoteAddr"`
}

type Connector interface {
	Connectivity
	StateHandler
	MethodHandler
	ntp.Handler
}

type Connectivity interface {
	ConnectChecker

	Start(ctx context.Context) error
	Close(thingId string) error
	Remove(thingId string) error
}

type ConnectChecker interface {
	IsConnected(thingId string) (bool, error)
	OnConnect() <-chan Event
	ClientInfo(thingId string) (ClientInfo, error)
}

type Event struct {
	Timestamp        int64  `json:"timestamp"`
	EventType        string `json:"eventType"`
	ThingId          string `json:"thingId"`
	RemoteAddr       string `json:"remoteAddr"`
	DisconnectReason string `json:"disconnectReason,omitempty"`
}

func TopicPresence(thingId string) string {
	return strings.ReplaceAll(TopicPresenceTmpl, "{thingId}", thingId)
}

func TopicAllPresence() string {
	return strings.ReplaceAll(TopicPresenceTmpl, "{thingId}", "+")
}
