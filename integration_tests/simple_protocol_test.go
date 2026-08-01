//go:build integration

package integration_tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/config"
	rest "ruff.io/tio/pkg/restapi"
)

func TestSimpleProtocol_IncrementalReport(t *testing.T) {
	if cfg.Protocol.Mode != "simple" {
		t.Skip("Skipping test: only runs in simple protocol mode")
	}

	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	thingId := ID()
	th := crateThing(thingId)
	deviceClient := newThingMqttClient(ctx, th.Id, th.AuthValue)
	err := deviceClient.Connect(ctx)
	require.NoError(t, err)
	defer deviceClient.Disconnect()
	waitConnected(t, thingId)

	setCh := make(chan map[string]any, 10)
	err = deviceClient.Subscribe("tio/"+thingId+"/down", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if err := testCodec.Unmarshal(m.Payload(), &msg); err != nil {
			return
		}
		if msg["t"] == "set" {
			setCh <- msg
		}
	})
	require.NoError(t, err)

	reportMsg := map[string]any{
		"t":  "report",
		"id": "rpt-1",
		"d": map[string]any{
			"version": 0,
			"state": map[string]any{
				"color": "red",
				"temp":  25,
			},
		},
	}
	payload, _ := testCodec.Marshal(reportMsg)
	err = deviceClient.Publish("tio/"+thingId+"/up", 1, false, payload)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		ss, err := shadowSvc.Get(ctx, thingId)
		return err == nil && ss.State.Reported["color"] == "red"
	}, 5*time.Second, 50*time.Millisecond)

	ss, err := shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	require.Equal(t, int64(1), ss.Version)
	require.Equal(t, "red", ss.State.Reported["color"])
	require.Equal(t, float64(25), ss.State.Reported["temp"])

	reportMsg2 := map[string]any{
		"t":  "report",
		"id": "rpt-2",
		"d": map[string]any{
			"version": 1,
			"state": map[string]any{
				"color":    "blue",
				"humidity": 60,
			},
		},
	}
	payload2, _ := testCodec.Marshal(reportMsg2)
	err = deviceClient.Publish("tio/"+thingId+"/up", 1, false, payload2)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		ss, err := shadowSvc.Get(ctx, thingId)
		return err == nil && ss.State.Reported["color"] == "blue"
	}, 5*time.Second, 50*time.Millisecond)

	ss, err = shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	require.Equal(t, int64(1), ss.Version)
	require.Equal(t, "blue", ss.State.Reported["color"])
	require.Equal(t, float64(25), ss.State.Reported["temp"])
	require.Equal(t, float64(60), ss.State.Reported["humidity"])
}

func TestSimpleProtocol_RecursiveMergeAndNullDelete(t *testing.T) {
	if cfg.Protocol.Mode != "simple" {
		t.Skip("Skipping test: only runs in simple protocol mode")
	}

	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	thingId := ID()
	th := crateThing(thingId)
	deviceClient := newThingMqttClient(ctx, th.Id, th.AuthValue)
	err := deviceClient.Connect(ctx)
	require.NoError(t, err)
	defer deviceClient.Disconnect()
	waitConnected(t, thingId)

	reportMsg := map[string]any{
		"t":  "report",
		"id": "rpt-merge-1",
		"d": map[string]any{
			"version": 0,
			"state": map[string]any{
				"config": map[string]any{
					"mode":    "auto",
					"level":   5,
					"options": []any{"a", "b"},
				},
			},
		},
	}
	payload, _ := testCodec.Marshal(reportMsg)
	err = deviceClient.Publish("tio/"+thingId+"/up", 1, false, payload)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		ss, err := shadowSvc.Get(ctx, thingId)
		return err == nil && ss.State.Reported["config"] != nil
	}, 5*time.Second, 50*time.Millisecond)

	reportMsg2 := map[string]any{
		"t":  "report",
		"id": "rpt-merge-2",
		"d": map[string]any{
			"version": 0,
			"state": map[string]any{
				"config": map[string]any{
					"level":   10,
					"options": []any{"c"},
					"debug":   true,
				},
			},
		},
	}
	payload2, _ := testCodec.Marshal(reportMsg2)
	err = deviceClient.Publish("tio/"+thingId+"/up", 1, false, payload2)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		ss, err := shadowSvc.Get(ctx, thingId)
		if err != nil {
			return false
		}
		cfgMap, ok := ss.State.Reported["config"].(map[string]any)
		if !ok {
			return false
		}
		return cfgMap["mode"] == "auto" && cfgMap["level"] == float64(10) && cfgMap["debug"] == true
	}, 5*time.Second, 50*time.Millisecond)

	ss, err := shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	cfgMap := ss.State.Reported["config"].(map[string]any)
	require.Equal(t, "auto", cfgMap["mode"])
	require.Equal(t, float64(10), cfgMap["level"])
	require.Equal(t, true, cfgMap["debug"])
	opts := cfgMap["options"].([]any)
	require.Equal(t, []any{"c"}, opts)

	reportMsg3 := map[string]any{
		"t":  "report",
		"id": "rpt-merge-3",
		"d": map[string]any{
			"version": 0,
			"state": map[string]any{
				"config": map[string]any{
					"debug": nil,
				},
			},
		},
	}
	payload3, _ := testCodec.Marshal(reportMsg3)
	err = deviceClient.Publish("tio/"+thingId+"/up", 1, false, payload3)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		ss, err := shadowSvc.Get(ctx, thingId)
		if err != nil {
			return false
		}
		cfgMap, ok := ss.State.Reported["config"].(map[string]any)
		if !ok {
			return false
		}
		_, hasDebug := cfgMap["debug"]
		return !hasDebug && cfgMap["mode"] == "auto"
	}, 5*time.Second, 50*time.Millisecond)
}

