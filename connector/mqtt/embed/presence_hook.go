package embed

import (
	"bytes"
	"errors"
	"net"
	"strings"
	"time"

	"ruff.io/tio/connector"

	mqtt "github.com/wind-c/comqtt/v2/mqtt"
	"github.com/wind-c/comqtt/v2/mqtt/packets"
)

// presenceHook drives local presence state from session lifecycle events.
type presenceHook struct {
	mqtt.HookBase
	b *Broker
}

func (h *presenceHook) ID() string {
	return "presence"
}

func (h *presenceHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnSessionEstablished,
		mqtt.OnDisconnect,
	}, []byte{b})
}

func (h *presenceHook) OnSessionEstablished(cl *mqtt.Client, pk packets.Packet) {
	if tcpConn, ok := cl.Net.Conn.(*net.TCPConn); ok {
		// MQTT has its own heartbeat; widen TCP keepalive to avoid chatter.
		tcpConn.SetKeepAliveConfig(net.KeepAliveConfig{Enable: true, Idle: 2 * time.Hour, Interval: 75 * time.Second, Count: 3})
	}

	username := string(cl.Properties.Username)
	// A concurrent connect+disconnect of the same id can leave the session map
	// holding an already-closed client; skip the stale established event.
	if existing, ok := h.b.svr.Clients.Get(cl.ID); !ok || existing.Closed() {
		return
	}

	now := time.Now()
	h.b.publishPresence(connector.ClientInfo{
		ClientId:    cl.ID,
		Username:    username,
		Connected:   true,
		ConnectedAt: &now,
		RemoteAddr:  cl.Net.Remote,
	})
}

func (h *presenceHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	username := string(cl.Properties.Username)
	// A session taken over by a newer connection with the same client id must
	// not emit a spurious disconnect: comqtt stops the old client before adding
	// the new one, so the live-client guard below cannot see the successor yet.
	// Stop records only the first cause, so StopCause() retains
	// ErrSessionTakenOver even though the read loop later stops with a net error.
	if errors.Is(cl.StopCause(), packets.ErrSessionTakenOver) {
		return
	}
	// Ignore a stale disconnect when a newer connection with the same id is live.
	if existing, ok := h.b.svr.Clients.Get(cl.ID); ok && !existing.Closed() {
		return
	}

	now := time.Now()
	discReason := ""
	if err != nil {
		discReason = err.Error()
		if len(discReason) > 256 {
			discReason = discReason[:256]
		}
	}
	h.b.publishPresence(connector.ClientInfo{
		ClientId:         cl.ID,
		Username:         username,
		Connected:        false,
		DisconnectedAt:   &now,
		DisconnectReason: discReason,
		RemoteAddr:       cl.Net.Remote,
	})
}

// isPublishPresent reports whether a principal should emit presence events.
// Superusers (usernames starting with "$") are skipped, as in main.
func isPublishPresent(username string) bool {
	return !strings.HasPrefix(username, "$")
}
