package emqx

import (
	"context"
	"encoding/json"
	"ruff.io/tio/connector"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	mq "ruff.io/tio/connector/mqtt/client"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/shadow"
)

// For synchronize client connections state

// presence topics have retained data,
// so we need to synchronize the newest presence events to the topics

const (
	maxWaitReceivePresenceTime = time.Second * 20
)

type clientState struct {
	connected      bool
	connectedAt    int64
	disconnectedAt int64
}

type presenceSyncImpl struct {
	presenceEvents map[string]*clientState // things presence state
	sync.RWMutex
}

var syncInstance = &presenceSyncImpl{presenceEvents: make(map[string]*clientState)}

// syncPresence Get retained presence events and compare them with the newest client state
// if retained presence events are expired, then republish the new one
func syncPresence(ctx context.Context, mqCl mq.Client,
	getClient func(id string) (client, bool),
	getAllClients func() map[string]client,
) {
	startTime := time.Now()
	// wait received presence events
	time.Sleep(maxWaitReceivePresenceTime)
	if ctx.Err() != nil {
		log.Debug("Give up sync presence cause context done")
		return
	}
	log.Infof("Starting presence sync, presence events length: %d", len(syncInstance.presenceEvents))
	syncInstance.diffAndPub(ctx, startTime, getClient, mqCl)
	clients := getAllClients()
	syncInstance.pubNewEvents(ctx, startTime, mqCl, clients, getClient)
	log.Infof("Synced client connect state to presence")
}

func receivePresence(ctx context.Context, mqCl mq.Client) {
	topic := connector.TopicPresenceAll
	err := mqCl.Subscribe(ctx, connector.TopicPresenceAll, 1, func(c mqtt.Client, m mqtt.Message) {
		var e connector.Event
		// log.Debugf("Got presence event: %s %s", m.Topic(), m.Payload())
		err := json.Unmarshal(m.Payload(), &e)
		if err != nil {
			log.Errorf("Unmarshal presence event %s error: %v", m.Payload(), err)
			return
		}
		thingId, err := shadow.GetThingIdFromTopic(m.Topic())
		if err != nil {
			log.Errorf("Can't get thing id from topic %s: %v", m.Topic(), err)
			return
		}
		syncInstance.updateLocalPresence(thingId, e)
	})

	if err != nil {
		log.Fatalf("Subscribe to topic %s error: %v", topic, err)
	}
}

func (s *presenceSyncImpl) updateLocalPresence(id string, e connector.Event) {
	s.RLock()
	defer s.RUnlock()
	if old, ok := s.presenceEvents[id]; ok {
		if e.Timestamp < old.connectedAt || e.Timestamp < old.disconnectedAt {
			return
		}
		old.connected = e.EventType == connector.EventConnected
		if old.connected {
			old.connectedAt = e.Timestamp
		} else {
			old.disconnectedAt = e.Timestamp
		}
	} else {
		st := &clientState{
			connected: e.EventType == connector.EventConnected,
		}
		if st.connected {
			st.connectedAt = e.Timestamp
		} else {
			st.disconnectedAt = e.Timestamp
		}
		s.presenceEvents[id] = st
	}
}

func (s *presenceSyncImpl) diffAndPub(
	ctx context.Context,
	startTime time.Time,
	getClient func(id string) (client, bool),
	mqCl mq.Client) {
	for thingId, c := range s.presenceEvents {
		var e connector.Event
		if c.connected {
			e = connector.Event{EventType: connector.EventConnected, Timestamp: c.connectedAt}
		} else {
			e = connector.Event{EventType: connector.EventDisconnected, Timestamp: c.disconnectedAt}
		}
		s.diffAndPubForThing(ctx, startTime, thingId, e, getClient, mqCl)
	}
}

func (s *presenceSyncImpl) diffAndPubForThing(
	ctx context.Context,
	startTime time.Time,
	thingId string,
	retainedEvent connector.Event,
	getClient func(id string) (client, bool),
	mqCl mq.Client,
) {
	if n, ok := getClient(thingId); ok {
		// time and connect state diff
		if n.info.Connected && n.info.ConnectedAt.UnixMilli() > retainedEvent.Timestamp {
			evt := connector.Event{
				EventType:  connector.EventConnected,
				Timestamp:  n.info.ConnectedAt.UnixMilli(),
				ThingId:    n.info.Username,
				RemoteAddr: n.info.IpAddress,
			}
			notifyEvent(ctx, mqCl, thingId, connector.TopicPresence(thingId), evt)
			log.Debugf("Sync presence: republish thing %q event: %#v", thingId, evt)
		} else if !n.info.Connected && n.info.DisconnectedAt.UnixMilli() > retainedEvent.Timestamp {
			evt := connector.Event{
				EventType:  connector.EventDisconnected,
				Timestamp:  n.info.DisconnectedAt.UnixMilli(),
				ThingId:    n.info.Username,
				RemoteAddr: n.info.IpAddress,
			}
			notifyEvent(ctx, mqCl, thingId, connector.TopicPresence(thingId), evt)
			log.Debugf("Sync presence: republish thing %q event: %#v", thingId, evt)
		}
	} else {
		if retainedEvent.EventType == connector.EventDisconnected {
			// ignore —— cause the last retained presence message is disconnected
			return
		}

		// no client connected now means that the client has been disconnected at some time before
		evt := connector.Event{
			EventType:        connector.EventDisconnected,
			Timestamp:        startTime.UnixMilli(),
			ThingId:          thingId,
			DisconnectReason: "disconnected during tio downtime",
		}
		log.Debugf(
			"Sync presence: to publish thing %q disconnected: %#v, it is disconnected when server is down",
			thingId, evt)
		notifyEvent(ctx, mqCl, thingId, connector.TopicPresence(thingId), evt)
	}
}

// pubNewEvents publishes events that are not yet retained in presence topics
func (s *presenceSyncImpl) pubNewEvents(
	ctx context.Context,
	startTime time.Time,
	mqCl mq.Client,
	clients map[string]client,
	getClient func(id string) (client, bool)) {
	for _, c := range clients {
		if _, ok := s.presenceEvents[c.info.ClientId]; !ok {
			if ctx.Err() != nil {
				return
			}
			// get it again because it may have been updated.
			if n, ok := getClient(c.info.ClientId); ok {
				var evt connector.Event
				if n.info.Connected {
					evt = connector.Event{
						EventType:  connector.EventConnected,
						Timestamp:  n.info.ConnectedAt.UnixMilli(),
						ThingId:    n.info.Username,
						RemoteAddr: n.info.IpAddress,
					}
				} else {
					evt = connector.Event{
						EventType:  connector.EventDisconnected,
						Timestamp:  n.info.DisconnectedAt.UnixMilli(),
						ThingId:    n.info.Username,
						RemoteAddr: n.info.IpAddress,
					}
				}
				log.Debugf("Sync presence: to publish thing %q new event: %#v", c.info.ClientId, evt)
				notifyEvent(ctx, mqCl, c.info.ClientId, connector.TopicPresence(c.info.ClientId), evt)
			}
		}
	}
}
