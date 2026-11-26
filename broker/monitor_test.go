package broker

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeServer struct {
	varz     *server.Varz
	varzErr  error
	conns    []*server.ConnInfo
	connzErr error
	lastOpts *server.ConnzOptions
}

func (f *fakeServer) Varz(*server.VarzOptions) (*server.Varz, error) {
	if f.varzErr != nil {
		return nil, f.varzErr
	}
	return f.varz, nil
}

func (f *fakeServer) Connz(o *server.ConnzOptions) (*server.Connz, error) {
	if f.connzErr != nil {
		return nil, f.connzErr
	}
	f.lastOpts = o
	conns := make([]*server.ConnInfo, 0, len(f.conns))
	for _, c := range f.conns {
		if o.CID != 0 && c.Cid != o.CID {
			continue
		}
		if o.MQTTClient != "" && c.MQTTClient != o.MQTTClient {
			continue
		}
		conns = append(conns, c)
	}
	total := len(conns)
	offset := min(o.Offset, total)
	limit := o.Limit
	if limit <= 0 {
		limit = total
	}
	end := min(offset+limit, total)
	return &server.Connz{Conns: conns[offset:end], Total: total, Offset: offset, Limit: limit}, nil
}

type fakeStreams struct {
	info *nats.StreamInfo
	err  error
}

func (f *fakeStreams) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) {
	return f.info, f.err
}

func mqttConn(cid uint64, clientID string, subs uint32) *server.ConnInfo {
	return &server.ConnInfo{
		Cid:        cid,
		Type:       "mqtt",
		MQTTClient: clientID,
		NumSubs:    subs,
		Start:      time.Unix(1000, 0),
		InMsgs:     10,
	}
}

func TestSampleAndOverview(t *testing.T) {
	srv := &fakeServer{
		varz: &server.Varz{Name: "node-1", Version: "2.15.3", Connections: 2},
	}
	m := New(srv, nil, Options{Interval: 15 * time.Second, History: time.Hour})
	m.sample()
	require.Len(t, m.samples, 1)

	srv.varz.InClientMsgs, srv.varz.OutClientMsgs = 100, 50
	srv.varz.InClientBytes, srv.varz.OutClientBytes = 1000, 500
	m.mu.Lock()
	m.samples[0].Time = time.Now().Add(-15 * time.Second)
	m.mu.Unlock()
	m.sample()

	ov := m.Overview()
	assert.Equal(t, "node-1", ov.Node)
	assert.Equal(t, "2.15.3", ov.Version)
	assert.Equal(t, 2, ov.Connections)
	require.Len(t, ov.Series, 1)
	require.NotNil(t, ov.Series[0].MsgRate)
	require.NotNil(t, ov.Series[0].ByteRate)
	assert.InDelta(t, 10.0, *ov.Series[0].MsgRate, 0.5)   // (100+50)/15
	assert.InDelta(t, 100.0, *ov.Series[0].ByteRate, 0.5) // (1000+500)/15
}

func TestBuildSeriesLeavesGaps(t *testing.T) {
	base := time.Unix(1000, 0)
	samples := []sample{
		{Time: base, InMsgs: 0, OutMsgs: 0, InBytes: 0, OutBytes: 0},
		{Time: base.Add(15 * time.Second), InMsgs: 30, OutMsgs: 30, InBytes: 300, OutBytes: 300},
		{Time: base.Add(45 * time.Second), InMsgs: 90, OutMsgs: 90, InBytes: 900, OutBytes: 900}, // missing sample
		{Time: base.Add(60 * time.Second), InMsgs: 1, OutMsgs: 1, InBytes: 1, OutBytes: 1},       // counter reset
		{Time: base.Add(75 * time.Second), InMsgs: 16, OutMsgs: 16, InBytes: 16, OutBytes: 16},
	}

	series := buildSeries(samples, 15*time.Second)
	require.Len(t, series, 4)
	assert.NotNil(t, series[0].MsgRate)
	assert.InDelta(t, 4.0, *series[0].MsgRate, 0.001)
	assert.Nil(t, series[1].MsgRate, "missing sample must be a gap")
	assert.Nil(t, series[2].MsgRate, "counter reset must be a gap")
	require.NotNil(t, series[3].MsgRate)
	assert.InDelta(t, 2.0, *series[3].MsgRate, 0.001)
}