func TestSimpleProtocol_GetReturnsFullState(t *testing.T) {
	if cfg.Protocol.Mode != "simple" {
		t.Skip("Skipping test: only runs in simple protocol mode")
	}

	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	thingId := ID()
	th := crateThing(thingId)
	deviceClient := newThingMqttClient(ctx, th.Id, th.AuthValue)
	err := deviceClient.Connect(ctx)
	require.NoError(t, err)
	defer deviceClient.Disconnect()
	waitConnected(t, thingId)

	reportMsg := map[string]any{
		"t":  "report",
		"id": "rpt-get-1",
		"d": map[string]any{
			"version": 0,
			"state": map[string]any{
				"color": "green",
			},
		},
	}
	payload, _ := testCodec.Marshal(reportMsg)
	err = deviceClient.Publish("tio/"+thingId+"/up", 1, false, payload)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		ss, err := shadowSvc.Get(ctx, thingId)
		return err == nil && ss.State.Reported["color"] == "green"
	}, 5*time.Second, 50*time.Millisecond)

	setDesiredBody := strings.NewReader(`{"clientToken":"ct-1","state":{"desired":{"brightness":80}}}`)
	req, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/v1/things/%s/shadows/default/state/desired", httpSvr.URL, thingId), setDesiredBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpSvr.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	require.Eventually(t, func() bool {
		ss, err := shadowSvc.Get(ctx, thingId)
		return err == nil && ss.State.Desired["brightness"] == float64(80)
	}, 5*time.Second, 50*time.Millisecond)

	downCh := make(chan map[string]any, 5)
	err = deviceClient.Subscribe("tio/"+thingId+"/down", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if err := testCodec.Unmarshal(m.Payload(), &msg); err != nil {
			return
		}
		downCh <- msg
	})
	require.NoError(t, err)

	getMsg := map[string]any{
		"t":  "get",
		"id": "get-1",
	}
	getPayload, _ := testCodec.Marshal(getMsg)
	err = deviceClient.Publish("tio/"+thingId+"/up", 1, false, getPayload)
	require.NoError(t, err)

	var setMsg map[string]any
	select {
	case setMsg = <-downCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for set response to get")
	}

	require.Equal(t, "set", setMsg["t"])
	require.Equal(t, "get-1", setMsg["id"])

	data := setMsg["d"].(map[string]any)
	state := data["state"].(map[string]any)
	require.Equal(t, float64(80), state["brightness"])

	ss, err := shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	verAfterGet := ss.Version

	getMsg2 := map[string]any{
		"t":  "get",
		"id": "get-2",
	}
	getPayload2, _ := testCodec.Marshal(getMsg2)
	err = deviceClient.Publish("tio/"+thingId+"/up", 1, false, getPayload2)
	require.NoError(t, err)

	select {
	case <-downCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for second set response to get")
	}

	ss, err = shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	require.Equal(t, verAfterGet, ss.Version, "get should not change version")
}

