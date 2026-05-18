package namespace

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"ruff.io/tio/config"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
	"ruff.io/tio/thing"
)

const rebuildPageSize = 1000

type Service struct {
	namespaces []config.Namespace
	publisher  connector.Publisher
	checker    connector.ConnectChecker

	mu         sync.RWMutex
	thingToNS  map[string]map[string]struct{}
	nsToThings map[string]map[string]struct{}
}

func NewService(namespaces []config.Namespace, conn connector.Connector) *Service {
	return &Service{
		namespaces: namespaces,
		publisher:  conn,
		checker:    conn,
		thingToNS:  make(map[string]map[string]struct{}),
		nsToThings: make(map[string]map[string]struct{}),
	}
}

func (s *Service) Link(ctx context.Context, shadowSvc shadow.Service) error {
	if len(s.namespaces) == 0 {
		return nil
	}
	if err := s.Rebuild(ctx, shadowSvc); err != nil {
		return err
	}

	thing.SubscribeLifecycle(func(ctx context.Context, event thing.LifecycleEvent) {
		switch event.Type {
		case thing.LifecycleThingCreated:
			s.ReconcileThing(ctx, event.Thing.Id, event.Tags)
		case thing.LifecycleThingDeleted:
			s.DeleteThing(ctx, event.Thing.Id)
		}
	})
	shadow.SubscribeTagsUpdate(func(thingId string, tags shadow.TagsValue) {
		s.ReconcileThing(context.Background(), thingId, tags)
	})
	shadowSvc.SubscribeUpdate(func(thingId string, notice shadow.StateUpdatedNotice) {
		if err := s.PublishShadowUpdated(context.Background(), thingId, notice); err != nil {
			slog.Error("Publish namespace shadow update", "thingId", thingId, "error", err)
		}
	})

	go s.forwardPresence(ctx)
	return nil
}

func (s *Service) Rebuild(ctx context.Context, shadowSvc shadow.CrudService) error {
	s.mu.Lock()
	s.thingToNS = make(map[string]map[string]struct{})
	s.nsToThings = make(map[string]map[string]struct{})
	s.mu.Unlock()

	for pageIndex := 1; ; pageIndex++ {
		page, err := shadowSvc.Query(ctx, model.PageQuery{PageIndex: pageIndex, PageSize: rebuildPageSize}, "")
		if err != nil {
			return err
		}
		if len(page.Content) == 0 {
			return nil
		}
		for _, item := range page.Content {
			ss, ok := item.(shadow.ShadowWithStatus)
			if !ok {
				continue
			}
			s.reconcileThingWithShadow(ctx, ss)
		}
		if len(page.Content) < rebuildPageSize {
			return nil
		}
	}
}

func (s *Service) ReconcileThing(ctx context.Context, thingId string, tags shadow.TagsValue) {
	next := setFromSlice(MatchNamespaces(s.namespaces, tags))
	old := s.setThingNamespaces(thingId, next)

	for ns := range next {
		if _, existed := old[ns]; existed {
			continue
		}
		s.publishCurrentPresence(ctx, ns, thingId)
	}
	for ns := range old {
		if _, exists := next[ns]; exists {
			continue
		}
		s.clearPresence(ns, thingId)
	}
}

func (s *Service) reconcileThingWithShadow(ctx context.Context, ss shadow.ShadowWithStatus) {
	next := setFromSlice(MatchNamespaces(s.namespaces, ss.Tags))
	old := s.setThingNamespaces(ss.ThingId, next)

	for ns := range next {
		if _, existed := old[ns]; existed {
			continue
		}
		s.publishPresenceFromShadow(ctx, ns, ss)
	}
	for ns := range old {
		if _, exists := next[ns]; exists {
			continue
		}
		s.clearPresence(ns, ss.ThingId)
	}
}

func (s *Service) DeleteThing(ctx context.Context, thingId string) {
	old := s.setThingNamespaces(thingId, nil)
	for ns := range old {
		s.clearPresence(ns, thingId)
	}
}

func (s *Service) PublishShadowUpdated(ctx context.Context, thingId string, notice shadow.StateUpdatedNotice) error {
	payload, err := json.Marshal(notice)
	if err != nil {
		return err
	}
	for _, ns := range s.NamespacesForThing(thingId) {
		if err := s.publisher.Publish(TopicShadowUpdated(ns, thingId), shadow.DefaultQos, false, payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) NamespacesForThing(thingId string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set := s.thingToNS[thingId]
	res := make([]string, 0, len(set))
	for ns := range set {
		res = append(res, ns)
	}
	return res
}

func (s *Service) forwardPresence(ctx context.Context) {
	ch := s.checker.OnConnect()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			s.PublishPresenceEvent(event)
		}
	}
}

