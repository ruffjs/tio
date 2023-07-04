package shadow

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrans(t *testing.T) {
	cases := []struct {
		src map[string]any
		sel map[string]string
		res map[string]any
	}{
		{
			src: map[string]any{
				"a": 1,
				"b": 2,
				"x": map[string]any{
					"y": 3,
				},
			},
			sel: map[string]string{
				"a":   "q",
				"b":   "s",
				"x.y": "r",
			},
			res: map[string]any{
				"q": 1,
				"s": 2,
				"r": 3,
			},
		},
		{
			src: map[string]any{
				"a": 1,
				"x": 3,
			},
			sel: map[string]string{
				"a":   "q",
				"b":   "s",
				"x.y": "r",
			},
			res: map[string]any{
				"q": 1,
			},
		},
	}
	for _, c := range cases {
		res := transMap(c.src, c.sel)
		require.Equal(t, c.res, res, fmt.Sprintf("src=%#v , sel=%#v", c.src, c.sel))
	}

}
