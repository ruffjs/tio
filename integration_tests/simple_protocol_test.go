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

func toInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case uint64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
}

func TestSimpleProtocol_ShadowUpdate(t *testing.T) {
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

	replyCh := make(chan map[string]any, 10)
	err = deviceClient.Subscribe("tio/"+thingId+"/down/shadow_update_reply", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if testCodec.Unmarshal(m.Payload(), &msg) == nil {
			replyCh <- msg
		}
	})
	require.NoError(t, err)

	updateMsg := map[string]any{
		"state": map[string]any{
			"color": "red",
			"temp":  25,
		},
	}
	payload, _ := testCodec.Marshal(updateMsg)
	err = deviceClient.Publish("tio/"+thingId+"/up/shadow_update", 1, false, payload)
	require.NoError(t, err)

	select {
	case reply := <-replyCh:
		require.EqualValues(t, 200, reply["code"])
		require.EqualValues(t, 2, reply["version"])
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for shadow_update_reply")
	}

	require.Eventually(t, func() bool {
		ss, err := shadowSvc.Get(ctx, thingId)
		return err == nil && ss.State.Reported["color"] == "red"
	}, 5*time.Second, 50*time.Millisecond)

	ss, err := shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	require.Equal(t, int64(2), ss.Version)
	require.Equal(t, "red", ss.State.Reported["color"])
	require.EqualValues(t, 25, ss.State.Reported["temp"])

	updateMsg2 := map[string]any{
		"state": map[string]any{
			"color":    "blue",
			"humidity": 60,
		},
	}
	payload2, _ := testCodec.Marshal(updateMsg2)
	err = deviceClient.Publish("tio/"+thingId+"/up/shadow_update", 1, false, payload2)
	require.NoError(t, err)

	select {
	case reply := <-replyCh:
		require.EqualValues(t, 200, reply["code"])
		require.EqualValues(t, 3, reply["version"])
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for second shadow_update_reply")
	}

	require.Eventually(t, func() bool {
		ss, err := shadowSvc.Get(ctx, thingId)
		return err == nil && ss.State.Reported["color"] == "blue"
	}, 5*time.Second, 50*time.Millisecond)

	ss, err = shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	require.Equal(t, int64(3), ss.Version)
	require.Equal(t, "blue", ss.State.Reported["color"])
	require.EqualValues(t, 25, ss.State.Reported["temp"])
	require.EqualValues(t, 60, ss.State.Reported["humidity"])
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

	publishUpdate := func(state map[string]any) {
		msg := map[string]any{"state": state}
		payload, _ := testCodec.Marshal(msg)
		err := deviceClient.Publish("tio/"+thingId+"/up/shadow_update", 1, false, payload)
		require.NoError(t, err)
	}

	publishUpdate(map[string]any{
		"config": map[string]any{
			"mode":    "auto",
			"level":   5,
			"options": []any{"a", "b"},
		},
	})

	require.Eventually(t, func() bool {
		ss, err := shadowSvc.Get(ctx, thingId)
		return err == nil && ss.State.Reported["config"] != nil
	}, 5*time.Second, 50*time.Millisecond)

	publishUpdate(map[string]any{
		"config": map[string]any{
			"level":   10,
			"options": []any{"c"},
			"debug":   true,
		},
	})

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
	require.EqualValues(t, 10, cfgMap["level"])
	require.Equal(t, true, cfgMap["debug"])
	opts := cfgMap["options"].([]any)
	require.Equal(t, []any{"c"}, opts)

	publishUpdate(map[string]any{
		"config": map[string]any{
			"debug": nil,
		},
	})

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

func TestSimpleProtocol_ShadowGetReturnsFullState(t *testing.T) {
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

	updateMsg := map[string]any{
		"state": map[string]any{"color": "green"},
	}
	payload, _ := testCodec.Marshal(updateMsg)
	err = deviceClient.Publish("tio/"+thingId+"/up/shadow_update", 1, false, payload)
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

	replyCh := make(chan map[string]any, 5)
	err = deviceClient.Subscribe("tio/"+thingId+"/down/shadow_get_reply", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if testCodec.Unmarshal(m.Payload(), &msg) == nil {
			replyCh <- msg
		}
	})
	require.NoError(t, err)

	getPayload, _ := testCodec.Marshal(map[string]any{})
	err = deviceClient.Publish("tio/"+thingId+"/up/shadow_get", 1, false, getPayload)
	require.NoError(t, err)

	var getReply map[string]any
	select {
	case getReply = <-replyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for shadow_get_reply")
	}

	require.EqualValues(t, 200, getReply["code"])

	ss, err := shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	require.EqualValues(t, ss.Version, getReply["version"])

	state := getReply["state"].(map[string]any)
	desired := state["desired"].(map[string]any)
	reported := state["reported"].(map[string]any)
	require.EqualValues(t, 80, desired["brightness"])
	require.Equal(t, "green", reported["color"])

	verAfterGet := ss.Version

	getPayload2, _ := testCodec.Marshal(map[string]any{})
	err = deviceClient.Publish("tio/"+thingId+"/up/shadow_get", 1, false, getPayload2)
	require.NoError(t, err)

	select {
	case <-replyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for second shadow_get_reply")
	}

	ss, err = shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	require.Equal(t, verAfterGet, ss.Version, "get should not change version")
}

