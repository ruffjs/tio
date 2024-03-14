package embed_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"ruff.io/tio/connector"

	"ruff.io/tio/connector/mqtt/client"
	"ruff.io/tio/connector/mqtt/embed"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/config"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/shadow"
)

func TestEmbedBrokerConnectivity(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	host := "localhost"
	port := 21883
	brk := embed.InitBroker(embed.MochiConfig{
		TcpPort: port,
		AuthzFn: func(embed.ConnectParams) bool {
			return true
		},
		AclFn: func(user string, topic string, write bool) bool {
			return true
		},
	})

	want := []string{"hi-one", "hi-two"}
	gotConn := make([]string, 0)
	gotDisc := make([]string, 0)

	monCl := client.NewClient(config.MqttClientConfig{ClientId: "mon", User: "mon", Password: "mon", Port: port, Host: host})
	err := monCl.Connect(ctx)
	require.NoError(t, err)

	err = monCl.Subscribe(ctx, connector.TopicPresence("+"), 0, func(c mqtt.Client, m mqtt.Message) {
		var evt connector.PresenceEvent
		err := json.Unmarshal(m.Payload(), &evt)
		require.NoError(t, err, "should unmarshal event")
		thingId, err := shadow.GetThingIdFromTopic(m.Topic())
		require.NoError(t, err)
		if evt.EventType == connector.EventConnected {
			gotConn = append(gotConn, thingId)
		} else if evt.EventType == connector.EventDisconnected {
			gotDisc = append(gotDisc, thingId)
		} else {
			log.Fatalf("Unknown presence event type %s", evt.EventType)
		}
	})
	require.NoError(t, err)

	for _, c := range want {
		clId := c
		go func() {
			// client connect and disconnect
			cl := client.NewClient(config.MqttClientConfig{ClientId: clId, User: clId, Port: port, Host: host})
			err = cl.Connect(ctx)
			require.NoError(t, err)
			// wait connect event handle done
			time.Sleep(time.Millisecond * 1)

			getCl, err := brk.ClientInfo(clId)
			require.NoError(t, err, "should find "+clId)
			require.Equal(t, clId, getCl.ClientId)
			require.Equal(t, true, brk.IsConnected(clId), "should connected "+clId)

			cl.Disconnect()
			// wait disconnect event handle done
			time.Sleep(time.Millisecond * 1)
			require.Equal(t, false, brk.IsConnected(clId), "should disconnected "+clId)
			getCl, err = brk.ClientInfo(clId)
			require.NoError(t, err, "should find disconnected client "+clId)
			require.Equal(t, clId, getCl.ClientId)
		}()
	}

	time.Sleep(time.Millisecond * 10)
	require.True(t, subSlice(want, gotConn), "conn %v should contains %v", gotConn, want)

	require.True(t, subSlice(want, gotDisc), "disc %v should contains %v", gotDisc, want)

	cancel()
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
func subSlice(s1 []string, s2 []string) bool {
	if len(s1) > len(s2) {
		return false
	}
	for _, e := range s1 {
		if !contains(s2, e) {
			return false
		}
	}
	return true
}
