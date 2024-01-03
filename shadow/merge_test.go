package shadow_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/shadow"
)

var cases = []struct {
	name        string
	target      shadow.StateValue
	src         shadow.StateValue
	want        shadow.StateValue
	keysUpdated []string
	keysRemoved []string
}{
	{
		"add 1 and 3 depth",
		shadow.StateValue{"a": 1},
		shadow.StateValue{"a": 1, "b": map[string]any{"bb": map[string]any{"bbb": 43}}},
		shadow.StateValue{"a": 1, "b": map[string]any{"bb": map[string]any{"bbb": 43}}},
		[]string{"a", "b.bb.bbb"},
		[]string{},
	},
	{
		"add and remove",
		shadow.StateValue{"hi": "you", "hello": "world", "test": map[string]any{"aa": 23}},
		shadow.StateValue{"hi": nil, "hello": nil, "test": map[string]any{"bb": 43}},
		shadow.StateValue{"test": map[string]any{"aa": 23, "bb": 43}},
		[]string{"test.bb"},
		[]string{"hi", "hello"},
	},
	{
		"replace scalar by map",
		shadow.StateValue{"hi": "you", "hello": "world", "test": "xx"},
		shadow.StateValue{"hi": "you", "hello": "world", "test": map[string]any{"aa": 23}},
		shadow.StateValue{"hi": "you", "hello": "world", "test": map[string]any{"aa": 23}},
		[]string{"test.aa"},
		[]string{},
	},
	{
		"add hello",
		shadow.StateValue{"hi": "you"},
		shadow.StateValue{"hi": "you", "hello": "world"},
		shadow.StateValue{"hi": "you", "hello": "world"},
		[]string{"hello"},
		[]string{},
	},
	{
		"merge numbers",
		shadow.StateValue{},
		shadow.StateValue{"int": 20, "float64": 3.45},
		shadow.StateValue{"int": 20, "float64": 3.45},
		[]string{"int", "float64"},
		[]string{},
	},
	{
		"merge array",
		shadow.StateValue{},
		shadow.StateValue{"a": []any{1, "a"}},
		shadow.StateValue{"a": []any{1, "a"}},
		[]string{"a"},
		[]string{},
	},
	{
		"merge nested map to nil",
		nil,
		shadow.StateValue{"hi": "you", "hello": "world", "test": map[string]any{"aa": 23}},
		shadow.StateValue{"hi": "you", "hello": "world", "test": map[string]any{"aa": 23}},
		[]string{"hi", "hello", "test.aa"},
		[]string{},
	},
	{
		"merge same",
		shadow.StateValue{"hi": "you", "hello": "world"},
		shadow.StateValue{"hi": "you", "hello": "world"},
		shadow.StateValue{"hi": "you", "hello": "world"},
		[]string{"hi", "hello"},
		[]string{},
	},
	{
		"merge same with map",
		shadow.StateValue{"hi": "you", "hello": "world", "test": map[string]any{"aa": 23}},
		shadow.StateValue{"hi": "you", "hello": "world", "test": map[string]any{"aa": 23}},
		shadow.StateValue{"hi": "you", "hello": "world", "test": map[string]any{"aa": 23}},
		[]string{"hi", "hello", "test.aa"},
		[]string{},
	},
	{
		"remove map field",
		shadow.StateValue{"hi": "you", "hello": "world", "test": map[string]any{"aa": 23}},
		shadow.StateValue{"hi": "you", "hello": "world", "test": nil},
		shadow.StateValue{"hi": "you", "hello": "world"},
		[]string{"hi", "hello"},
		[]string{"test"},
	},
	{
		"remove all",
		shadow.StateValue{"hi": "you", "hello": "world", "test": map[string]any{"aa": 23}},
		shadow.StateValue{"hi": nil, "hello": nil, "test": nil},
		shadow.StateValue{},
		[]string{},
		[]string{"hi", "hello", "test"},
	},
	{
		"merge nothong",
		shadow.StateValue{"hi": "you", "hello": "world", "test": map[string]any{"aa": 23}},
		shadow.StateValue{},
		shadow.StateValue{"hi": "you", "hello": "world", "test": map[string]any{"aa": 23}},
		[]string{},
		[]string{},
	},
	{
		"merge with nil",
		shadow.StateValue{},
		shadow.StateValue{"test": map[string]any{"aa": "xx", "bb": nil}},
		shadow.StateValue{"test": map[string]any{"aa": "xx"}},
		[]string{},
		[]string{},
	},
	{
		"merge with nil nested",
		shadow.StateValue{},
		shadow.StateValue{"test": map[string]any{"aa": "xx", "bb": map[string]any{"cc": nil}}},
		shadow.StateValue{"test": map[string]any{"aa": "xx"}},
		[]string{},
		[]string{},
	},
}

