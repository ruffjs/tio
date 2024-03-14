package emqx_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"ruff.io/tio/connector"

	"ruff.io/tio/connector/mqtt/client"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/config"
	mq "ruff.io/tio/connector/mqtt"
	"ruff.io/tio/connector/mqtt/emqx"
	mockmq "ruff.io/tio/connector/mqtt/mock"
	"ruff.io/tio/pkg/log"
)

const (
	thingIdDisconnected      = "thingIdDisconnected"
	thingIdConnected         = "thingIdConnected"
	thingIdConnectedBySub    = "thingIdConnectedBySub"
	thingIdDisconnectedBySub = "thingIdDisconnectedBySub"
)

func TestEmqxAdapter_IsConnected(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	mqCl := mockMqClient("tio", nil, nil)
	a, hSvr := setup(mqCl)
	// mock subscribe
	mqCl.On("Subscribe", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mqCl.On("Publish", mock.Anything, mq.DefaultQos, mock.Anything, mock.Anything).Return(mockmq.NewMockToken())
	err := a.Start(ctx)
	require.NoError(t, err)
	defer hSvr.Close()
	defer cancel()

	t.Run("Receive mqtt message for client presence", func(t *testing.T) {
		thingId := thingIdConnectedBySub
		topic := emqxTopicConn(thingId)
		mqCl.Publish(topic, mq.DefaultQos, false, genMqttConnectedMsg(thingId))
		time.Sleep(time.Millisecond * 10)

		r, err := a.IsConnected(thingId)
		require.NoError(t, err)
		require.True(t, r)
	})

	t.Run("Call http api for client presence", func(t *testing.T) {
		r, err := a.IsConnected(thingIdDisconnected)
		require.NoError(t, err)
		require.True(t, !r, "%s should not be connected", thingIdDisconnected)

		r, err = a.IsConnected(thingIdConnected)
		require.NoError(t, err)
		require.True(t, r, "%s should be connected", thingIdDisconnected)
	})
}

func emqxTopicConn(id string) string { return "$SYS/brokers/anyNode/clients/" + id + "/connected" }
func emqxTopicDisc(id string) string { return "$SYS/brokers/anyNode/clients/" + id + "/disconnected" }

func TestEmqxAdapter_RepublishPresence(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())

	cases := []struct {
		thingId string
		typ     string
	}{
		{"ccc1-for-conn", connector.EventConnected},
		{"ccc2-for-conn", connector.EventDisconnected},
		{"ccc3-for-conn", connector.EventConnected},
	}

	latestPub := []struct {
		topic string
		event connector.PresenceEvent
	}{}
	// mock mqtt client
	pubCallback := func(topic string, qos byte, retained bool, payload interface{}) {
		if !strings.Contains(topic, "-for-conn/presence") {
			// Only connection messages for this test are collected
			return
		}
		log.Debugf("====PUB==== topic=%q payload=%q", topic, payload)
		e := struct {
			topic string
			event connector.PresenceEvent
		}{
			topic: topic,
		}
		err := json.Unmarshal(payload.([]byte), &e.event)
		require.NoError(t, err)
		latestPub = append(latestPub, e)
	}
	subCallback := func(ctx context.Context, topic string, qos byte, callback mqtt.MessageHandler) {
		log.Debugf("====SUB==== topic=%q", topic)
	}
	mockMqtt := mockMqClient("test", pubCallback, subCallback)

	a, hSvr := setup(mockMqtt)
	defer hSvr.Close()
	defer cancel()
	mockMqtt.On("Subscribe", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	err := a.Start(ctx)
	require.NoError(t, err)

	token := &mockmq.Token{DoneCh: make(chan struct{})}
	close(token.DoneCh)
	pubCall := mockMqtt.On("Publish", mock.Anything, mq.DefaultQos, mock.Anything, mock.Anything).Return(token)

	for _, c := range cases {
		log.Debugf("====== thing %v ", c.thingId)
		var pubData []byte
		if c.typ == connector.EventConnected {
			pubData = genMqttConnectedMsg(c.thingId)
			mockMqtt.Publish(emqxTopicConn(c.thingId), mq.DefaultQos, false, pubData)
		} else {
			pubData = genMqttDisconnectedMsg(c.thingId)
			mockMqtt.Publish(emqxTopicDisc(c.thingId), mq.DefaultQos, false, pubData)
		}
		time.Sleep(time.Millisecond * 5)

		slog.Info("====", "vv", latestPub)
		require.Equal(t, 2, len(latestPub), "event length should be equal to 2")

		topicPre := connector.TopicPresence(c.thingId)
		topicPreEvt := connector.TopicPresenceEvent(c.thingId)
		for _, l := range latestPub {
			slog.Info("topic presence", "evt", l)
			require.True(t, topicPre == l.topic || topicPreEvt == l.topic, "topic")
			d := time.Now().UnixMilli() - l.event.Timestamp
			require.True(t, d > 0 && d < 20, "presence time")
		}
		latestPub = latestPub[:0]
	}

	pubCall.Unset()
}

func mockMqClient(clientId string, pc mockmq.PubCallback, sc mockmq.SubCallback) *mockmq.MockedMqttClient {
	token := &mockmq.Token{DoneCh: make(chan struct{})}
	close(token.DoneCh)
	mqCl := mockmq.NewMqttClient(clientId, pc, sc)
	mqCl.On("Connect").Return(token)
	return mqCl
}

func setup(mqCl client.Client) (connector.Connectivity, *httptest.Server) {
	hSvr := mockEmqxApiSvr()
	a := emqx.NewEmqxAdapter(config.EmqxAdapterConfig{
		ApiPrefix:   hSvr.URL, // "http://localhost:18083",
		ApiUser:     "admin",
		ApiPassword: "123",
	}, mqCl)

	return a, hSvr
}

func genHttpConnectedMsg(clientId string) []byte {
	tmpl := `
	{
		"username": "{c}",
		"connected": true,
		"connected_at": "2022-09-21T04:31:34.454+00:00",
		"clientid": "{c}"
	}
	`
	s := strings.ReplaceAll(tmpl, "{c}", clientId)
	return []byte(s)
}

func genHttpConnectedClientsMsg(clientId string) []byte {
	tmpl := `
	{
		"data": [{
			"username": "{c}",
			"connected": true,
			"connected_at": "2022-09-21T04:31:34.454+00:00",
			"clientid": "{c}"
		}],
		"meta":{
			"count": 1,
			"page": 1,
			"limit": 1000
		}
	}
	`
	s := strings.ReplaceAll(tmpl, "{c}", clientId)
	return []byte(s)
}

func genMqttConnectedMsg(clientId string) []byte {
	tmpl := `{
      "username": "{c}",
      "ts": {t},
      "sockport": 1883,
      "proto_ver": 4,
      "proto_name": "MQTT",
      "keepalive": 60,
      "ipaddress": "127.0.0.1",
      "expiry_interval": 0,
      "connected_at": {t},
      "connack": 0,
      "clientid": "{c}",
      "clean_start": true
	}`
	s := strings.ReplaceAll(tmpl, "{c}", clientId)
	s = strings.ReplaceAll(s, "{t}", strconv.FormatInt(time.Now().UnixMilli(), 10))
	return []byte(s)
}
func genMqttDisconnectedMsg(clientId string) []byte {
	tmpl := `{
    	"username": "{c}",
    	"ts": {t},
    	"sockport": 1883,
    	"reason": "tcp_closed",
    	"proto_ver": 4,
    	"proto_name": "MQTT",
    	"ipaddress": "127.0.0.1",
    	"disconnected_at": {t},
    	"clientid": "{c}"
	}`
	s := strings.ReplaceAll(tmpl, "{c}", clientId)
	s = strings.ReplaceAll(s, "{t}", strconv.FormatInt(time.Now().UnixMilli(), 10))
	return []byte(s)
}

func mockEmqxApiSvr() *httptest.Server {
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.RequestURI, thingIdDisconnected) {
			w.WriteHeader(404)
			_, _ = w.Write([]byte("Not found"))
		} else if strings.Contains(r.RequestURI, thingIdConnected) {
			d := genHttpConnectedMsg(thingIdConnected)
			_, _ = w.Write(d)
		} else if strings.Contains(r.RequestURI, "/api/v5/clients?") {
			log.Debugf("fetch emqx clients %s", r.RequestURI)
			d := genHttpConnectedClientsMsg(thingIdConnected)
			_, _ = w.Write(d)
		} else {
			log.Fatalf("Should never reach here: method=%s path=%s", r.Method, r.RequestURI)
		}
	})
	return ts
}
