package mock

import (
	"log/slog"

	"github.com/stretchr/testify/mock"
	"ruff.io/tio/rule/model"
	"ruff.io/tio/rule/source"
)

func NewSource(name string) *SrcMock {
	return &SrcMock{
		name:     name,
		handlers: make(map[string]source.MsgHander),
	}
}

var _ source.Source = &SrcMock{}

type SrcMock struct {
	mock.Mock
	name     string
	handlers map[string]source.MsgHander
}

// Status implements source.Source.
func (s *SrcMock) Status() model.StatusInfo {
	panic("unimplemented")
}

func (s *SrcMock) Name() string {
	return s.name
}

func (s *SrcMock) MockMsg(msg source.Msg) {
	for _, m := range s.handlers {
		m(msg)
	}
}

func (s *SrcMock) OnMsg(ruleName string, h source.MsgHander) {
	s.handlers[ruleName] = h
}

func (s *SrcMock) Start() error {
	slog.Info("Moke rule source start invoked")
	r := s.Called()
	return r.Get(0).(error)
}

func (s *SrcMock) Stop() {
	slog.Info("Moke rule source stop invoked")
	s.Called()
}

func (s *SrcMock) Type() string {
	return "mock"
}
