//go:build integration

package integration_tests

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/codec"
	"ruff.io/tio/pkg/protocol"
	"ruff.io/tio/shadow"
	"ruff.io/tio/thing"
)

func TestDualProtocolCreateAndReconnect(t *testing.T) {
	if cfg.Protocol.Mode != "dual" {
		t.Skip("requires Dual mode")
	}
	for _, mode := range []string{"legacy", "simple"} {
		for _, encoding := range []string{"json", "cbor"} {
			t.Run(mode+"_"+encoding, func(t *testing.T) {
				id := ID()
				th, err := thingSvc.Create(t.Context(), thing.Thing{
					Id: id, Enabled: true, AuthType: "password", AuthValue: "test-password",
					MessageProtocol: mode, PayloadEncoding: encoding,
				}, nil, false)
				require.NoError(t, err)
				client := newThingMqttClient(t.Context(), id, th.AuthValue)
				require.NoError(t, client.Connect(t.Context()))
				defer client.Disconnect()
				waitConnected(t, id)
				before, err := shadowSvc.Get(t.Context(), id)
				require.NoError(t, err)
				c, err := codec.New(encoding)
				require.NoError(t, err)
				// Reconnect and start business messages using the persisted profile.
				client.Disconnect()
				client = newThingMqttClient(t.Context(), id, th.AuthValue)
				require.NoError(t, client.Connect(t.Context()))
				defer client.Disconnect()
				waitConnected(t, id)
				up, down := shadow.TopicUpdateOf(id), shadow.TopicUpdateAcceptedOf(id)
				update := any(shadow.StateReq{State: shadow.StateDR{Reported: shadow.StateValue{"enabled": true}}, ClientToken: "dual-update"})
				if mode == "simple" {
					up = protocol.TopicUp(id, protocol.TypeShadowUpdate)
					down = protocol.TopicDown(id, protocol.TypeShadowUpdateReply)
					update = map[string]any{"state": map[string]any{"enabled": true}}
				}
				results := make(chan []byte, 8)
				require.NoError(t, client.Subscribe(down, 1, func(_ mqtt.Client, m mqtt.Message) { results <- append([]byte(nil), m.Payload()...) }))
				payload, err := c.Marshal(update)
				require.NoError(t, err)
				require.NoError(t, client.Publish(up, 1, false, payload))
				select {
				case payload := <-results:
					var reply map[string]any
					require.NoError(t, c.Unmarshal(payload, &reply))
					require.Greater(t, toInt64(reply["version"]), before.Version)
				case <-time.After(5 * time.Second):
					t.Fatal("shadow update timeout")
				}
				current, err := shadowSvc.Get(t.Context(), id)
				require.NoError(t, err)
				require.Equal(t, true, current.State.Reported["enabled"])
				methodTopic := shadow.TopicMethodRequest(id, "echo")
				if mode == "simple" {
					methodTopic = protocol.TopicDown(id, protocol.TypeMethodReq)
				}
				require.NoError(t, client.Subscribe(methodTopic, 1, func(_ mqtt.Client, m mqtt.Message) {
					var request map[string]any
					if c.Unmarshal(m.Payload(), &request) != nil {
						return
					}
					response := map[string]any{"code": 200, "data": "pong"}
					topic := shadow.TopicMethodResponse(id, "echo")
					if mode == "simple" {
						response["id"] = request["id"]
						topic = protocol.TopicUp(id, protocol.TypeMethodResp)
					} else {
						response["clientToken"] = request["clientToken"]
					}
					payload, err := c.Marshal(response)
					if err == nil {
						_ = client.Publish(topic, 1, false, payload)
					}
				}))
				for _, endpoint := range []string{"methods/echo", "invoke"} {
					body := `{"data":{},"respTimeout":3}`
					if endpoint == "invoke" {
						body = `{"method":"echo","params":{},"timeout":3}`
					}
					resp, err := http.Post(httpSvr.URL+"/api/v1/things/"+id+"/"+endpoint, "application/json", strings.NewReader(body))
					require.NoError(t, err)
					raw, err := io.ReadAll(resp.Body)
					resp.Body.Close()
					require.NoError(t, err)
					require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
					var result struct {
						Data struct {
							Code int    `json:"code"`
							Data string `json:"data"`
						} `json:"data"`
					}
					require.NoError(t, json.Unmarshal(raw, &result))
					require.Equal(t, 200, result.Data.Code, string(raw))
					require.Equal(t, "pong", result.Data.Data)
				}
			})
		}
	}
}

func TestDualDefaultProfile(t *testing.T) {
	if cfg.Protocol.Mode != "dual" {
		t.Skip("requires Dual mode")
	}
	id := ID()
	th := crateThing(id)
	profiles := thing.DeviceProfiles{Service: thingSvc, DefaultCodec: cfg.Protocol.Encoding}
	p, err := profiles.GetDeviceProfile(t.Context(), id)
	require.NoError(t, err)
	require.Equal(t, "legacy", p.MessageProtocol)
	require.Equal(t, cfg.Protocol.Encoding, p.PayloadEncoding)
	client := newThingMqttClient(t.Context(), id, th.AuthValue)
	require.NoError(t, client.Connect(t.Context()))
	defer client.Disconnect()
	received := make(chan []byte, 2)
	require.NoError(t, client.Subscribe(shadow.TopicGetAcceptedOf(id), 1, func(_ mqtt.Client, m mqtt.Message) { received <- append([]byte(nil), m.Payload()...) }))
	payload, err := testCodec.Marshal(map[string]any{"clientToken": "default"})
	require.NoError(t, err)
	require.NoError(t, client.Publish(strings.Replace(shadow.TopicPrefixTmpl, "{thingId}", id, 1)+shadow.TopicGet, 1, false, payload))
	select {
	case raw := <-received:
		var reply shadow.StateAcceptedResp
		require.NoError(t, testCodec.Unmarshal(raw, &reply))
		require.Equal(t, "default", reply.ClientToken)
	case <-time.After(5 * time.Second):
		t.Fatal("default profile timeout")
	}
}

