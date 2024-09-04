package sink

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
)

// InfluxDB sink, use influxdb line protocol
// Ref: https://docs.influxdata.com/influxdb/v2/reference/syntax/line-protocol

// Transform data to line protocol and sink to influxdb, jq script example for PresenceEvent:
//   - input:  {"thingId":"test", "eventType": "connected", "timestamp": 1711529686403}
//   - jq:     .payload | "presence,thingId=" + .thingId + " v=" + (.eventType=="connected"|tostring) + " " + (.timestamp|tostring)
//   - output: presence,thingId=test v=true 1711529686403

const TypeInfluxDB = "influxdb"

func init() {
	Register(TypeInfluxDB, NewInfluxDB)
}

type InfluxDBConfig struct {
}

func NewInfluxDB(ctx context.Context, name string, cfg map[string]any, conn connector.Conn) (Sink, error) {
	var ac InfluxDBConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, fmt.Errorf("decode config")
	}
	c, ok := conn.(*connector.InfluxDB)
	if !ok {
		return nil, fmt.Errorf("wrong connector type for InfluxDB sink")
	}

	a := &InfluxDBImpl{
		ctx:  ctx,
		name: name,
		cfg:  ac,
		conn: c,
		ch:   make(chan *Msg, 10000),
	}
	go a.publishLoop()
	return a, nil
}

type InfluxDBImpl struct {
	ctx  context.Context
	name string
	cfg  InfluxDBConfig
	conn *connector.InfluxDB
	ch   chan *Msg

	started bool
}

func (s *InfluxDBImpl) Start() error {
	s.started = true
	return s.Status().Error
}

func (s *InfluxDBImpl) Status() model.StatusInfo {
	if !s.started {
		return model.StatusNotStarted()
	}
	return withConnStatus(s.conn.Name(), s.conn.Status())
}

func (s *InfluxDBImpl) Stop() error {
	s.started = false
	return nil
}

func (s *InfluxDBImpl) Name() string {
	return s.name
}

func (*InfluxDBImpl) Type() string {
	return TypeInfluxDB
}

func (s *InfluxDBImpl) Publish(msg Msg) {
	if s.started {
		s.ch <- &msg
	}
}

func (s *InfluxDBImpl) publishLoop() {
	for {
		var msg *Msg
		select {
		case <-s.ctx.Done():
			return
		case msg = <-s.ch:
		}
		r, err := s.conn.Client().R().
			SetContext(s.ctx).
			SetBody(msg.Payload).
			Post("")
		if err != nil {
			slog.Error("Rule sinke InfluxDB post data", "error", err, "resposeBody", r.Body())
		} else if r.IsError() {
			slog.Error("Rule sink InfluxDB post data", "httpStatus", r.StatusCode, "resposeBody", r.Body())
		} else {
			slog.Debug("Rule sink InfluxDB post data SUCCESS", "payload", msg.Payload)
		}
	}
}