func TestSimpleProtocol_MethodCallAndReply(t *testing.T) {
	if cfg.Protocol.Mode != "simple" {
		t.Skip("Skipping test: only runs in simple protocol mode")
	}

	ctx, cancel := context.WithTimeout(testCtx, 15*time.Second)
	defer cancel()

	thingId := ID()
	th := crateThing(thingId)
	deviceClient := newThingMqttClient(ctx, th.Id, th.AuthValue)
	err := deviceClient.Connect(ctx)
	require.NoError(t, err)
	defer deviceClient.Disconnect()
	waitConnected(t, thingId)

	callCh := make(chan map[string]any, 5)
	err = deviceClient.Subscribe("tio/"+thingId+"/down", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if err := testCodec.Unmarshal(m.Payload(), &msg); err != nil {
			return
		}
		if msg["t"] == "call" {
			callCh <- msg
		}
	})
	require.NoError(t, err)

	go func() {
		select {
		case callMsg := <-callCh:
			callID := callMsg["id"].(string)
			replyMsg := map[string]any{
				"t":  "reply",
				"id": callID,
				"d": map[string]any{
					"result": "success",
					"nested": map[string]any{
						"value": 42,
					},
				},
			}
			replyPayload, _ := testCodec.Marshal(replyMsg)
			deviceClient.Publish("tio/"+thingId+"/up", 1, false, replyPayload)
		case <-ctx.Done():
		}
	}()

	methodBody := strings.NewReader(`{
		"method": "doSomething",
		"params": {"action": "test", "key": "value"},
		"timeout": 5
	}`)
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/v1/things/%s/invoke", httpSvr.URL, thingId), methodBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpSvr.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var respBody rest.Resp[any]
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, respBody.Code)
	replyData, ok := respBody.Data.(map[string]any)
	require.True(t, ok, "reply data should be an object")
	require.Equal(t, "success", replyData["result"])
	nested, ok := replyData["nested"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(42), nested["value"])
	resp.Body.Close()
}

func TestSimpleProtocol_MethodCallMNotOverridden(t *testing.T) {
	if cfg.Protocol.Mode != "simple" {
		t.Skip("Skipping test: only runs in simple protocol mode")
	}

	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	thingId := ID()
	th := crateThing(thingId)
	deviceClient := newThingMqttClient(ctx, th.Id, th.AuthValue)
	err := deviceClient.Connect(ctx)
	require.NoError(t, err)
	defer deviceClient.Disconnect()
	waitConnected(t, thingId)

	callCh := make(chan map[string]any, 5)
	err = deviceClient.Subscribe("tio/"+thingId+"/down", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if err := testCodec.Unmarshal(m.Payload(), &msg); err != nil {
			return
		}
		if msg["t"] == "call" {
			callCh <- msg
		}
	})
	require.NoError(t, err)

	methodBody := strings.NewReader(`{
		"method": "realMethod",
		"params": {"m": "evil", "action": "test"},
		"timeout": 2
	}`)
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/v1/things/%s/invoke", httpSvr.URL, thingId), methodBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpSvr.Client().Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	var callMsg map[string]any
	select {
	case callMsg = <-callCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for call message")
	}

	d := callMsg["d"].(map[string]any)
	require.Equal(t, "realMethod", d["m"], "m field must be the method name, not overridable by params")
}

