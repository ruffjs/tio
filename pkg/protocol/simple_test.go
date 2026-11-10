package protocol

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/connector"
	connmock "ruff.io/tio/connector/mock"
	dbmock "ruff.io/tio/db/mock"
	"ruff.io/tio/pkg/codec"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
	shadowMock "ruff.io/tio/shadow/mock"
	"ruff.io/tio/thing"
)

func newTestShadowSvc() shadow.Service {
	return newTestShadowSvcWithConfig(shadow.Config{})
}

func newTestShadowSvcWithConfig(cfg shadow.Config) shadow.Service {
	db := dbmock.NewSqliteConnTest()
	err := db.AutoMigrate(&thing.Entity{}, &shadow.Entity{}, &shadow.ConnStatusEntity{})
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Millisecond * 100)
	return shadow.NewTestSvc(shadow.NewShadowRepo(db), shadowMock.NewConnectivity(), cfg)
}

func TestTopicBuilders(t *testing.T) {
	assert.Equal(t, "tio/device1/up/shadow_get", TopicUp("device1", TypeShadowGet))
	assert.Equal(t, "tio/device1/down/shadow_desired", TopicDown("device1", TypeShadowDesired))
	assert.Equal(t, "tio/device1/event", TopicEvent("device1"))
	assert.Equal(t, "tio/device1/data", TopicData("device1"))
	assert.Equal(t, "tio/+/up/#", TopicAllUp())
}

func TestParseTopic(t *testing.T) {
	tests := []struct {
		name        string
		topic       string
		wantThingId string
		wantDir     string
		wantType    string
		wantErr     bool
	}{
		{"valid up", "tio/device1/up/shadow_get", "device1", LevelUp, TypeShadowGet, false},
		{"valid down", "tio/device1/down/shadow_desired", "device1", LevelDown, TypeShadowDesired, false},
		{"valid method req", "tio/device1/down/method_req", "device1", LevelDown, TypeMethodReq, false},
		{"valid method resp", "tio/device1/up/method_resp", "device1", LevelUp, TypeMethodResp, false},
		{"valid ntp req", "tio/device1/up/ntp_req", "device1", LevelUp, TypeNtpReq, false},
		{"valid event", "tio/device1/event", "device1", LevelEvent, "", false},
		{"valid data", "tio/device1/data", "device1", LevelData, "", false},
		{"invalid prefix", "mqtt/device1/up/shadow_get", "", "", "", true},
		{"invalid structure", "tio/device1", "", "", "", true},
		{"invalid direction", "tio/device1/sideways/shadow_get", "", "", "", true},
		{"wildcard thingId", "tio/+/up/shadow_get", "", "", "", true},
		{"empty thingId", "tio//up/shadow_get", "", "", "", true},
		{"empty type", "tio/device1/up/", "", "", "", true},
		{"event with type", "tio/device1/event/extra", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thingId, dir, typ, err := ParseTopic(tt.topic)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantThingId, thingId)
				assert.Equal(t, tt.wantDir, dir)
				assert.Equal(t, tt.wantType, typ)
			}
		})
	}
}

func TestParseTopicDir(t *testing.T) {
	tests := []struct {
		name        string
		topic       string
		wantThingId string
		wantDir     string
		wantErr     bool
	}{
		{"up with type", "tio/device1/up/shadow_get", "device1", "up", false},
		{"down with type", "tio/device1/down/shadow_desired", "device1", "down", false},
		{"event", "tio/device1/event", "device1", "event", false},
		{"data", "tio/device1/data", "device1", "data", false},
		{"custom single", "tio/device1/custom", "device1", "custom", false},
		{"custom deep path", "tio/device1/custom/sub/deep", "device1", "custom", false},
		{"up without type", "tio/device1/up", "device1", "up", false},
		{"invalid prefix", "mqtt/device1/up", "", "", true},
		{"no direction", "tio/device1", "", "", true},
		{"wildcard thingId", "tio/+/up/shadow_get", "", "", true},
		{"empty thingId", "tio//up/shadow_get", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thingId, dir, err := ParseTopicDir(tt.topic)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantThingId, thingId)
				assert.Equal(t, tt.wantDir, dir)
			}
		})
	}
}