func TestDeepCopyMap(t *testing.T) {
	cases := []map[string]any{
		{
			"a": nil,
			"b": struct {
				c string
			}{c: "ss"},
		},
		{
			"a": "a",
			"b": 1,
			"c": 1.3,
			"d": true,
			"e": []string{"1", "2"},
			"f": map[string]any{
				"x": 1,
				"y": "y",
			},
		},
	}
	for _, c := range cases {
		r := shadow.DeepCopyMap(c)
		require.Equal(t, c, r)
	}
}

func TestMergeState(t *testing.T) {
	for i, c := range cases {
		var meta shadow.MetaValue
		var updatedMeta shadow.MetaValue
		log.Debugf("====> in case: %s %d", c.name, i)
		log.Debugf("update state origin=%#v", c.target)

		tgt := shadow.StateValue(shadow.DeepCopyMap(c.target))
		shadow.MergeState(&tgt, c.src, &meta, &updatedMeta)
		log.Debugf("target=%#v source=%#v meta=%#v", tgt, c.src, meta)
		require.Equal(t, c.want, tgt, "target=%#v source=%#v meta=%#v", tgt, c.src, meta)

		for _, k := range c.keysUpdated {
			assertMeta(t, k, meta)
			assertMeta(t, k, updatedMeta)
		}
		for _, k := range c.keysRemoved {
			_, ok := meta[k]
			require.Falsef(t, ok, "metadata key %s should be removed", k)
		}
	}
}

func BenchmarkMergeState(b *testing.B) {
	cl := len(cases)
	for i := 0; i < b.N; i++ {
		idx := i % cl
		c := cases[idx]
		var meta shadow.MetaValue
		var updatedMeta shadow.MetaValue
		shadow.MergeState(&c.target, c.src, &meta, &updatedMeta)
	}
}

func assertMeta(t *testing.T, k string, meta shadow.MetaValue) {
	mv, ok := shadow.ValueByPath(meta, k)
	require.Truef(t, ok, "no metadata for key %s", k)
	tsMeta, ok := mv.(map[string]any)
	require.Truef(t, ok, "metadata for key %s is not valid: %#v", k, tsMeta)
	require.Truef(t, ok, "should have meta for key %v ", k)
	ts, ok := tsMeta["timestamp"].(int64)
	require.True(t, ok, "should have timestamp metadata for key %q: %#v", k, tsMeta["timestamp"])
	_ = time.Now().UnixMilli() - ts
	// require.True(t, tsDelta >= 0 && tsDelta < 10, "Time should be closer to the current time, but got %d", tsDelta)
}

func TestStateUnmarshal(t *testing.T) {
	cases := []string{
		`
		{ "a" : {
			"b": {
				"c": 1,
				"d": "d"
			}
		 }
		}
	`,
		`
		{"a" : {
			"b": {
				"c": "c",
				"d": "d"
			}
		 }
		}
	`,
	}
	for i, c := range cases {
		var s shadow.StateValue
		_ = json.Unmarshal([]byte(c), &s)
		log.Debugf("====> in case: %d, %#v", i, s)
	}
}

