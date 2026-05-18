package namespace_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/config"
	"ruff.io/tio/connector"
	"ruff.io/tio/namespace"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
)

type publishCall struct {
	topic    string
	retained bool
	payload  []byte
}

type fakeConnector struct {
	published       []publishCall
	clients         map[string]connector.ClientInfo
	presence        chan connector.PresenceEvent
	clientInfoCalls int
}

func newFakeConnector() *fakeConnector {
	return &fakeConnector{
		clients:  make(map[string]connector.ClientInfo),
		presence: make(chan connector.PresenceEvent),
	}
}

func (f *fakeConnector) Publish(topic string, qos byte, retained bool, payload []byte) error {
	f.published = append(f.published, publishCall{topic: topic, retained: retained, payload: payload})
	return nil
}

func (f *fakeConnector) Subscribe(context.Context, string, byte, func(connector.Message)) error {
	return nil
}
func (f *fakeConnector) Start(context.Context) error { return nil }
func (f *fakeConnector) Close(string) error          { return nil }
func (f *fakeConnector) Remove(string) error         { return nil }
func (f *fakeConnector) IsConnected(thingId string) (bool, error) {
	return f.clients[thingId].Connected, nil
}
func (f *fakeConnector) OnConnect() <-chan connector.PresenceEvent { return f.presence }
func (f *fakeConnector) ClientInfo(thingId string) (connector.ClientInfo, error) {
	f.clientInfoCalls++
	info, ok := f.clients[thingId]
	if !ok {
		return connector.ClientInfo{}, fmt.Errorf("not found")
	}
	return info, nil
}
func (f *fakeConnector) AllClientInfo() ([]connector.ClientInfo, error) { return nil, nil }

type fakeShadowService struct {
	items []shadow.ShadowWithStatus
}

func (f fakeShadowService) Init(context.Context) {}
func (f fakeShadowService) Create(context.Context, string) (shadow.Shadow, error) {
	return shadow.Shadow{}, nil
}
func (f fakeShadowService) Delete(context.Context, string) error { return nil }
func (f fakeShadowService) Query(_ context.Context, pq model.PageQuery, _ string) (shadow.Page, error) {
	if pq.PageIndex < 1 || pq.PageSize <= 0 {
		return shadow.Page{}, nil
	}
	start := (pq.PageIndex - 1) * pq.PageSize
	if start >= len(f.items) {
		return shadow.Page{Total: int64(len(f.items)), Content: []any{}}, nil
	}
	end := start + pq.PageSize
	if end > len(f.items) {
		end = len(f.items)
	}
	content := make([]any, 0, end-start)
	for _, item := range f.items[start:end] {
		content = append(content, item)
	}
	return shadow.Page{Total: int64(len(f.items)), Content: content}, nil
}
func (f fakeShadowService) Get(context.Context, string) (shadow.ShadowWithStatus, error) {
	return shadow.ShadowWithStatus{}, nil
}
func (f fakeShadowService) SetDesired(context.Context, string, shadow.StateReq) (shadow.Shadow, error) {
	return shadow.Shadow{}, nil
}
func (f fakeShadowService) SetReported(context.Context, string, shadow.StateReq) (shadow.Shadow, error) {
	return shadow.Shadow{}, nil
}
func (f fakeShadowService) SubscribeUpdate(shadow.StateUpdateSubscribe) {}
func (f fakeShadowService) SubscribeDelta(shadow.StateDeltaSubscribe)   {}
func (f fakeShadowService) SubAccepted(shadow.StateAcceptedSubscribe)   {}
func (f fakeShadowService) SubRejected(shadow.StateRejectedSubscribe)   {}
func (f fakeShadowService) SetTag(context.Context, string, shadow.TagsReq) error {
	return nil
}
func (f fakeShadowService) GetFromCache(string) (shadow.ShadowWithStatus, bool) {
	return shadow.ShadowWithStatus{}, false
}
func (f fakeShadowService) NotifyCreated(string, shadow.ShadowWithEnable) {}
func (f fakeShadowService) NotifyDeleted(string)                          {}
func (f fakeShadowService) NotifyUpdate(string, bool)                     {}

