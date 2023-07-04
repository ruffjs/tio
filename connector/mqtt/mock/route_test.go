package mock

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatchTopic(t *testing.T) {
	cases := []struct {
		sub   string
		topic string
		match bool
	}{
		{"/#", "/test", true},
		{"/#", "/test/ssss", true},
		{"/#", "/test/ssss/df", true},
		{"/test/#", "/test/ssss/df", true},
		{"/test/+/b/c", "/test/a/b/c", true},
		{"/test/a/#", "/test/a/b/c", true},
		{"/test/b/#", "/test/a/b/c", false},
		{"/test/x/#", "/test/a/b/c", false},
	}

	for _, c := range cases {
		re := MatchTopic(c.sub, c.topic)
		require.Equal(t, c.match, re, "sub=%q topic=%q match=%t", c.sub, c.topic, c.match)
	}
}
