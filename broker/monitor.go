package broker

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

const (
	// StreamName is the JetStream stream the embedded MQTT broker uses.
	StreamName = "$MQTT_msgs"

	DefaultInterval = 15 * time.Second
	DefaultHistory  = time.Hour

	DefaultPageSize = 20
	MaxPageSize     = 200
)

var ErrConnectionNotFound = errors.New("connection not found")

// sortable lists the NATS connection sorts for open connections. ByStop and ByReason
// are excluded because they apply to closed connections and would dereference nil.
var sortable = map[server.SortOpt]struct{}{
	server.ByCid:      {},
	server.ByStart:    {},
	server.BySubs:     {},
	server.ByPending:  {},
	server.ByOutMsgs:  {},
	server.ByInMsgs:   {},
	server.ByOutBytes: {},
	server.ByInBytes:  {},
	server.ByLast:     {},
	server.ByIdle:     {},
	server.ByUptime:   {},
	server.ByRTT:      {},
}

// ValidSort reports whether field is a supported NATS Connz sort for open connections.
func ValidSort(field string) bool {
	_, ok := sortable[server.SortOpt(field)]
	return ok
}

func connzSort(field string) server.SortOpt {
	if ValidSort(field) {
		return server.SortOpt(field)
	}
	return server.BySubs
}

// Server is the subset of the embedded NATS server used for sampling.
type Server interface {
	Varz(opts *server.VarzOptions) (*server.Varz, error)
	Connz(opts *server.ConnzOptions) (*server.Connz, error)
}

// StreamInfoer reads JetStream stream state, usually the connector's JetStream context.
type StreamInfoer interface {
	StreamInfo(stream string, opts ...nats.JSOpt) (*nats.StreamInfo, error)
}

type Options struct {
	Interval time.Duration
	History  time.Duration
	// ServerDefaultMaxConsumers is the effective server-level consumer limit (<=0 means unlimited).
	ServerDefaultMaxConsumers int
}

type sample struct {
	Time        time.Time
	InMsgs      int64
	OutMsgs     int64
	InBytes     int64
	OutBytes    int64
	Connections int
}

type Monitor struct {
	srv     Server
	streams StreamInfoer
	opt     Options

	mu      sync.RWMutex
	samples []sample
	node    string
	version string
}

func New(srv Server, streams StreamInfoer, opt Options) *Monitor {
	if opt.Interval <= 0 {
		opt.Interval = DefaultInterval
	}
	if opt.History <= 0 {
		opt.History = DefaultHistory
	}
	return &Monitor{srv: srv, streams: streams, opt: opt}
}

// Start samples immediately and then on every interval until ctx is done.
func (m *Monitor) Start(ctx context.Context) {
	m.sample()
	go func() {
		ticker := time.NewTicker(m.opt.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.sample()
			}
		}
	}()
}

func (m *Monitor) maxSamples() int {
	return int(m.opt.History/m.opt.Interval) + 1
}

