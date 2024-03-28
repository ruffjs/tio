package sink

import (
	"log/slog"
	"os"
	"strings"

	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/rule/connector"
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

func NewHttp(name string, cfg map[string]any, conn connector.Conn) Sink {
	var ac HttpConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		slog.Error("decode sink Http config", "name", name, "error", err)
		os.Exit(1)
	}
	ac.Method = strings.ToUpper(ac.Method)

	c, ok := conn.(*connector.Http)
	if !ok {
		slog.Error("wrong connector type for Http sink")
		os.Exit(1)
	}

	a := &HttpImpl{
		name: name,
		cfg:  ac,
		conn: c,
		ch:   make(chan *Msg, 10000),
	}
	go a.publishLoop()
	return a
}

type HttpImpl struct {
	name string
	cfg  HttpConfig
	conn *connector.Http
	ch   chan *Msg
}

func (s *HttpImpl) Name() string {
	return s.name
}

func (*HttpImpl) Type() string {
	return TypeHttp
}

func (s *HttpImpl) Publish(msg Msg) {
	s.ch <- &msg
}

func (s *HttpImpl) publishLoop() {
	for {
		msg := <-s.ch
		r := s.conn.Client().R().
			SetHeaders(s.cfg.Headers)
		if s.cfg.Method != "GET" && s.cfg.Method != "DELETE" {
			r.SetBody(msg)
		}
		resp, err := r.Execute(s.cfg.Method, s.cfg.Path)

		if err != nil {
			slog.Error("Rule sinke Http send data", "error", err, "resposeBody", resp.Body())
		} else if resp.IsError() {
			slog.Error("Rule sink Http send data", "httpStatus", resp.StatusCode, "resposeBody", resp.Body())
		} else {
			slog.Debug("Rule sink Http send data SUCCESS", "message", msg)
		}
	}
}