func TestSimpleProtocol_ShadowUpdateVersionConflict(t *testing.T) {
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

	replyCh := make(chan map[string]any, 5)
	err = deviceClient.Subscribe("tio/"+thingId+"/down/shadow_update_reply", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if testCodec.Unmarshal(m.Payload(), &msg) == nil {
			replyCh <- msg
		}
	})
	require.NoError(t, err)

	updateMsg := map[string]any{
		"state": map[string]any{"color": "red"},
	}
	payload, _ := testCodec.Marshal(updateMsg)
	err = deviceClient.Publish("tio/"+thingId+"/up/shadow_update", 1, false, payload)
	require.NoError(t, err)

	select {
	case reply := <-replyCh:
		require.EqualValues(t, 200, reply["code"])
		require.EqualValues(t, 2, reply["version"])
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for first update reply")
	}

	conflictMsg := map[string]any{
		"version": 1,
		"state":   map[string]any{"color": "blue"},
	}
	conflictPayload, _ := testCodec.Marshal(conflictMsg)
	err = deviceClient.Publish("tio/"+thingId+"/up/shadow_update", 1, false, conflictPayload)
	require.NoError(t, err)

	select {
	case reply := <-replyCh:
		require.EqualValues(t, 409, reply["code"])
		require.EqualValues(t, 2, reply["version"])
		require.NotEmpty(t, reply["message"])
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for conflict reply")
	}

	ss, err := shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	require.Equal(t, "red", ss.State.Reported["color"])
}