func TestConnectionsUsesNATSPagination(t *testing.T) {
	conns := make([]*server.ConnInfo, 100)
	for i := range conns {
		conns[i] = mqttConn(uint64(i+1), "dev", 0)
	}
	srv := &fakeServer{conns: conns}
	m := New(srv, nil, Options{})

	page, err := m.Connections(ConnQuery{Page: 3, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 100, page.Total)
	assert.Equal(t, 3, page.Page)
	assert.Equal(t, 10, page.PageSize)
	require.Len(t, page.Items, 10)
	assert.Equal(t, uint64(21), page.Items[0].CID)
	assert.Equal(t, uint64(30), page.Items[9].CID)

	require.NotNil(t, srv.lastOpts)
	assert.Equal(t, 20, srv.lastOpts.Offset)
	assert.Equal(t, 10, srv.lastOpts.Limit)
	assert.Equal(t, server.BySubs, srv.lastOpts.Sort)
}

func TestConnectionsSortPassthrough(t *testing.T) {
	for _, field := range []string{"cid", "start", "subs", "pending", "msgs_to", "msgs_from", "bytes_to", "bytes_from", "last", "idle", "uptime", "rtt"} {
		t.Run(field, func(t *testing.T) {
			srv := &fakeServer{}
			m := New(srv, nil, Options{})
			_, err := m.Connections(ConnQuery{Sort: field})
			require.NoError(t, err)
			require.NotNil(t, srv.lastOpts)
			assert.Equal(t, server.SortOpt(field), srv.lastOpts.Sort)
		})
	}

	for _, field := range []string{"", "bogus", "stop", "reason"} {
		t.Run("fallback_"+field, func(t *testing.T) {
			srv := &fakeServer{}
			m := New(srv, nil, Options{})
			_, err := m.Connections(ConnQuery{Sort: field})
			require.NoError(t, err)
			assert.Equal(t, server.BySubs, srv.lastOpts.Sort)
		})
	}
}

func TestConnectionsIncludesAllConnectionTypes(t *testing.T) {
	srv := &fakeServer{conns: []*server.ConnInfo{
		mqttConn(1, "dev-1", 0),
		{Cid: 2, Type: "nats", Name: "tio-app-internal", NumSubs: 3, IP: "10.0.0.5", Port: 5000, InBytes: 100, OutBytes: 200},
	}}
	m := New(srv, nil, Options{})

	page, err := m.Connections(ConnQuery{PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 2, page.Total)
	require.Len(t, page.Items, 2)
	assert.Equal(t, "mqtt", page.Items[0].Type)
	assert.Equal(t, "dev-1", page.Items[0].ClientID)
	assert.Equal(t, "nats", page.Items[1].Type)
	assert.Equal(t, "tio-app-internal", page.Items[1].Name)
	assert.Equal(t, "10.0.0.5", page.Items[1].IP)
	assert.Equal(t, 5000, page.Items[1].Port)
	assert.Equal(t, int64(100), page.Items[1].InBytes)
	assert.Equal(t, int64(200), page.Items[1].OutBytes)
}

func TestConnectionsExactClientID(t *testing.T) {
	srv := &fakeServer{conns: []*server.ConnInfo{
		mqttConn(1, "alpha", 0),
		mqttConn(2, "bravo", 5),
	}}
	m := New(srv, nil, Options{})

	page, err := m.Connections(ConnQuery{ClientID: "bravo"})
	require.NoError(t, err)
	assert.Equal(t, 1, page.Total)
	assert.Equal(t, "bravo", page.Items[0].ClientID)
	require.NotNil(t, srv.lastOpts)
	assert.Equal(t, "bravo", srv.lastOpts.MQTTClient)
}

func TestConnectionDetail(t *testing.T) {
	srv := &fakeServer{conns: []*server.ConnInfo{{
		Cid:        9,
		Type:       "mqtt",
		MQTTClient: "dev-9",
		NumSubs:    3,
		IP:         "39.144.1.2",
		Port:       4567,
		Subs: []string{
			"$iothub.things.dev-9.methods.*.req",
			"$iothub.things.dev-9.shadows.name.default.update.delta",
			"$MQTT.sub.abc",
		},
		InBytes:  100,
		OutBytes: 200,
		Pending:  5,
		Idle:     "1s",
		Uptime:   "2s",
		RTT:      "3ms",
	}}}
	m := New(srv, nil, Options{})

	detail, err := m.Connection(9)
	require.NoError(t, err)
	assert.Equal(t, "dev-9", detail.ClientID)
	assert.Equal(t, uint32(3), detail.SubscriptionCount)
	assert.Equal(t, "39.144.1.2", detail.IP)
	assert.Equal(t, 4567, detail.Port)
	assert.Equal(t, int64(100), detail.InBytes)
	assert.Equal(t, int64(200), detail.OutBytes)
	assert.Equal(t, 5, detail.Pending)
	assert.Equal(t, "3ms", detail.RTT)
	assert.ElementsMatch(t, []string{
		"$iothub.things.dev-9.methods.*.req",
		"$iothub.things.dev-9.shadows.name.default.update.delta",
	}, detail.Subscriptions)
	assert.Equal(t, []string{"$MQTT.sub.abc"}, detail.InternalSubscriptions)

	_, err = m.Connection(10)
	assert.ErrorIs(t, err, ErrConnectionNotFound)
}

func TestConsumerUsage(t *testing.T) {
	ok := New(&fakeServer{}, &fakeStreams{info: &nats.StreamInfo{
		Config: nats.StreamConfig{MaxConsumers: 100},
		State:  nats.StreamState{Consumers: 7},
	}}, Options{}).consumerUsage()
	assert.Equal(t, ConsumerUsage{Stream: StreamName, State: "ok", Usage: 7, Limit: 100, LimitState: "limited"}, ok)

	inherited := New(&fakeServer{}, &fakeStreams{info: &nats.StreamInfo{
		Config: nats.StreamConfig{MaxConsumers: -1},
	}}, Options{ServerDefaultMaxConsumers: 80000}).consumerUsage()
	assert.Equal(t, 80000, inherited.Limit)
	assert.Equal(t, "limited", inherited.LimitState)

	unlimited := New(&fakeServer{}, &fakeStreams{info: &nats.StreamInfo{}}, Options{}).consumerUsage()
	assert.Equal(t, -1, unlimited.Limit)
	assert.Equal(t, "unlimited", unlimited.LimitState)

	missing := New(&fakeServer{}, &fakeStreams{err: nats.ErrStreamNotFound}, Options{}).consumerUsage()
	assert.Equal(t, "not_created", missing.State)

	unknown := New(&fakeServer{}, &fakeStreams{err: assert.AnError}, Options{}).consumerUsage()
	assert.Equal(t, "unknown", unknown.State)
	assert.Equal(t, "unknown", unknown.LimitState)
}