func TestSimpleProtocol_EventDataTransparent(t *testing.T) {
	if cfg.Protocol.Mode != "simple" {
		t.Skip("Skipping test: only runs in simple protocol mode")
	}

	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	thingId := ID()
	th := crateThing(thingId)
	deviceClient := newThingMqttClient(ctx, th.Id, th.AuthValue)
	err := deviceClient.Connect(ctx)
	require.NoError(t, err)
	defer deviceClient.Disconnect()
	waitConnected(t, thingId)

	nc := natsConnector.AppConn()

	eventSubj := "tio." + thingId + ".event"
	dataSubj := "tio." + thingId + ".data"

	eventSub, err := nc.SubscribeSync(eventSubj)
	require.NoError(t, err)
	defer eventSub.Unsubscribe()

	dataSub, err := nc.SubscribeSync(dataSubj)
	require.NoError(t, err)
	defer dataSub.Unsubscribe()

	err = nc.Flush()
	require.NoError(t, err)

	ss, err := shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	initialVersion := ss.Version

	eventPayload := []byte(`{"type":"alarm","severity":"high","timestamp":1234567890}`)
	err = deviceClient.Publish("tio/"+thingId+"/event", 1, false, eventPayload)
	require.NoError(t, err)

	eventMsg, err := eventSub.NextMsg(5 * time.Second)
	require.NoError(t, err)
	require.Equal(t, eventPayload, eventMsg.Data, "Event payload should be delivered byte-for-byte")

	dataPayload := []byte(`{"temperature":25.5,"humidity":60}`)
	err = deviceClient.Publish("tio/"+thingId+"/data", 0, false, dataPayload)
	require.NoError(t, err)

	dataMsg, err := dataSub.NextMsg(5 * time.Second)
	require.NoError(t, err)
	require.Equal(t, dataPayload, dataMsg.Data, "Data payload should be delivered byte-for-byte")

	time.Sleep(200 * time.Millisecond)

	ss, err = shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	require.Equal(t, initialVersion, ss.Version, "Shadow version should not change after Event/Data")
	require.Empty(t, ss.State.Desired, "Shadow desired state should be empty")
	require.Empty(t, ss.State.Reported, "Shadow reported state should be empty")
}

func TestSimpleProtocol_DeltaSetNotification(t *testing.T) {
	if cfg.Protocol.Mode != "simple" {
		t.Skip("Skipping test: only runs in simple protocol mode")
	}

	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	thingId := ID()
	th := crateThing(thingId)
	deviceClient := newThingMqttClient(ctx, th.Id, th.AuthValue)
	err := deviceClient.Connect(ctx)
	require.NoError(t, err)
	defer deviceClient.Disconnect()
	waitConnected(t, thingId)

	setCh := make(chan map[string]any, 10)
	err = deviceClient.Subscribe("tio/"+thingId+"/down", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if err := testCodec.Unmarshal(m.Payload(), &msg); err != nil {
			return
		}
		if msg["t"] == "set" {
			setCh <- msg
		}
	})
	require.NoError(t, err)

	setDesiredBody := strings.NewReader(`{"clientToken":"ct-delta-1","state":{"desired":{"color":"red","brightness":50}}}`)
	req, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/v1/things/%s/shadows/default/state/desired", httpSvr.URL, thingId), setDesiredBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpSvr.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	var setMsg map[string]any
	select {
	case setMsg = <-setCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for set notification")
	}

	require.Equal(t, "set", setMsg["t"])
	data := setMsg["d"].(map[string]any)
	state := data["state"].(map[string]any)
	require.Equal(t, "red", state["color"])
	require.Equal(t, float64(50), state["brightness"])

	reportMsg := map[string]any{
		"t":  "report",
		"id": "rpt-delta-1",
		"d": map[string]any{
			"version": 0,
			"state": map[string]any{
				"color":      "blue",
				"brightness": float64(50),
			},
		},
	}
	payload, _ := testCodec.Marshal(reportMsg)
	err = deviceClient.Publish("tio/"+thingId+"/up", 1, false, payload)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		ss, err := shadowSvc.Get(ctx, thingId)
		return err == nil && ss.State.Reported["color"] == "blue"
	}, 5*time.Second, 50*time.Millisecond)
	beforeEqualDesired, err := shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)

	setDesiredBody2 := strings.NewReader(`{"clientToken":"ct-delta-2","state":{"desired":{"color":"blue","brightness":50}}}`)
	req2, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/v1/things/%s/shadows/default/state/desired", httpSvr.URL, thingId), setDesiredBody2)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := httpSvr.Client().Do(req2)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	resp2.Body.Close()

	select {
	case <-setCh:
		t.Fatal("should not receive set notification when desired equals reported")
	case <-time.After(500 * time.Millisecond):
	}

	afterEqualDesired, err := shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	require.Equal(t, beforeEqualDesired.Version+1, afterEqualDesired.Version,
		"desired changing to equal reported must still increment version")
}