func (s *Service) PublishPresenceEvent(event connector.PresenceEvent) {
	if event.ThingId == "" {
		return
	}
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("Marshal namespace presence event", "thingId", event.ThingId, "error", err)
		return
	}
	for _, ns := range s.NamespacesForThing(event.ThingId) {
		if err := s.publisher.Publish(TopicPresence(ns, event.ThingId), 1, true, payload); err != nil {
			slog.Error("Publish namespace presence retain", "ns", ns, "thingId", event.ThingId, "error", err)
		}
		if err := s.publisher.Publish(TopicPresenceEvent(ns, event.ThingId), 1, false, payload); err != nil {
			slog.Error("Publish namespace presence event", "ns", ns, "thingId", event.ThingId, "error", err)
		}
	}
}

func (s *Service) setThingNamespaces(thingId string, next map[string]struct{}) map[string]struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	old := s.thingToNS[thingId]
	if old == nil {
		old = map[string]struct{}{}
	}
	for ns := range old {
		if things := s.nsToThings[ns]; things != nil {
			delete(things, thingId)
		}
	}
	if len(next) == 0 {
		delete(s.thingToNS, thingId)
		return old
	}
	s.thingToNS[thingId] = next
	for ns := range next {
		if s.nsToThings[ns] == nil {
			s.nsToThings[ns] = make(map[string]struct{})
		}
		s.nsToThings[ns][thingId] = struct{}{}
	}
	return old
}

func (s *Service) publishCurrentPresence(ctx context.Context, ns, thingId string) {
	info, err := s.checker.ClientInfo(thingId)
	if err != nil {
		return
	}
	event := connector.PresenceEvent{
		ThingId:          thingId,
		ClientId:         info.ClientId,
		RemoteAddr:       info.RemoteAddr,
		DisconnectReason: info.DisconnectReason,
	}
	if info.Connected {
		event.EventType = connector.EventConnected
		if info.ConnectedAt != nil {
			event.Timestamp = info.ConnectedAt.UnixMilli()
		}
	} else {
		event.EventType = connector.EventDisconnected
		if info.DisconnectedAt != nil {
			event.Timestamp = info.DisconnectedAt.UnixMilli()
		}
	}
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("Marshal namespace current presence", "ns", ns, "thingId", thingId, "error", err)
		return
	}
	if err := s.publisher.Publish(TopicPresence(ns, thingId), 1, true, payload); err != nil {
		slog.Error("Publish namespace current presence", "ns", ns, "thingId", thingId, "error", err)
	}
}

func (s *Service) publishPresenceFromShadow(_ context.Context, ns string, ss shadow.ShadowWithStatus) {
	if ss.Connected == nil {
		return
	}
	event := connector.PresenceEvent{
		ThingId: ss.ThingId,
	}
	if ss.RemoteAddr != "" {
		event.RemoteAddr = ss.RemoteAddr
	}
	if *ss.Connected {
		event.EventType = connector.EventConnected
		if ss.ConnectedAt != nil {
			event.Timestamp = ss.ConnectedAt.UnixMilli()
		}
	} else {
		event.EventType = connector.EventDisconnected
		if ss.DisconnectedAt != nil {
			event.Timestamp = ss.DisconnectedAt.UnixMilli()
		}
	}
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("Marshal namespace presence from shadow", "ns", ns, "thingId", ss.ThingId, "error", err)
		return
	}
	if err := s.publisher.Publish(TopicPresence(ns, ss.ThingId), 1, true, payload); err != nil {
		slog.Error("Publish namespace presence from shadow", "ns", ns, "thingId", ss.ThingId, "error", err)
	}
}

func (s *Service) clearPresence(ns, thingId string) {
	if err := s.publisher.Publish(TopicPresence(ns, thingId), 1, true, nil); err != nil {
		slog.Error("Clear namespace presence retain", "ns", ns, "thingId", thingId, "error", err)
	}
}

func setFromSlice(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	res := make(map[string]struct{}, len(values))
	for _, v := range values {
		res[v] = struct{}{}
	}
	return res
}