func TestMergeState_MetaDelete(t *testing.T) {
	nowMs := shadow.MetaTimestamp{Timestamp: time.Now().UnixMilli()}
	cases := []struct {
		target     shadow.StateValue
		src        shadow.StateValue
		meta       shadow.MetaValue
		metaRemove []string
	}{
		{
			shadow.StateValue{
				"hi": "you", "hello": "world",
				"test": map[string]any{"aa": map[string]any{"bb": "bbValue"}},
			},
			shadow.StateValue{
				"hi": nil, "hello": nil,
				"test": map[string]any{"aa": map[string]any{"bb": nil}}},
			shadow.MetaValue{
				"hi":    nowMs,
				"hello": nowMs,
				"test":  map[string]any{"aa": map[string]any{"bb": nowMs}},
			},
			[]string{"hi", "hello", "test.aa.bb"},
		},
		{
			shadow.StateValue{
				"hi": "you", "hello": "world",
				"test": map[string]any{"aa": map[string]any{"bb": "bbValue"}},
			},
			shadow.StateValue{"hi": nil, "hello": nil,
				"test": map[string]any{"aa": nil}},
			shadow.MetaValue{
				"hi":    nowMs,
				"hello": nowMs,
				"test":  map[string]any{"aa": map[string]any{"bb": nowMs}},
			},
			[]string{"test.aa.bb"},
		},
	}
	for i, c := range cases {
		log.Debugf("====> in case: %d", i)
		var meta = c.meta
		var updatedMeta shadow.MetaValue
		shadow.MergeState(&c.target, c.src, &meta, &updatedMeta)
		for _, k := range c.metaRemove {
			mv, ok := shadow.ValueByPath(c.meta, k)
			require.Falsef(t, ok, "should deleted metadata for key %q : %#v", k, mv)
		}
	}
}

func TestMerge_DeltaState(t *testing.T) {
	cases := []struct {
		desired     string
		reported    string
		desiredMeta string
		delta       string
		deltaMeta   string
	}{
		{
			desired: `{
				"a": 1,
				"b": { "c": "c" },
				"d": { "e": 2 },
				"f": {
					"g": { "h": true }
				}
			}`,
			reported: `{
				"a": { "a2": 3 },
				"b": { "c": "cc" },
				"d": { "e": 2 },
				"f": {
					"g": {
						"h": true,
						"i": 3
					}
				}
			}`,
			desiredMeta: `{
				"a": { "timestamp": 1665555014139 },
				"b": {
					"c": {"timestamp": 1665555014144 }
				},
				"d": {
					"e": {"timestamp": 1665555014140}
				},
				"f": {
					"g": {
						"h": {"timestamp": 1665555014139}
					}
				}
			}`,
			delta: `{
				"a": 1,
				"b": { "c": "c" }
			}`,
			deltaMeta: `{
				"a": {"timestamp": 1665555014139},
				"b": {
					"c": {"timestamp": 1665555014144}
				}
			}`,
		},

		// Array field test. An array is treated as a field
		{
			desired: `{
				"arr1": [1,2,3],
				"arr2": [{"a": 1}, {"b": 2}],
				"arr3": ["a", "b", "c"]
			}`,
			reported: `{
				"arr": [1,2,4],
				"arr2": [{"a": 1}, {"b": 3}],
				"arr3": ["a", "b", "c"]
			}`,
			desiredMeta: `{
				"arr1": { "timestamp": 1665555014139 },
				"arr2": {"timestamp": 1665555014220},
				"arr3": {"timestamp": 1665555023110}
			}`,
			delta: `{
					"arr1": [1,2,3],
					"arr2": [{"a": 1}, {"b": 2}]
			}`,
			deltaMeta: `{
				"arr1": {"timestamp": 1665555014139},
				"arr2": {"timestamp": 1665555014220}
			}`,
		},
	}

	s2m := func(s string) map[string]any {
		var m map[string]any
		err := json.Unmarshal([]byte(s), &m)
		require.NoError(t, err, "Error unmarshalling: %s, err: %v", s, err)
		return m
	}

	for _, c := range cases {
		d, m := shadow.DeltaState(s2m(c.desired), s2m(c.reported), s2m(c.desiredMeta))
		require.Equal(t, s2m(c.delta), map[string]any(d), "delta state mismatch")
		require.Equal(t, s2m(c.deltaMeta), map[string]any(m), "delta meta mismatch")
	}
}
