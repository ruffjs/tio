package protocol

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	connmock "ruff.io/tio/connector/mock"
	dbmock "ruff.io/tio/db/mock"
	"ruff.io/tio/pkg/codec"
	"ruff.io/tio/shadow"
	shadowMock "ruff.io/tio/shadow/mock"
	"ruff.io/tio/thing"
)

func newTestShadowSvc() shadow.Service {
	db := dbmock.NewSqliteConnTest()
	err := db.AutoMigrate(&thing.Entity{}, &shadow.Entity{}, &shadow.ConnStatusEntity{})
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Millisecond * 100)
	return shadow.NewTestSvc(shadow.NewShadowRepo(db), shadowMock.NewConnectivity(), shadow.Config{})
}

func TestTopicBuilders(t *testing.T) {
	assert.Equal(t, "tio/device1/up", TopicUp("device1"))
	assert.Equal(t, "tio/device1/down", TopicDown("device1"))
	assert.Equal(t, "tio/device1/event", TopicEvent("device1"))
	assert.Equal(t, "tio/device1/data", TopicData("device1"))
	assert.Equal(t, "tio/+/up", TopicAllUp())
}

func TestParseTopic(t *testing.T) {
	tests := []struct {
		name        string
		topic       string
		wantThingId string
		wantLevel   string
		wantErr     bool
	}{
		{"valid up", "tio/device1/up", "device1", LevelUp, false},
		{"valid down", "tio/device1/down", "device1", LevelDown, false},
		{"valid event", "tio/device1/event", "device1", LevelEvent, false},
		{"valid data", "tio/device1/data", "device1", LevelData, false},
		{"invalid prefix", "mqtt/device1/up", "", "", true},
		{"invalid structure", "tio/device1", "", "", true},
		{"invalid level", "tio/device1/invalid", "", "", true},
		{"wildcard thingId", "tio/+/up", "", "", true},
		{"empty thingId", "tio//up", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thingId, level, err := ParseTopic(tt.topic)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantThingId, thingId)
				assert.Equal(t, tt.wantLevel, level)
			}
		})
	}
}

