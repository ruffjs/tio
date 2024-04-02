package process_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/rule/process"
)

func Test_New(t *testing.T) {
	configs := []struct {
		conf   process.Config
		hasErr bool
	}{
		{
			conf: process.Config{
				Name: "filter-1",
				Type: "filter",
				Jq:   ".a > 0",
			},
		},
		{
			conf: process.Config{
				Name: "trans-1",
				Type: "transform",
				Jq:   ".a",
			},
		},
		{
			conf: process.Config{
				Name: "wrong-type",
				Type: "wrong-type-xx",
				Jq:   ".a",
			},
			hasErr: true,
		},
		{
			conf: process.Config{
				Name: "wrong jq string",
				Type: "wrong-jq",
				Jq:   ".a.",
			},
			hasErr: true,
		},
	}

	for _, c := range configs {
		p, err := process.NewProcess(c.conf)
		if c.hasErr {
			require.Error(t, err)
			continue
		}
		require.Equal(t, c.conf.Name, p.Name())
		require.Equal(t, c.conf.Type, p.Type())
	}
}

func Test_Filter(t *testing.T) {
	defaultFilter, err := process.NewFilter("compare-filter", ".a > 0")
	require.NoError(t, err)

	propGetFilter, err := process.NewFilter("prop-get-filter", ".a")
	require.NoError(t, err)

	cases := []struct {
		in     any
		out    any
		filter process.Process
		hasErr bool
	}{
		{
			in:  nil,
			out: false,
		},
		{
			in:  map[string]any{"a": 33},
			out: true,
		},
		{
			in:  map[string]any{"a": -1},
			out: false,
		},
		{
			// can't be scalar, must be map[string]any
			in:     "3",
			hasErr: true,
		},
		{
			in:     []any{map[string]any{"a": 3}},
			hasErr: true,
		},
		{
			in:     map[string]any{"a": true},
			out:    true,
			filter: propGetFilter,
		},
		{
			// filter reuslt can't be nil, must be bool
			in:     map[string]any{"xxx": true},
			filter: propGetFilter,
			hasErr: true,
		},
	}

	for _, c := range cases {
		f := defaultFilter
		if c.filter != nil {
			f = c.filter
		}
		o, err := f.Run(c.in)
		if c.hasErr {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, c.out, o)
	}
}

func Test_Transform(t *testing.T) {
	f, err := process.NewTrans("test-trans", ".a")
	require.NoError(t, err)

	cases := []struct {
		in     any
		out    any
		hasErr bool
	}{
		{
			in:  nil,
			out: nil,
		},
		{
			in:  map[string]any{"a": 33},
			out: 33,
		},
		{
			in:  map[string]any{"a": -1},
			out: -1,
		},
		{
			// can't be scalar, must be map[string]any
			in:     "3",
			hasErr: true,
		},
		{
			in:     []any{map[string]any{"a": 3}},
			hasErr: true,
		},
	}

	for _, c := range cases {
		o, err := f.Run(c.in)
		if c.hasErr {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, c.out, o)
	}
}
