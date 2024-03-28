package model_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/pkg/model"
)

func Test_GetThingIdFromTopic(t *testing.T) {
	cases := []struct {
		thingId string
		topic   string
		hasErr  bool
	}{
		{
			"test",
			"$iothub/things/test/xxx",
			false,
		},
		{
			"test",
			"$iothub/user/things/test/xxx",
			false,
		},
		{
			"abc",
			"$iothub/events/things/abc/presence",
			false,
		},
		{
			"abc",
			"$iothub/things/abc/presence",
			false,
		},
		{
			"xyzuvw",
			"$iothub/things/xyzuvw/jobs",
			false,
		},
	}
	for _, c := range cases {
		id, err := model.GetThingIdFromTopic(c.topic)
		require.True(t, c.hasErr == (err != nil))
		require.Equal(t, c.thingId, id)
	}
}
