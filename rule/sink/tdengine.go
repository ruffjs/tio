package sink

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mitchellh/mapstructure"
	"ruff.io/tio"
	"ruff.io/tio/pkg/uuid"
	"ruff.io/tio/rule/connector"
)

// Tdengine sink, use SQL
// Ref: https://docs.tdengine.com/reference/rest-api/

// Transform data to SQL and sink to Tdengine, jq script example for PresenceEvent:
//   - input:  {"payload": {"thingId":"test", "eventType": "connected", "timestamp": 1711529686403} }
//   - jq:     .payload | "INSERT INTO presence(ts, thing_id, type) VALUES(" + (.timestamp|tostring) + ", \"" + .thingId + "\", \"" + .eventType + "\")"
//   - output: presence,thingId=test v=true 1711529686403

const TypeTdengine = "tdengine"

func init() {
	Register(TypeTdengine, NewTdengine)
}

type TdengineConfig struct {
}

func NewTdengine(name string, cfg map[string]any, conn connector.Conn) Sink {
	var ac TdengineConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		slog.Error("decode sink Tdengine config", "name", name, "error", err)
		os.Exit(1)
	}
	c, ok := conn.(*connector.Tdengine)
	if !ok {
		slog.Error("wrong connector type for Tdengine sink")
		os.Exit(1)
	}

	a := &TdengineImpl{
		name:     name,
		cfg:      ac,
		conn:     c,
		ch:       make(chan *Msg, 10000),
		uuidProd: uuid.New(),
	}
	go a.publishLoop()
	return a
}

type TdengineImpl struct {
	name     string
	cfg      TdengineConfig
	conn     *connector.Tdengine
	ch       chan *Msg
	uuidProd tio.IdProvider
}

func (s *TdengineImpl) Name() string {
	return s.name
}

func (*TdengineImpl) Type() string {
	return TypeTdengine
}

func (s *TdengineImpl) Publish(msg Msg) {
	s.ch <- &msg
}

func (s *TdengineImpl) publishLoop() {
	for {
		msg := <-s.ch
		reqId := fmt.Sprintf("%d", time.Now().UnixNano())
		r, err := s.conn.Client().R().
			SetQueryParam("req_id", reqId).
			SetBody(msg.Payload).
			Post("")
		if err != nil {
			slog.Error("Rule sinke Tdengine post data", "reqId", reqId, "error", err, "resposeBody", r.Body())
		} else if r.IsError() {
			slog.Error("Rule sink Tdengine post data", "reqId", reqId, "httpStatus", r.StatusCode, "resposeBody", r.Body())
		} else {
			slog.Debug("Rule sink Tdengine post data SUCCESS", "reqId", reqId, "payload", msg.Payload)
		}
	}
}