func TestDualSameConnectionProfileChange(t *testing.T) {
	if cfg.Protocol.Mode != "dual" {
		t.Skip("requires Dual mode")
	}
	id := ID()
	th := crateThing(id)
	client := newThingMqttClient(t.Context(), id, th.AuthValue)
	require.NoError(t, client.Connect(t.Context()))
	defer client.Disconnect()
	simpleDown := make(chan []byte, 8)
	legacyDown := make(chan []byte, 8)
	require.NoError(t, client.Subscribe(protocol.TopicDown(id, protocol.TypeShadowDelta), 1, func(_ mqtt.Client, m mqtt.Message) { simpleDown <- append([]byte(nil), m.Payload()...) }))
	require.NoError(t, client.Subscribe(shadow.TopicDeltaStateOf(id), 1, func(_ mqtt.Client, m mqtt.Message) { legacyDown <- append([]byte(nil), m.Payload()...) }))
	for _, p := range []connector.DeviceProfile{{MessageProtocol: "simple", PayloadEncoding: "cbor"}, {MessageProtocol: "legacy", PayloadEncoding: "json"}} {
		require.NoError(t, thingSvc.Update(t.Context(), id, thing.ThingPatch{
			MessageProtocol: &p.MessageProtocol, PayloadEncoding: &p.PayloadEncoding,
		}))
		c, err := codec.New(p.PayloadEncoding)
		require.NoError(t, err)
		ntpUp, ntpDown := "$iothub/things/"+id+"/ntp/req", "$iothub/things/"+id+"/ntp/resp"
		if p.MessageProtocol == "simple" {
			ntpUp = protocol.TopicUp(id, protocol.TypeNtpReq)
			ntpDown = protocol.TopicDown(id, protocol.TypeNtpResp)
		}
		ntpReplies := make(chan []byte, 2)
		require.NoError(t, client.Subscribe(ntpDown, 1, func(_ mqtt.Client, m mqtt.Message) { ntpReplies <- append([]byte(nil), m.Payload()...) }))
		raw, err := c.Marshal(map[string]any{"clientSendTime": 123})
		require.NoError(t, err)
		require.NoError(t, client.Publish(ntpUp, 1, false, raw))
		select {
		case raw := <-ntpReplies:
			var reply map[string]any
			require.NoError(t, c.Unmarshal(raw, &reply))
			require.EqualValues(t, 123, toInt64(reply["clientSendTime"]))
		case <-time.After(5 * time.Second):
			t.Fatal("NTP timeout")
		}
		_, err = shadowSvc.SetDesired(t.Context(), id, shadow.StateReq{State: shadow.StateDR{Desired: shadow.StateValue{"mode": p.MessageProtocol}}})
		require.NoError(t, err)
		selected, other := simpleDown, legacyDown
		if p.MessageProtocol == "legacy" {
			selected, other = legacyDown, simpleDown
		}
		select {
		case raw := <-selected:
			var reply map[string]any
			require.NoError(t, c.Unmarshal(raw, &reply))
		case <-time.After(5 * time.Second):
			t.Fatal("shadow delta timeout")
		}
		select {
		case <-other:
			t.Fatal("shadow delta reached the other protocol")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestDualThingCommunicationFieldsAPI(t *testing.T) {
	if cfg.Protocol.Mode != "dual" {
		t.Skip("requires Dual mode")
	}
	id := ID()
	body := `{"thingId":"` + id + `","password":"test-password","messageProtocol":"simple","payloadEncoding":"cbor","tags":{"zone":"east"}}`
	resp, err := http.Post(httpSvr.URL+"/api/v1/things/", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode, string(raw))
	var result struct {
		Data thing.Thing `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	require.Equal(t, "simple", result.Data.MessageProtocol)
	require.Equal(t, "cbor", result.Data.PayloadEncoding)
	patch := func(body string, status int) {
		req, err := http.NewRequest(http.MethodPatch, httpSvr.URL+"/api/v1/things/"+id, strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.NoError(t, err)
		require.Equal(t, status, resp.StatusCode, string(raw))
	}
	patch(`{"payloadEncoding":"json"}`, 200)
	current, err := thingSvc.Get(t.Context(), id)
	require.NoError(t, err)
	require.Equal(t, "simple", current.MessageProtocol)
	require.Equal(t, "json", current.PayloadEncoding)
	patch(`{"enabled":false,"messageProtocol":"unknown"}`, 400)
	current, err = thingSvc.Get(t.Context(), id)
	require.NoError(t, err)
	require.True(t, current.Enabled)
	require.Equal(t, "simple", current.MessageProtocol)
	patch(`{"messageProtocol":"","payloadEncoding":""}`, 200)
	profiles := thing.DeviceProfiles{Service: thingSvc, DefaultCodec: cfg.Protocol.Encoding}
	p, err := profiles.GetDeviceProfile(t.Context(), id)
	require.NoError(t, err)
	require.Equal(t, "legacy", p.MessageProtocol)
	require.Equal(t, cfg.Protocol.Encoding, p.PayloadEncoding)
	shadowState, err := shadowSvc.Get(t.Context(), id)
	require.NoError(t, err)
	require.Equal(t, int64(1), shadowState.Version)
	require.Equal(t, "east", shadowState.Tags["zone"])
}