func TestValidateShadowUpdate(t *testing.T) {
	tests := []struct {
		name    string
		req     ShadowUpdateReq
		wantErr bool
	}{
		{"valid state", ShadowUpdateReq{State: map[string]any{"color": "red"}}, false},
		{"empty state is valid", ShadowUpdateReq{State: map[string]any{}}, false},
		{"nil state", ShadowUpdateReq{State: nil}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateShadowUpdate(tt.req)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSimpleHandler_ShadowGet(t *testing.T) {
	ctx := t.Context()

	mockConn := connmock.NewMockConnector()
	mockConn.Start(ctx)
	c, _ := codec.New("json")
	shadowSvc := newTestShadowSvc()

	handler, err := NewSimpleHandler(mockConn, c, shadowSvc, nil)
	require.NoError(t, err)
	defer handler.Stop()

	err = handler.Start(ctx)
	require.NoError(t, err)

	thingId := "test-device-shadow-get"
	_, err = shadowSvc.Create(ctx, thingId)
	require.NoError(t, err)

	_, err = shadowSvc.SetDesired(ctx, thingId, shadow.StateReq{
		State: shadow.StateDR{
			Desired: map[string]any{"light": true},
		},
		Version: 1,
	})
	require.NoError(t, err)

	getPayload, _ := c.Marshal(map[string]any{})
	mockConn.SimulateMessage(TopicUp(thingId, TypeShadowGet), getPayload)

	require.Eventually(t, func() bool {
		for _, pub := range mockConn.PublishedMessages() {
			if pub.Topic == TopicDown(thingId, TypeShadowGetReply) {
				var reply ShadowGetReply
				if c.Unmarshal(pub.Payload, &reply) == nil && reply.Code == 200 {
					assert.Equal(t, int64(2), reply.Version)
					require.NotNil(t, reply.State)
					assert.Equal(t, true, reply.State.Desired["light"])
					return true
				}
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
}

func TestSimpleHandler_ShadowDesired(t *testing.T) {
	for _, encoding := range []string{"json", "cbor"} {
		t.Run(encoding, func(t *testing.T) {
			ctx := t.Context()
			conn := connmock.NewMockConnector()
			require.NoError(t, conn.Start(ctx))
			c, err := codec.New(encoding)
			require.NoError(t, err)
			svc := newTestShadowSvc()
			h, err := NewSimpleHandler(conn, c, svc, nil)
			require.NoError(t, err)
			defer h.Stop()
			require.NoError(t, h.Start(ctx))
			id := "desired-" + encoding
			_, err = svc.Create(ctx, id)
			require.NoError(t, err)

			update := func(desired bool, patch, want map[string]any) {
				t.Helper()
				before := len(conn.PublishedMessages())
				var updated shadow.Shadow
				var err error
				if desired {
					updated, err = svc.SetDesired(ctx, id, shadow.StateReq{State: shadow.StateDR{Desired: patch}})
				} else {
					updated, err = svc.SetReported(ctx, id, shadow.StateReq{State: shadow.StateDR{Reported: patch}})
				}
				require.NoError(t, err)
				messages := conn.PublishedMessages()[before:]
				require.Len(t, messages, len(want))
				seen := map[string]bool{}
				for _, msg := range messages {
					require.Equal(t, TopicDown(id, TypeShadowDesired), msg.Topic)
					require.Equal(t, "PublishReliable", msg.Method)
					var got ShadowDesired
					require.NoError(t, c.Unmarshal(msg.Payload, &got))
					require.Equal(t, updated.Version, got.Version)
					expected, exists := want[got.Name]
					require.True(t, exists)
					require.False(t, seen[got.Name])
					seen[got.Name] = true
					actualJSON, err := json.Marshal(got.Value)
					require.NoError(t, err)
					expectedJSON, err := json.Marshal(expected)
					require.NoError(t, err)
					require.JSONEq(t, string(expectedJSON), string(actualJSON))
					var fields map[string]any
					require.NoError(t, c.Unmarshal(msg.Payload, &fields))
					require.Len(t, fields, 3)
					require.Contains(t, fields, "value")
				}
			}
			baseline := map[string]any{"config": map[string]any{"a": 1, "b": 0}, "enabled": true, "levels": []any{1, 2}}
			update(false, baseline, nil)
			update(true, baseline, baseline) // desired changes notify even when reported matches.
			update(true, baseline, nil)
			update(false, map[string]any{"enabled": false}, nil)
			update(true, map[string]any{"config": map[string]any{"b": 2}}, map[string]any{"config": map[string]any{"a": 1, "b": 2}})
			update(true, map[string]any{"config": map[string]any{"a": nil}}, map[string]any{"config": map[string]any{"b": 2}})
			update(true, map[string]any{"config": nil, "enabled": false, "levels": []any{3}}, map[string]any{"config": nil, "enabled": false, "levels": []any{3}})
			update(false, map[string]any{"config": nil}, nil)
			update(true, map[string]any{"config": nil}, nil)
		})
	}
}

func TestSimpleHandler_ShadowUpdate(t *testing.T) {
	ctx := t.Context()

	mockConn := connmock.NewMockConnector()
	mockConn.Start(ctx)
	c, _ := codec.New("json")
	shadowSvc := newTestShadowSvc()

	handler, err := NewSimpleHandler(mockConn, c, shadowSvc, nil)
	require.NoError(t, err)
	defer handler.Stop()

	err = handler.Start(ctx)
	require.NoError(t, err)

	thingId := "test-device-shadow-update"
	_, err = shadowSvc.Create(ctx, thingId)
	require.NoError(t, err)

	updateMsg := ShadowUpdateReq{
		State: map[string]any{
			"temperature": 25.5,
		},
	}
	payload, _ := c.Marshal(updateMsg)
	mockConn.SimulateMessage(TopicUp(thingId, TypeShadowUpdate), payload)

	require.Eventually(t, func() bool {
		for _, pub := range mockConn.PublishedMessages() {
			if pub.Topic == TopicDown(thingId, TypeShadowUpdateReply) {
				var reply ShadowUpdateReply
				if c.Unmarshal(pub.Payload, &reply) == nil && reply.Code == 200 {
					assert.Equal(t, int64(2), reply.Version)
					return true
				}
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)

	ss, err := shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	assert.Equal(t, 25.5, ss.State.Reported["temperature"])
}

func TestSimpleHandler_ShadowUpdateVersionConflict(t *testing.T) {
	ctx := t.Context()

	mockConn := connmock.NewMockConnector()
	mockConn.Start(ctx)
	c, _ := codec.New("json")
	shadowSvc := newTestShadowSvc()

	handler, err := NewSimpleHandler(mockConn, c, shadowSvc, nil)
	require.NoError(t, err)
	defer handler.Stop()
	require.NoError(t, handler.Start(ctx))

	thingId := "test-device-version-conflict"
	_, err = shadowSvc.Create(ctx, thingId)
	require.NoError(t, err)

	_, err = shadowSvc.SetReported(ctx, thingId, shadow.StateReq{
		State: shadow.StateDR{Reported: map[string]any{"color": "red"}},
	})
	require.NoError(t, err)

	ss, _ := shadowSvc.Get(ctx, thingId)
	require.Equal(t, int64(2), ss.Version)

	updateMsg := ShadowUpdateReq{
		Version: 1,
		State:   map[string]any{"color": "blue"},
	}
	payload, _ := c.Marshal(updateMsg)
	mockConn.SimulateMessage(TopicUp(thingId, TypeShadowUpdate), payload)

	require.Eventually(t, func() bool {
		for _, pub := range mockConn.PublishedMessages() {
			if pub.Topic == TopicDown(thingId, TypeShadowUpdateReply) {
				var reply ShadowUpdateReply
				if c.Unmarshal(pub.Payload, &reply) == nil && reply.Code == 409 {
					assert.Equal(t, int64(2), reply.Version)
					return true
				}
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
}

func TestSimpleHandler_ShadowUpdateNoOp(t *testing.T) {
	ctx := t.Context()

	mockConn := connmock.NewMockConnector()
	mockConn.Start(ctx)
	c, _ := codec.New("json")
	shadowSvc := newTestShadowSvc()

	handler, err := NewSimpleHandler(mockConn, c, shadowSvc, nil)
	require.NoError(t, err)
	defer handler.Stop()

	err = handler.Start(ctx)
	require.NoError(t, err)

	thingId := "test-device-noop"
	_, err = shadowSvc.Create(ctx, thingId)
	require.NoError(t, err)

	_, err = shadowSvc.SetReported(ctx, thingId, shadow.StateReq{
		State: shadow.StateDR{Reported: map[string]any{"color": "red"}},
	})
	require.NoError(t, err)

	ss, _ := shadowSvc.Get(ctx, thingId)
	versionBefore := ss.Version

	updateMsg := ShadowUpdateReq{
		State: map[string]any{"color": "red"},
	}
	payload, _ := c.Marshal(updateMsg)
	mockConn.SimulateMessage(TopicUp(thingId, TypeShadowUpdate), payload)

	require.Eventually(t, func() bool {
		for _, pub := range mockConn.PublishedMessages() {
			if pub.Topic == TopicDown(thingId, TypeShadowUpdateReply) {
				var reply ShadowUpdateReply
				if c.Unmarshal(pub.Payload, &reply) == nil && reply.Code == 200 {
					assert.Equal(t, versionBefore, reply.Version)
					return true
				}
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)

	ss, err = shadowSvc.Get(ctx, thingId)
	require.NoError(t, err)
	assert.Equal(t, versionBefore, ss.Version, "no-op should not increment version")
}

func TestSimpleHandler_Invoke(t *testing.T) {
	ctx := t.Context()

	mockConn := connmock.NewMockConnector()
	mockConn.Start(ctx)
	c, _ := codec.New("json")
	shadowSvc := newTestShadowSvc()

	handler, err := NewSimpleHandler(mockConn, c, shadowSvc, nil)
	require.NoError(t, err)
	defer handler.Stop()

	err = handler.Start(ctx)
	require.NoError(t, err)

	thingId := "test-device-invoke"
	_, err = shadowSvc.Create(ctx, thingId)
	require.NoError(t, err)

	mockConn.SetConnected(thingId, true)

	type invokeResult struct {
		value SimpleInvokeResult
		err   error
	}
	resultCh := make(chan invokeResult, 1)
	go func() {
		result, invokeErr := handler.Invoke(ctx, thingId, "setTemperature", map[string]any{"value": 25}, 5*time.Second)
		resultCh <- invokeResult{value: result, err: invokeErr}
	}()

	require.Eventually(t, func() bool {
		for _, pub := range mockConn.PublishedMessages() {
			if pub.Topic == TopicDown(thingId, TypeMethodReq) {
				var req MethodReq
				if c.Unmarshal(pub.Payload, &req) == nil {
					assert.Equal(t, "setTemperature", req.Method)
					return true
				}
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)

	var req MethodReq
	for _, pub := range mockConn.PublishedMessages() {
		if pub.Topic == TopicDown(thingId, TypeMethodReq) {
			c.Unmarshal(pub.Payload, &req)
			break
		}
	}

	respPayload, _ := c.Marshal(MethodResp{
		ID:   req.ID,
		Code: 200,
		Data: map[string]any{"result": "success"},
	})
	mockConn.SimulateMessage(TopicUp(thingId, TypeMethodResp), respPayload)

	var result SimpleInvokeResult
	select {
	case r := <-resultCh:
		require.NoError(t, r.err)
		result = r.value
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for invoke result")
	}

	assert.Equal(t, 200, result.Code)
	resultMap, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "success", resultMap["result"])
}

func TestSimpleHandler_NtpReq(t *testing.T) {
	ctx := t.Context()

	mockConn := connmock.NewMockConnector()
	mockConn.Start(ctx)
	c, _ := codec.New("json")
	shadowSvc := newTestShadowSvc()

	handler, err := NewSimpleHandler(mockConn, c, shadowSvc, nil)
	require.NoError(t, err)
	defer handler.Stop()

	err = handler.Start(ctx)
	require.NoError(t, err)

	thingId := "test-device-ntp"
	_, err = shadowSvc.Create(ctx, thingId)
	require.NoError(t, err)

	clientSendTime := time.Now().UnixMilli()
	ntpReq := NtpReq{ClientSendTime: clientSendTime}
	payload, _ := c.Marshal(ntpReq)
	mockConn.SimulateMessage(TopicUp(thingId, TypeNtpReq), payload)

	require.Eventually(t, func() bool {
		for _, pub := range mockConn.PublishedMessages() {
			if pub.Topic == TopicDown(thingId, TypeNtpResp) {
				var resp NtpResp
				if c.Unmarshal(pub.Payload, &resp) == nil && resp.Code == 200 {
					assert.Equal(t, clientSendTime, resp.ClientSendTime)
					assert.True(t, resp.ServerRecvTime > 0)
					assert.True(t, resp.ServerSendTime >= resp.ServerRecvTime)
					return true
				}
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
}

type staticProfiles map[string]connector.DeviceProfile

func (p staticProfiles) GetDeviceProfile(_ context.Context, thingId string) (connector.DeviceProfile, error) {
	if v, ok := p[thingId]; ok {
		return v, nil
	}
	return connector.DeviceProfile{}, model.ErrNotFound
}

func TestSimpleHandler_UsesThingProfileCodec(t *testing.T) {
	ctx := t.Context()
	mockConn := connmock.NewMockConnector()
	require.NoError(t, mockConn.Start(ctx))
	jsonCodec, err := codec.New("json")
	require.NoError(t, err)
	cborCodec, err := codec.New("cbor")
	require.NoError(t, err)
	profiles := staticProfiles{
		"thing-cbor":   {MessageProtocol: "simple", PayloadEncoding: "cbor"},
		"thing-legacy": {MessageProtocol: "legacy", PayloadEncoding: "json"},
	}
	handler, err := NewSimpleHandler(mockConn, jsonCodec, newTestShadowSvc(), profiles)
	require.NoError(t, err)
	require.NoError(t, handler.Start(ctx))
	defer handler.Stop()

	payload, err := cborCodec.Marshal(NtpReq{ClientSendTime: 123})
	require.NoError(t, err)
	mockConn.SimulateMessage(TopicUp("thing-cbor", TypeNtpReq), payload)
	require.Eventually(t, func() bool {
		for _, pub := range mockConn.PublishedMessages() {
			if pub.Topic != TopicDown("thing-cbor", TypeNtpResp) {
				continue
			}
			var resp NtpResp
			return cborCodec.Unmarshal(pub.Payload, &resp) == nil && resp.Code == 200
		}
		return false
	}, time.Second, 10*time.Millisecond)

	// A simple topic message from a legacy thing is ignored.
	jsonPayload, err := jsonCodec.Marshal(NtpReq{ClientSendTime: 456})
	require.NoError(t, err)
	before := len(mockConn.PublishedMessages())
	mockConn.SimulateMessage(TopicUp("thing-legacy", TypeNtpReq), jsonPayload)
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, before, len(mockConn.PublishedMessages()))
}

func TestSimpleHandler_MethodRespBypassesWorkerPool(t *testing.T) {
	ctx := t.Context()

	mockConn := connmock.NewMockConnector()
	require.NoError(t, mockConn.Start(ctx))
	c, err := codec.New("json")
	require.NoError(t, err)
	handler, err := NewSimpleHandler(mockConn, c, newTestShadowSvc(), nil)
	require.NoError(t, err)
	require.NoError(t, handler.Start(ctx))

	handler.Stop()
	replyCh := make(chan MethodResp, 1)
	handler.addPendingCall("thing-1", "call-1", c, replyCh)
	defer handler.removePendingCall("thing-1", "call-1")

	payload, err := c.Marshal(MethodResp{
		ID:   "call-1",
		Code: 200,
		Data: map[string]any{"ok": true},
	})
	require.NoError(t, err)
	mockConn.SimulateMessage(TopicUp("thing-1", TypeMethodResp), payload)

	select {
	case resp := <-replyCh:
		assert.Equal(t, 200, resp.Code)
		data, ok := resp.Data.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, data["ok"])
	case <-time.After(time.Second):
		t.Fatal("method resp was blocked by the worker pool")
	}
}