func TestSimpleProtocol_ShadowUpdateNoOp(t *testing.T) {
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

	replyCh := make(chan map[string]any, 5)
	err = deviceClient.Subscribe("tio/"+thingId+"/down/shadow_update_reply", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if testCodec.Unmarshal(m.Payload(), &msg) == nil {
			replyCh <- msg
		}
	})
	require.NoError(t, err)

	updateMsg := map[string]any{
		"state": map[string]any{"color": "red"},
	}
	payload, _ := testCodec.Marshal(updateMsg)
	err = deviceClient.Publish("tio/"+thingId+"/up/shadow_update", 1, false, payload)
	require.NoError(t, err)

	select {
	case reply := <-replyCh:
		require.EqualValues(t, 200, reply["code"])
		require.EqualValues(t, 2, reply["version"])
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for first update reply")
	}

	noopMsg := map[string]any{
		"state": map[string]any{"color": "red"},
	}
	noopPayload, _ := testCodec.Marshal(noopMsg)
	err = deviceClient.Publish("tio/"+thingId+"/up/shadow_update", 1, false, noopPayload)
	require.NoError(t, err)

	select {
	case reply := <-replyCh:
		require.EqualValues(t, 200, reply["code"])
		require.EqualValues(t, 2, reply["version"], "no-op should not increment version")
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for no-op reply")
	}

	ss, err := shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	require.Equal(t, int64(2), ss.Version, "no-op should not increment version")
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
	err = deviceClient.Subscribe("tio/"+thingId+"/down/method_req", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if testCodec.Unmarshal(m.Payload(), &msg) == nil {
			callCh <- msg
		}
	})
	require.NoError(t, err)

	go func() {
		select {
		case callMsg := <-callCh:
			callID := callMsg["id"].(string)
			replyMsg := map[string]any{
				"id":   callID,
				"code": 200,
				"data": map[string]any{
					"result": "success",
					"nested": map[string]any{
						"value": 42,
					},
				},
			}
			replyPayload, _ := testCodec.Marshal(replyMsg)
			deviceClient.Publish("tio/"+thingId+"/up/method_resp", 1, false, replyPayload)
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

	var respBody rest.Resp[rest.H]
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)
	require.Equal(t, 200, respBody.Code)
	respData := respBody.Data["data"].(map[string]any)
	require.Equal(t, "success", respData["result"])
	nested := respData["nested"].(map[string]any)
	require.EqualValues(t, 42, nested["value"])
	resp.Body.Close()
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
	err = deviceClient.Subscribe("tio/"+thingId+"/down/method_req", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if testCodec.Unmarshal(m.Payload(), &msg) == nil {
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
	err = deviceClient.Subscribe("tio/"+thingId+"/down/method_req", 1, func(c mqtt.Client, m mqtt.Message) {
		select {
		case callReceived <- struct{}{}:
		default:
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

func TestSimpleProtocol_DeltaDesiredNotification(t *testing.T) {
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

	desiredCh := make(chan map[string]any, 10)
	err = deviceClient.Subscribe("tio/"+thingId+"/down/shadow_desired", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if testCodec.Unmarshal(m.Payload(), &msg) == nil {
			desiredCh <- msg
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

	var desiredMsg map[string]any
	select {
	case desiredMsg = <-desiredCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for shadow_desired notification")
	}

	state := desiredMsg["state"].(map[string]any)
	require.Equal(t, "red", state["color"])
	require.EqualValues(t, 50, state["brightness"])

	updateMsg := map[string]any{
		"state": map[string]any{
			"color":      "blue",
			"brightness": float64(50),
		},
	}
	payload, _ := testCodec.Marshal(updateMsg)
	err = deviceClient.Publish("tio/"+thingId+"/up/shadow_update", 1, false, payload)
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
	case <-desiredCh:
		t.Fatal("should not receive shadow_desired when desired equals reported")
	case <-time.After(500 * time.Millisecond):
	}

	afterEqualDesired, err := shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	require.Equal(t, beforeEqualDesired.Version+1, afterEqualDesired.Version,
		"desired changing to equal reported must still increment version")
}

func TestSimpleProtocol_ReportedUpdateDoesNotTriggerDesired(t *testing.T) {
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

	desiredCh := make(chan map[string]any, 10)
	err = deviceClient.Subscribe("tio/"+thingId+"/down/shadow_desired", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if testCodec.Unmarshal(m.Payload(), &msg) == nil {
			desiredCh <- msg
		}
	})
	require.NoError(t, err)

	setDesiredBody := strings.NewReader(`{"clientToken":"ct-1","state":{"desired":{"color":"red","brightness":50}}}`)
	req, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/v1/things/%s/shadows/default/state/desired", httpSvr.URL, thingId), setDesiredBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpSvr.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	select {
	case <-desiredCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for initial shadow_desired")
	}

	updateMsg := map[string]any{
		"state": map[string]any{
			"color":      "blue",
			"brightness": 50,
		},
	}
	payload, _ := testCodec.Marshal(updateMsg)
	err = deviceClient.Publish("tio/"+thingId+"/up/shadow_update", 1, false, payload)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		ss, err := shadowSvc.Get(ctx, thingId)
		return err == nil && ss.State.Reported["color"] == "blue"
	}, 5*time.Second, 50*time.Millisecond)

	select {
	case <-desiredCh:
		t.Fatal("reported update must not trigger shadow_desired")
	case <-time.After(500 * time.Millisecond):
	}
}

func TestSimpleProtocol_NtpTimeSync(t *testing.T) {
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

	replyCh := make(chan map[string]any, 5)
	err = deviceClient.Subscribe("tio/"+thingId+"/down/ntp_resp", 1, func(c mqtt.Client, m mqtt.Message) {
		var msg map[string]any
		if testCodec.Unmarshal(m.Payload(), &msg) == nil {
			replyCh <- msg
		}
	})
	require.NoError(t, err)

	clientSendTime := time.Now().UnixMilli()
	ntpReq := map[string]any{"clientSendTime": clientSendTime}
	payload, _ := testCodec.Marshal(ntpReq)
	err = deviceClient.Publish("tio/"+thingId+"/up/ntp_req", 1, false, payload)
	require.NoError(t, err)

	select {
	case reply := <-replyCh:
		require.EqualValues(t, 200, reply["code"])
		require.EqualValues(t, clientSendTime, reply["clientSendTime"])
		recv := toInt64(reply["serverRecvTime"])
		send := toInt64(reply["serverSendTime"])
		require.True(t, recv > 0)
		require.True(t, send >= recv)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for ntp_resp")
	}
}

func TestSimpleProtocol_CustomTopicFreePubSub(t *testing.T) {
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

	customTopic := "tio/" + thingId + "/custom/channel"
	received := make(chan []byte, 5)
	err = deviceClient.Subscribe(customTopic, 1, func(c mqtt.Client, m mqtt.Message) {
		received <- m.Payload()
	})
	require.NoError(t, err)

	payload := []byte(`{"hello":"world"}`)
	err = deviceClient.Publish(customTopic, 1, false, payload)
	require.NoError(t, err)

	select {
	case got := <-received:
		require.Equal(t, payload, got, "custom topic payload should match")
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for custom topic message")
	}
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