func TestServiceReconcileThingPublishesAndClearsPresence(t *testing.T) {
	conn := newFakeConnector()
	now := time.Now()
	conn.clients["thing-1"] = connector.ClientInfo{
		ClientId:    "thing-1",
		Connected:   true,
		ConnectedAt: &now,
	}
	svc := namespace.NewService([]config.Namespace{
		{Name: "biz-a", Tags: map[string]string{"ns": "a"}},
		{Name: "biz-b", Tags: map[string]string{"ns": "b"}},
	}, conn)

	svc.ReconcileThing(context.Background(), "thing-1", shadow.TagsValue{"ns": "a"})
	require.Len(t, conn.published, 1)
	require.Equal(t, namespace.TopicPresence("biz-a", "thing-1"), conn.published[0].topic)
	require.True(t, conn.published[0].retained)
	require.NotNil(t, conn.published[0].payload)

	svc.ReconcileThing(context.Background(), "thing-1", shadow.TagsValue{"ns": "b"})
	require.Len(t, conn.published, 3)
	require.Equal(t, namespace.TopicPresence("biz-b", "thing-1"), conn.published[1].topic)
	require.Equal(t, namespace.TopicPresence("biz-a", "thing-1"), conn.published[2].topic)
	require.Nil(t, conn.published[2].payload)
}

func TestServicePublishPresenceEventUsesThingNamespaces(t *testing.T) {
	conn := newFakeConnector()
	svc := namespace.NewService([]config.Namespace{
		{Name: "biz", Tags: map[string]string{"ns": "biz"}},
	}, conn)
	svc.ReconcileThing(context.Background(), "thing-1", shadow.TagsValue{"ns": "biz"})
	conn.published = nil

	svc.PublishPresenceEvent(connector.PresenceEvent{
		ThingId:   "thing-1",
		ClientId:  "thing-1",
		EventType: connector.EventConnected,
		Timestamp: time.Now().UnixMilli(),
	})

	require.Len(t, conn.published, 2)
	require.Equal(t, namespace.TopicPresence("biz", "thing-1"), conn.published[0].topic)
	require.True(t, conn.published[0].retained)
	require.Equal(t, namespace.TopicPresenceEvent("biz", "thing-1"), conn.published[1].topic)
	require.False(t, conn.published[1].retained)
}

func TestServiceRebuildPaginatesAndDoesNotCallClientInfo(t *testing.T) {
	conn := newFakeConnector()
	svc := namespace.NewService([]config.Namespace{
		{Name: "biz", Tags: map[string]string{"ns": "biz"}},
	}, conn)

	items := make([]shadow.ShadowWithStatus, 0, 1001)
	now := time.Now()
	disconnectedAt := now.Add(-time.Minute)
	for i := 0; i < 1000; i++ {
		items = append(items, shadow.ShadowWithStatus{
			Shadow:         shadow.Shadow{ThingId: fmt.Sprintf("thing-%d", i), Tags: shadow.TagsValue{"ns": "biz"}},
			Connected:      model.Ref(true),
			ConnectedAt:    &now,
			DisconnectedAt: nil,
			RemoteAddr:     "127.0.0.1",
		})
	}
	items = append(items, shadow.ShadowWithStatus{
		Shadow:         shadow.Shadow{ThingId: "thing-1000", Tags: shadow.TagsValue{"ns": "biz"}},
		Connected:      model.Ref(false),
		DisconnectedAt: &disconnectedAt,
	})

	err := svc.Rebuild(context.Background(), fakeShadowService{items: items})
	require.NoError(t, err)
	require.Equal(t, 0, conn.clientInfoCalls)
	require.Len(t, svc.NamespacesForThing("thing-0"), 1)
	require.Len(t, svc.NamespacesForThing("thing-1000"), 1)
	require.Len(t, conn.published, len(items))
	require.Equal(t, namespace.TopicPresence("biz", "thing-1000"), conn.published[len(conn.published)-1].topic)
	require.True(t, conn.published[len(conn.published)-1].retained)
	require.NotNil(t, conn.published[len(conn.published)-1].payload)
}