func (m *Monitor) sample() {
	v, err := m.srv.Varz(nil)
	if err != nil {
		return
	}
	s := sample{
		Time:        time.Now(),
		InMsgs:      v.InClientMsgs,
		OutMsgs:     v.OutClientMsgs,
		InBytes:     v.InClientBytes,
		OutBytes:    v.OutClientBytes,
		Connections: v.Connections,
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.node = v.Name
	m.version = v.Version
	m.samples = append(m.samples, s)
	if over := len(m.samples) - m.maxSamples(); over > 0 {
		m.samples = m.samples[over:]
	}
}

type SeriesPoint struct {
	Time     time.Time `json:"time"`
	MsgRate  *float64  `json:"msgRate"`
	ByteRate *float64  `json:"byteRate"`
}

type ConsumerUsage struct {
	Stream string `json:"stream"`
	// State is "ok", "not_created" or "unknown".
	State string `json:"state"`
	Usage int    `json:"usage"`
	// Limit is -1 when there is no effective limit.
	Limit int `json:"limit"`
	// LimitState is "limited", "unlimited" or "unknown".
	LimitState string `json:"limitState"`
}

type Overview struct {
	Node        string        `json:"node"`
	Version     string        `json:"version"`
	SampledAt   time.Time     `json:"sampledAt"`
	Connections int           `json:"connections"`
	Consumer    ConsumerUsage `json:"consumer"`
	Series      []SeriesPoint `json:"series"`
}

func (m *Monitor) Overview() Overview {
	m.mu.RLock()
	samples := append([]sample(nil), m.samples...)
	node, version := m.node, m.version
	m.mu.RUnlock()

	ov := Overview{
		Node:     node,
		Version:  version,
		Consumer: m.consumerUsage(),
		Series:   buildSeries(samples, m.opt.Interval),
	}
	if n := len(samples); n > 0 {
		ov.SampledAt = samples[n-1].Time
		ov.Connections = samples[n-1].Connections
	}
	return ov
}

// buildSeries converts cumulative samples to per-second rates. Counter resets and missing samples leave a gap (null).
func buildSeries(samples []sample, interval time.Duration) []SeriesPoint {
	maxGap := interval*7/4 + time.Second
	out := make([]SeriesPoint, 0, len(samples))
	for i := 1; i < len(samples); i++ {
		prev, cur := samples[i-1], samples[i]
		dt := cur.Time.Sub(prev.Time).Seconds()
		p := SeriesPoint{Time: cur.Time}
		if dt > 0 && cur.Time.Sub(prev.Time) <= maxGap {
			inMsgs, outMsgs := cur.InMsgs-prev.InMsgs, cur.OutMsgs-prev.OutMsgs
			inBytes, outBytes := cur.InBytes-prev.InBytes, cur.OutBytes-prev.OutBytes
			if inMsgs >= 0 && outMsgs >= 0 && inBytes >= 0 && outBytes >= 0 {
				msgRate := float64(inMsgs+outMsgs) / dt
				byteRate := float64(inBytes+outBytes) / dt
				p.MsgRate, p.ByteRate = &msgRate, &byteRate
			}
		}
		out = append(out, p)
	}
	return out
}

func (m *Monitor) consumerUsage() ConsumerUsage {
	u := ConsumerUsage{Stream: StreamName}
	if m.streams == nil {
		u.State, u.LimitState = "unknown", "unknown"
		return u
	}
	info, err := m.streams.StreamInfo(StreamName)
	if errors.Is(err, nats.ErrStreamNotFound) {
		u.State, u.LimitState = "not_created", "unknown"
		return u
	}
	if err != nil {
		u.State, u.LimitState = "unknown", "unknown"
		return u
	}

	u.State = "ok"
	u.Usage = info.State.Consumers
	u.Limit = info.Config.MaxConsumers
	if u.Limit <= 0 {
		u.Limit = m.opt.ServerDefaultMaxConsumers
	}
	if u.Limit <= 0 {
		u.Limit, u.LimitState = -1, "unlimited"
	} else {
		u.LimitState = "limited"
	}
	return u
}

type ConnQuery struct {
	// ClientID is an exact MQTT client ID, filtered by NATS Connz.
	ClientID string
	// Sort is one of the NATS Connz sort options; see ValidSort.
	Sort     string
	Page     int
	PageSize int
}

type ConnItem struct {
	CID           uint64    `json:"cid"`
	ClientID      string    `json:"clientId"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	IP            string    `json:"ip"`
	Port          int       `json:"port"`
	Start         time.Time `json:"start"`
	LastActivity  time.Time `json:"lastActivity"`
	InMsgs        int64     `json:"inMsgs"`
	OutMsgs       int64     `json:"outMsgs"`
	InBytes       int64     `json:"inBytes"`
	OutBytes      int64     `json:"outBytes"`
	Subscriptions uint32    `json:"subscriptions"`
}

type ConnPage struct {
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
	Total    int        `json:"total"`
	Items    []ConnItem `json:"items"`
}

type ConnDetail struct {
	CID          uint64    `json:"cid"`
	ClientID     string    `json:"clientId"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	IP           string    `json:"ip"`
	Port         int       `json:"port"`
	Start        time.Time `json:"start"`
	LastActivity time.Time `json:"lastActivity"`
	InMsgs       int64     `json:"inMsgs"`
	OutMsgs      int64     `json:"outMsgs"`
	InBytes      int64     `json:"inBytes"`
	OutBytes     int64     `json:"outBytes"`
	Pending      int       `json:"pending"`
	Idle         string    `json:"idle"`
	Uptime       string    `json:"uptime"`
	RTT          string    `json:"rtt"`
	// SubscriptionCount is the total reported by NATS, including internal subscriptions.
	SubscriptionCount     uint32   `json:"subscriptionCount"`
	Subscriptions         []string `json:"subscriptions"`
	InternalSubscriptions []string `json:"internalSubscriptions"`
}

// Connections returns one NATS page. Filtering by Client ID, sorting and pagination
// are performed by the embedded server; no result set is fetched and re-processed here.
func (m *Monitor) Connections(q ConnQuery) (ConnPage, error) {
	page, pageSize := q.Page, q.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	connz, err := m.srv.Connz(&server.ConnzOptions{
		MQTTClient: q.ClientID,
		Sort:       connzSort(q.Sort),
		Offset:     (page - 1) * pageSize,
		Limit:      pageSize,
	})
	if err != nil {
		return ConnPage{}, err
	}
	items := make([]ConnItem, 0, len(connz.Conns))
	for _, c := range connz.Conns {
		items = append(items, connItem(c))
	}
	return ConnPage{Page: page, PageSize: pageSize, Total: connz.Total, Items: items}, nil
}

func connItem(c *server.ConnInfo) ConnItem {
	return ConnItem{
		CID:           c.Cid,
		ClientID:      c.MQTTClient,
		Name:          c.Name,
		Type:          c.Type,
		IP:            c.IP,
		Port:          c.Port,
		Start:         c.Start,
		LastActivity:  c.LastActivity,
		InMsgs:        c.InMsgs,
		OutMsgs:       c.OutMsgs,
		InBytes:       c.InBytes,
		OutBytes:      c.OutBytes,
		Subscriptions: c.NumSubs,
	}
}

func (m *Monitor) Connection(cid uint64) (ConnDetail, error) {
	connz, err := m.srv.Connz(&server.ConnzOptions{CID: cid, Subscriptions: true})
	if err != nil {
		return ConnDetail{}, err
	}
	for _, c := range connz.Conns {
		if c.Cid != cid {
			continue
		}
		visible, internal := splitSubscriptions(c.Subs)
		return ConnDetail{
			CID:                   c.Cid,
			ClientID:              c.MQTTClient,
			Name:                  c.Name,
			Type:                  c.Type,
			IP:                    c.IP,
			Port:                  c.Port,
			Start:                 c.Start,
			LastActivity:          c.LastActivity,
			InMsgs:                c.InMsgs,
			OutMsgs:               c.OutMsgs,
			InBytes:               c.InBytes,
			OutBytes:              c.OutBytes,
			Pending:               c.Pending,
			Idle:                  c.Idle,
			Uptime:                c.Uptime,
			RTT:                   c.RTT,
			SubscriptionCount:     c.NumSubs,
			Subscriptions:         visible,
			InternalSubscriptions: internal,
		}, nil
	}
	return ConnDetail{}, ErrConnectionNotFound
}

func splitSubscriptions(subs []string) (visible, internal []string) {
	visible = make([]string, 0, len(subs))
	internal = make([]string, 0)
	for _, s := range subs {
		if strings.HasPrefix(s, "$MQTT.") {
			internal = append(internal, s)
			continue
		}
		visible = append(visible, s)
	}
	return visible, internal
}
