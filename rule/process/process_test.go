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
				Name:   "filter-1",
				Type:   "filter",
				Runner: "jq",
				Jq:     ".a > 0",
			},
		},
		{
			conf: process.Config{
				Name:   "trans-1",
				Type:   "transform",
				Runner: "jq",
				Jq:     ".a",
			},
		},
		{
			conf: process.Config{
				Name:   "wrong-type",
				Type:   "wrong-type-xx",
				Runner: "jq",
				Jq:     ".a",
			},
			hasErr: true,
		},
		{
			conf: process.Config{
				Name:   "wrong jq string",
				Type:   "wrong-jq",
				Runner: "jq",
				Jq:     ".a.",
			},
			hasErr: true,
		},

		// js

		{
			conf: process.Config{
				Name:   "filter-11",
				Type:   "filter",
				Runner: "js",
				Js:     "function run() { return 3 > 0; }",
			},
		},
		{
			conf: process.Config{
				Name:   "trans-11",
				Type:   "transform",
				Runner: "js",
				Js:     "function run(data) {return null;}",
			},
		},
		{
			conf: process.Config{
				Name:   "wrong js string",
				Type:   "transform",
				Runner: "js",
				Js:     "a=1",
			},
			hasErr: true,
		},
	}

	for _, c := range configs {
		p, err := process.NewProcess(c.conf)
		if c.hasErr {
			require.Error(t, err, c.conf.Name)
			continue
		} else {
			require.NoError(t, err, c.conf.Name)
		}
		require.Equal(t, c.conf.Name, p.Name(), c.conf.Name)
		require.Equal(t, c.conf.Type, p.Type(), c.conf.Name)
	}
}

func Test_Filter(t *testing.T) {
	defaultFilter, err := process.NewFilter(process.Config{Name: "compare-filter", Jq: ".a > 0"})
	require.NoError(t, err)

	propGetFilter, err := process.NewFilter(process.Config{Name: "prop-get-filter", Jq: ".a"})
	require.NoError(t, err)

	jsFilter, err := process.NewFilter(process.Config{Name: "js-filter", Js: "function run(d) { return d.a > 0; }"})
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
		{
			in:     map[string]any{"a": true},
			out:    true,
			filter: jsFilter,
		},
		{
			in:     map[string]any{"a": false},
			out:    false,
			filter: jsFilter,
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
	jqTr, err := process.NewTrans(process.Config{Name: "jq-trans", Jq: ".a"})
	require.NoError(t, err)

	jsTr, err := process.NewTrans(process.Config{Name: "js-trans", Js: "run = d => d.a ;"})
	require.NoError(t, err)

	cases := []struct {
		in     any
		out    any
		tr     process.Process
		hasErr bool
	}{
		{
			tr:  jqTr,
			in:  nil,
			out: nil,
		},
		{
			tr:  jsTr,
			in:  nil,
			out: nil,
		},
		{
			tr:  jqTr,
			in:  map[string]any{"a": 33},
			out: 33,
		},
		{
			tr:  jsTr,
			in:  map[string]any{"a": 33},
			out: 33,
		},
		{
			tr:  jqTr,
			in:  map[string]any{"a": -1},
			out: -1,
		},
		{
			tr:  jsTr,
			in:  map[string]any{"a": -1},
			out: -1,
		},
		{
			// can't be scalar, must be map[string]any
			tr:     jqTr,
			in:     "3",
			hasErr: true,
		},
		{
			// can't be scalar, must be map[string]any
			tr:     jsTr,
			in:     "3",
			hasErr: true,
		},
		{
			tr:     jqTr,
			in:     []any{map[string]any{"a": 3}},
			hasErr: true,
		},
		{
			tr:     jsTr,
			in:     []any{map[string]any{"a": 3}},
			hasErr: true,
		},
	}

	for _, c := range cases {
		o, err := jqTr.Run(c.in)
		if c.hasErr {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, c.out, o)
	}
}
