package mock

import (
	"log/slog"

	"github.com/stretchr/testify/mock"
	"ruff.io/tio/rule/source"
)

func NewSource(name string) *SrcMock {
	return &SrcMock{
		name:     name,
		handlers: make([]source.MsgHander, 0),
	}
}

var _ source.Source = &SrcMock{}

type SrcMock struct {
	mock.Mock
	name     string
	handlers []source.MsgHander
}

func (s *SrcMock) Name() string {
	return s.name
}

func (s *SrcMock) MockMsg(msg source.Msg) {
	for _, m := range s.handlers {
		m(msg)
	}
}

func (s *SrcMock) OnMsg(h source.MsgHander) {
	s.handlers = append(s.handlers, h)
}

func (s *SrcMock) Start() {
	slog.Info("Moke rule source start invoked")
	s.Called()
}

func (s *SrcMock) Stop() {
	slog.Info("Moke rule source stop invoked")
	s.Called()
}

func (s *SrcMock) Type() string {
	return "mock"
}
