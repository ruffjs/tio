package redissplit_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/pkg/redissplit"
)

// expect(s('set foo bar')).to.eql(['set', 'foo', 'bar']);
// expect(s('set "foo bar"')).to.eql(['set', 'foo bar']);
// expect(s('set "foo bar\\" baz"')).to.eql(['set', 'foo bar" baz']);
// expect(s('set \\  bar')).to.eql(['set', '\\', 'bar']);
// expect(s('  set    foo  \r \n  bar  \v ')).to.eql(['set', 'foo', 'bar']);
// expect(s('"set" "foo" "bar"')).to.eql(['set', 'foo', 'bar']);

// expect(function () { s('set foo "bar'); }).to.throw();
// expect(function () { s('set foo "bar"dsf'); }).to.throw();
// expect(function () { s("set foo 'bar"); }).to.throw();

func Test_redissplit(t *testing.T) {
	cases := []struct {
		in     string
		out    []string
		hasErr bool
	}{
		{
			in:  "set foo bar",
			out: []string{"set", "foo", "bar"},
		},
		{
			in:  `set "foo bar"`,
			out: []string{"set", "foo bar"},
		},
		{
			in:  `set "foo bar\" baz"`,
			out: []string{"set", `foo bar" baz`},
		},
		{
			in:  `set \  bar`,
			out: []string{"set", `\`, "bar"},
		},
		{
			in:  "  set    foo  \r \n  bar  \v ",
			out: []string{"set", "foo", "bar"},
		},
		{
			in:  `"set" "foo" "bar"`,
			out: []string{"set", "foo", "bar"},
		},
		{
			in:     `set foo "bar`,
			hasErr: true,
		},
		{
			in:     `set foo "bar"dsf`,
			hasErr: true,
		},
		{
			in:     `set foo 'bar`,
			hasErr: true,
		},
	}
	for _, c := range cases {
		o, err := redissplit.SplitArgs(c.in)
		require.True(t, c.hasErr == (err != nil))
		require.Equal(t, c.out, o)
	}
}
