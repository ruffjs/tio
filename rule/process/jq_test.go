package process_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/rule/process"
)

func Test_JqRun(t *testing.T) {
	cases := []struct {
		input  any
		jq     string
		output any
		hasErr bool
	}{
		{
			input:  map[string]any{"a": 3},
			jq:     ".a",
			output: 3,
		},
		{
			input:  nil,
			jq:     ".a",
			output: nil,
		},
	}

	for _, c := range cases {
		r, err := process.NewJqRunner(c.jq)
		require.NoError(t, err)
		out, err := r.Run(context.TODO(), c.input)
		require.NoError(t, err)
		require.Equal(t, c.output, out)
	}
}