func TestSimpleProtocol_ConfigValidation(t *testing.T) {
	tests := []struct {
		name  string
		mode  string
		valid bool
	}{
		{"legacy mode is valid", "legacy", true},
		{"simple mode is valid", "simple", true},
		{"empty mode is invalid", "", false},
		{"unknown mode is invalid", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := config.Protocol{Mode: tt.mode, Encoding: "json"}
			err := p.Validate()
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestSimpleProtocol_MethodCallTimeout(t *testing.T) {
	if cfg.Protocol.Mode != "simple" {
		t.Skip("Skipping test: only runs in simple protocol mode")
	}

	ctx, cancel := context.WithTimeout(testCtx, 15*time.Second)
	defer cancel()

	thingId := ID()
	th := crateThing(thingId)
	deviceClient := newThingMqttClient(ctx, th.Id, th.AuthValue)
	err := deviceClient.Connect(ctx)
	require.NoError(t, err)
	defer deviceClient.Disconnect()
	waitConnected(t, thingId)

	callCh := make(chan map[string]any, 5)
	err = deviceClient.Subscribe("tio/"+thingId+"/down", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if err := testCodec.Unmarshal(m.Payload(), &msg); err != nil {
			return
		}
		if msg["t"] == "call" {
			callCh <- msg
		}
	})
	require.NoError(t, err)

	methodBody := strings.NewReader(`{
		"method": "timeoutTest",
		"params": {"key": "value"},
		"timeout": 1
	}`)
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/v1/things/%s/invoke", httpSvr.URL, thingId), methodBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpSvr.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var respBody rest.Resp[any]
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)
	require.Equal(t, 504, respBody.Code, "should return timeout error code")
	resp.Body.Close()

	select {
	case <-callCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for call message")
	}
}

func TestSimpleProtocol_MethodCallHTTPCancellation(t *testing.T) {
	if cfg.Protocol.Mode != "simple" {
		t.Skip("Skipping test: only runs in simple protocol mode")
	}

	ctx, cancel := context.WithTimeout(testCtx, 15*time.Second)
	defer cancel()

	thingId := ID()
	th := crateThing(thingId)
	deviceClient := newThingMqttClient(ctx, th.Id, th.AuthValue)
	err := deviceClient.Connect(ctx)
	require.NoError(t, err)
	defer deviceClient.Disconnect()
	waitConnected(t, thingId)

	callReceived := make(chan struct{}, 1)
	err = deviceClient.Subscribe("tio/"+thingId+"/down", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if err := testCodec.Unmarshal(m.Payload(), &msg); err != nil {
			return
		}
		if msg["t"] == "call" {
			select {
			case callReceived <- struct{}{}:
			default:
			}
		}
	})
	require.NoError(t, err)

	httpCtx, httpCancel := context.WithCancel(ctx)
	methodBody := strings.NewReader(`{
		"method": "cancelTest",
		"params": {"key": "value"},
		"timeout": 30
	}`)
	req, _ := http.NewRequestWithContext(httpCtx, http.MethodPost,
		fmt.Sprintf("%s/api/v1/things/%s/invoke", httpSvr.URL, thingId), methodBody)
	req.Header.Set("Content-Type", "application/json")

	done := make(chan struct{})
	var resp *http.Response
	var doErr error
	go func() {
		resp, doErr = httpSvr.Client().Do(req)
		close(done)
	}()

	select {
	case <-callReceived:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for call message")
	}

	httpCancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for HTTP request to complete after cancellation")
	}

	if doErr == nil {
		resp.Body.Close()
	}
}

func TestSimpleProtocol_LegacyMethodRouteNotExposed(t *testing.T) {
	if cfg.Protocol.Mode != "simple" {
		t.Skip("Skipping test: only runs in simple protocol mode")
	}

	thingId := ID()
	crateThing(thingId)

	methodBody := strings.NewReader(`{
		"respTimeout": 1,
		"data": {"hello": "world"}
	}`)
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/v1/things/%s/methods/testMethod", httpSvr.URL, thingId), methodBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpSvr.Client().Do(req)
	require.NoError(t, err)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode,
		"legacy method route should not be registered in simple mode")
}
