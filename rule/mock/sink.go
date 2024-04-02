package mock

import (
	"github.com/stretchr/testify/mock"
	"ruff.io/tio/rule/sink"
)

const TypeLog = "mock"

func NewSink(name string) *SinkMock {
	a := &SinkMock{
		name: name,
	}
	return a
}

var _ sink.Sink = &SinkMock{}

type SinkMock struct {
	mock.Mock
	name string
}

func (s *SinkMock) Name() string {
	return s.name
}

func (*SinkMock) Type() string {
	return TypeLog
}

func (s *SinkMock) Publish(msg sink.Msg) {
	s.Called(msg)
}
