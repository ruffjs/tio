package mock

import (
	"github.com/stretchr/testify/mock"
	"ruff.io/tio/shadow"
)

func NewShadowGetter() *MockShadowGetter {
	return &MockShadowGetter{}
}

type MockShadowGetter struct {
	mock.Mock
}

func (m *MockShadowGetter) GetFromCache(thingId string) (shadow.ShadowWithStatus, bool) {
	args := m.Called(thingId)
	return args.Get(0).(shadow.ShadowWithStatus), args.Get(1).(bool)
}

var _ shadow.CacheGetter = (*MockShadowGetter)(nil)
