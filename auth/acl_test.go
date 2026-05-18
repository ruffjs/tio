package auth_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/auth"
	"ruff.io/tio/config"
	"ruff.io/tio/shadow"
)

func TestTopicAcl(t *testing.T) {

	mBg := &mockBindingGetter{}

	cases := []struct {
		name   string
		supers []config.UserPassword
		nsCfg  []config.Namespace
		user   string
		topic  string
		bind   struct {
			thingId string
		}
		write  bool
		result bool
	}{
		{
			name:   "super user should access all",
			supers: []config.UserPassword{{Name: "a"}, {Name: "b"}},
			user:   "a",
			topic:  shadow.TopicUpdateOf("c"),
			write:  true,
			result: true,
		},
		{
			name:   "super user should access all",
			supers: []config.UserPassword{{Name: "a"}, {Name: "b"}},
			user:   "b",
			topic:  shadow.TopicStateUpdatedOf("c"),
			write:  true,
			result: true,
		},
		{
			name:   "wrong user can't access",
			supers: []config.UserPassword{{Name: "a"}, {Name: "b"}},
			user:   "d",
			topic:  shadow.TopicStateUpdatedOf("c"),
			write:  true,
			result: false,
		},
		{
			name:   "user can access their own topic",
			supers: []config.UserPassword{{Name: "a"}, {Name: "b"}},
			user:   "c",
			topic:  shadow.TopicUpdateOf("c"),
			write:  true,
			result: true,
		},
		{
			name:   "gateway can access bound thing's topic",
			supers: []config.UserPassword{{Name: "a"}, {Name: "b"}},
			user:   "e",
			topic:  shadow.TopicUpdateOf("x"),
			bind:   struct{ thingId string }{thingId: "x"},
			write:  true,
			result: true,
		},
		{
			name:   "namespace user can subscribe own namespace",
			nsCfg:  []config.Namespace{{Name: "biz"}},
			user:   "$ns/biz",
			topic:  "$iothub/ns/biz/things/c/presence",
			result: true,
		},
		{
			name:   "namespace user cannot subscribe other namespace",
			nsCfg:  []config.Namespace{{Name: "biz"}},
			user:   "$ns/biz",
			topic:  "$iothub/ns/other/things/c/presence",
			result: false,
		},
		{
			name:   "namespace user cannot publish namespace topic",
			nsCfg:  []config.Namespace{{Name: "biz"}},
			user:   "$ns/biz",
			topic:  "$iothub/ns/biz/things/c/presence",
			write:  true,
			result: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var call *mock.Call
			if c.bind.thingId != "" {
				call = mBg.Mock.On("IsBoundGateway", mock.Anything, c.bind.thingId, c.user).Return(true, nil)
			} else {
				call = mBg.Mock.On("IsBoundGateway", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
			}

			aclFn := auth.TopicAcl(mBg, c.supers, c.nsCfg)
			r := aclFn(c.user, c.user, c.topic, c.write)
			require.Equal(t, c.result, r, fmt.Sprintf("user %s should access %s : %t", c.user, c.topic, c.result))

			call.Unset()
		})
	}
}

type mockBindingGetter struct {
	mock.Mock
}

func (m *mockBindingGetter) IsBoundGateway(ctx context.Context, thingId string, gatewayThingId string) (bool, error) {
	r := m.Called(ctx, thingId, gatewayThingId)
	return r.Bool(0), r.Error(1)
}
