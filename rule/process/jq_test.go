package process_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/rule/process"
)

func Test_JqRun(t *testing.T) {
	r, err := process.NewJqRunner(".a")
	require.NoError(t, err)
	out, err := r.Run(context.TODO(), map[string]any{"a": 3})
	require.NoError(t, err)
	fmt.Printf("out: %v\n", out)
}
