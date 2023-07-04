package shadow_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/shadow"
)

func TestTopicMethodRequest(t *testing.T) {
	cases := []struct {
		thingId string
		method  string
		expect  string
	}{
		{thingId: "abcd", method: "mmm1", expect: "$iothub/things/abcd/methods/mmm1/req"},
		{thingId: "xxqk", method: "m0sd5", expect: "$iothub/things/xxqk/methods/m0sd5/req"},
	}

	for _, c := range cases {
		topic := shadow.TopicMethodRequest(c.thingId, c.method)
		require.Equal(t, topic, c.expect)
	}
}

func TestTopicMethodAllResponse(t *testing.T) {
	expect := "$iothub/things/+/methods/+/resp"
	topic := shadow.TopicMethodAllResponse()
	require.Equal(t, expect, topic)
}
