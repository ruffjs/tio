package sink

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
)

// Http sink

const TypeHttp = "http"

func init() {
	Register(TypeHttp, NewHttp)
}

type HttpConfig struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
}

func NewHttp(ctx context.Context, name string, cfg map[string]any, conn connector.Conn) (Sink, error) {
	var ac HttpConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		slog.Error("decode sink Http config", "name", name, "error", err)
		return nil, fmt.Errorf("decode config: %w", err)
	}
	ac.Method = strings.ToUpper(ac.Method)

	c, ok := conn.(*connector.Http)
	if !ok {
		return nil, fmt.Errorf("wrong connector type for Http sink")
	}

	a := &HttpImpl{
		ctx:  ctx,
		name: name,
		cfg:  ac,
		conn: c,
		ch:   make(chan *Msg, 10000),
	}
	go a.publishLoop()
	return a, nil
}

type HttpImpl struct {
	ctx  context.Context
	name string
	cfg  HttpConfig
	conn *connector.Http
	ch   chan *Msg

	started bool
}

func (s *HttpImpl) Start() error {
	s.started = true
	return s.Status().Error
}

func (s *HttpImpl) Status() model.StatusInfo {
	if !s.started {
		return model.StatusNotStarted()
	}
	return withConnStatus(s.conn.Name(), s.conn.Status())
}

func (s *HttpImpl) Stop() error {
	s.started = false
	return nil
}

func (s *HttpImpl) Name() string {
	return s.name
}

func (*HttpImpl) Type() string {
	return TypeHttp
}

func (s *HttpImpl) Publish(msg Msg) {
	if s.started {
		s.ch <- &msg
	}
}

func (s *HttpImpl) publishLoop() {
	for {
		msg := <-s.ch
		r := s.conn.Client().R().
			SetContext(s.ctx).
			SetHeaders(s.cfg.Headers)
		if s.cfg.Method != "GET" && s.cfg.Method != "DELETE" {
			r.SetBody(msg)
		}
		resp, err := r.SetContext(s.ctx).Execute(s.cfg.Method, s.cfg.Path)

		if err != nil {
			slog.Error("Rule sinke Http send data", "error", err, "resposeBody", resp.Body())
		} else if resp.IsError() {
			slog.Error("Rule sink Http send data", "httpStatus", resp.StatusCode, "resposeBody", resp.Body())
		} else {
			slog.Debug("Rule sink Http send data SUCCESS", "message", msg)
		}
	}
}