func TestValidateReport(t *testing.T) {
	tests := []struct {
		name    string
		data    any
		wantErr bool
	}{
		{name: "json version", data: map[string]any{"version": float64(1), "state": map[string]any{}}},
		{name: "cbor version", data: map[string]any{"version": uint64(1), "state": map[string]any{}}},
		{name: "zero version", data: map[string]any{"version": 0, "state": map[string]any{}}},
		{name: "missing version", data: map[string]any{"state": map[string]any{}}, wantErr: true},
		{name: "fractional version", data: map[string]any{"version": 1.5, "state": map[string]any{}}, wantErr: true},
		{name: "negative version", data: map[string]any{"version": -1, "state": map[string]any{}}, wantErr: true},
		{name: "string version", data: map[string]any{"version": "1", "state": map[string]any{}}, wantErr: true},
		{name: "missing state", data: map[string]any{"version": 1}, wantErr: true},
		{name: "non-object data", data: "invalid", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReport(ControlMessage{Type: MsgTypeReport, Data: tt.data})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSimpleHandler_Report(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockConn := connmock.NewMockConnector()
	mockConn.Start(ctx)
	c, _ := codec.New("json")
	shadowSvc := newTestShadowSvc()

	handler, err := NewSimpleHandler(mockConn, c, shadowSvc)
	require.NoError(t, err)
	defer handler.Stop()

	err = handler.Start(ctx)
	require.NoError(t, err)

	thingId := "test-device-report"
	_, err = shadowSvc.Create(ctx, thingId)
	require.NoError(t, err)

	reportMsg := ControlMessage{
		Type: MsgTypeReport,
		ID:   "msg-1",
		Data: map[string]any{
			"version": 1.0,
			"state": map[string]any{
				"temperature": 25.5,
			},
		},
	}

	payload, err := c.Marshal(reportMsg)
	require.NoError(t, err)

	mockConn.SimulateMessage(TopicUp(thingId), payload)

	require.Eventually(t, func() bool {
		ss, getErr := shadowSvc.Get(ctx, thingId)
		return getErr == nil && ss.State.Reported["temperature"] == 25.5
	}, time.Second, 10*time.Millisecond)
}

func TestSimpleHandler_Get(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockConn := connmock.NewMockConnector()
	mockConn.Start(ctx)
	c, _ := codec.New("json")
	shadowSvc := newTestShadowSvc()

	handler, err := NewSimpleHandler(mockConn, c, shadowSvc)
	require.NoError(t, err)
	defer handler.Stop()

	err = handler.Start(ctx)
	require.NoError(t, err)

	thingId := "test-device-get"
	_, err = shadowSvc.Create(ctx, thingId)
	require.NoError(t, err)

	_, err = shadowSvc.SetDesired(ctx, thingId, shadow.StateReq{
		State: shadow.StateDR{
			Desired: map[string]any{
				"target": 30.0,
			},
		},
		Version: 1,
	})
	require.NoError(t, err)

	getMsg := ControlMessage{
		Type: MsgTypeGet,
		ID:   "msg-2",
	}
	payload, err := c.Marshal(getMsg)
	require.NoError(t, err)

	mockConn.SimulateMessage(TopicUp(thingId), payload)

	var setMsg ControlMessage
	require.Eventually(t, func() bool {
		for _, pub := range mockConn.PublishedMessages() {
			var msg ControlMessage
			if c.Unmarshal(pub.Payload, &msg) == nil && msg.ID == "msg-2" {
				setMsg = msg
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond, "should find message with ID msg-2")
	assert.Equal(t, MsgTypeSet, setMsg.Type)
	assert.Equal(t, "msg-2", setMsg.ID)

	data, ok := setMsg.Data.(map[string]any)
	require.True(t, ok)
	state, ok := data["state"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 30.0, state["target"])
}

func TestSimpleHandler_Invoke(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockConn := connmock.NewMockConnector()
	mockConn.Start(ctx)
	c, _ := codec.New("json")
	shadowSvc := newTestShadowSvc()

	handler, err := NewSimpleHandler(mockConn, c, shadowSvc)
	require.NoError(t, err)
	defer handler.Stop()

	err = handler.Start(ctx)
	require.NoError(t, err)

	thingId := "test-device-invoke"
	_, err = shadowSvc.Create(ctx, thingId)
	require.NoError(t, err)

	mockConn.SetConnected(thingId, true)

	type invokeResult struct {
		value any
		err   error
	}
	resultCh := make(chan invokeResult, 1)
	go func() {
		result, invokeErr := handler.Invoke(ctx, thingId, "setTemperature", map[string]any{"value": 25}, 5*time.Second)
		resultCh <- invokeResult{value: result, err: invokeErr}
	}()

	require.Eventually(t, func() bool {
		return len(mockConn.PublishedMessages()) >= 1
	}, time.Second, 10*time.Millisecond)
	var callMsg ControlMessage
	err = c.Unmarshal(mockConn.PublishedMessages()[0].Payload, &callMsg)
	require.NoError(t, err)
	require.Equal(t, MsgTypeCall, callMsg.Type)

	replyPayload, err := c.Marshal(ControlMessage{
		Type: MsgTypeReply,
		ID:   callMsg.ID,
		Data: map[string]any{"result": "success"},
	})
	require.NoError(t, err)
	mockConn.SimulateMessage(TopicUp(thingId), replyPayload)

	var result any
	select {
	case invokeResult := <-resultCh:
		require.NoError(t, invokeResult.err)
		result = invokeResult.value
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for invoke result")
	}

	resultMap, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "success", resultMap["result"])
}

func TestSimpleHandler_ReplyBypassesWorkerPool(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockConn := connmock.NewMockConnector()
	require.NoError(t, mockConn.Start(ctx))
	c, err := codec.New("json")
	require.NoError(t, err)
	handler, err := NewSimpleHandler(mockConn, c, newTestShadowSvc())
	require.NoError(t, err)
	require.NoError(t, handler.Start(ctx))

	// A reply must still be delivered when no DB worker can accept work.
	handler.Stop()
	replyCh := make(chan ControlMessage, 1)
	handler.addPendingCall("thing-1", "call-1", replyCh)
	defer handler.removePendingCall("thing-1", "call-1")

	payload, err := c.Marshal(ControlMessage{
		Type: MsgTypeReply,
		ID:   "call-1",
		Data: map[string]any{"ok": true},
	})
	require.NoError(t, err)
	mockConn.SimulateMessage(TopicUp("thing-1"), payload)

	select {
	case reply := <-replyCh:
		require.Equal(t, map[string]any{"ok": true}, reply.Data)
	case <-time.After(time.Second):
		t.Fatal("reply was blocked by the worker pool")
	}
}
