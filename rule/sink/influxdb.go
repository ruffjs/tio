package sink

import (
	"log/slog"
	"os"

	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/rule/connector"
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

func NewInfluxDB(name string, cfg map[string]any, conn connector.Conn) Sink {
	var ac InfluxDBConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		slog.Error("decode sink InfluxDB config", "name", name, "error", err)
		os.Exit(1)
	}
	c, ok := conn.(*connector.InfluxDB)
	if !ok {
		slog.Error("wrong connector type for InfluxDB sink")
		os.Exit(1)
	}

	a := &InfluxDBImpl{
		name: name,
		cfg:  ac,
		conn: c,
		ch:   make(chan *Msg, 10000),
	}
	go a.publishLoop()
	return a
}

type InfluxDBImpl struct {
	name string
	cfg  InfluxDBConfig
	conn *connector.InfluxDB
	ch   chan *Msg
}

func (s *InfluxDBImpl) Name() string {
	return s.name
}

func (*InfluxDBImpl) Type() string {
	return TypeInfluxDB
}

func (s *InfluxDBImpl) Publish(msg Msg) {
	s.ch <- &msg
}

func (s *InfluxDBImpl) publishLoop() {
	for {
		msg := <-s.ch
		r, err := s.conn.Client().R().
			SetBody(msg.Payload).
			Post("")
		if err != nil {
			slog.Error("Rule sinke InfluxDB post data", "error", err, "resposeBody", r.Body())
		} else if r.IsError() {
			slog.Error("Rule sink InfluxDB post data", "httpStatus", r.StatusCode, "resposeBody", r.Body())
		} else {
			slog.Debug("Rule sink InfluxDB post data SUCCESS")
		}
	}
}
