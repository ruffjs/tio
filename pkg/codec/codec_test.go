package codec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCodec(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		c, err := New("json")
		require.NoError(t, err)
		assert.NotNil(t, c)
	})

	t.Run("cbor", func(t *testing.T) {
		c, err := New("cbor")
		require.NoError(t, err)
		assert.NotNil(t, c)
	})

	t.Run("unknown", func(t *testing.T) {
		_, err := New("xml")
		require.Error(t, err)
	})

	t.Run("empty", func(t *testing.T) {
		_, err := New("")
		require.Error(t, err)
	})
}

func TestCodecRoundTrip(t *testing.T) {
	encodings := []string{"json", "cbor"}

	tests := []struct {
		name  string
		value any
	}{
		{"string", "hello"},
		{"int", 42},
		{"negative_int", -7},
		{"uint", uint64(99)},
		{"float", 3.14},
		{"bool_true", true},
		{"bool_false", false},
		{"null", nil},
		{"string_array", []any{"a", "b", "c"}},
		{"mixed_array", []any{1, "two", true, nil}},
		{"nested_map", map[string]any{
			"a": 1,
			"b": map[string]any{
				"c": "deep",
				"d": []any{1, 2, 3},
			},
		}},
		{"empty_map", map[string]any{}},
		{"empty_array", []any{}},
	}

	for _, enc := range encodings {
		t.Run(enc, func(t *testing.T) {
			c, err := New(enc)
			require.NoError(t, err)

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					data, err := c.Marshal(tt.value)
					require.NoError(t, err)

					var got any
					err = c.Unmarshal(data, &got)
					require.NoError(t, err)

					assert.Equal(t, normalize(tt.value), normalize(got))
				})
			}
		})
	}
}

func TestCBORMapDecodesToStringKeyMap(t *testing.T) {
	c, err := New("cbor")
	require.NoError(t, err)

	original := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"key": "value",
			},
		},
	}

	data, err := c.Marshal(original)
	require.NoError(t, err)

	var got map[string]any
	err = c.Unmarshal(data, &got)
	require.NoError(t, err)

	l1, ok := got["level1"].(map[string]any)
	require.True(t, ok, "level1 should be map[string]any, got %T", got["level1"])

	l2, ok := l1["level2"].(map[string]any)
	require.True(t, ok, "level2 should be map[string]any, got %T", l1["level2"])

	assert.Equal(t, "value", l2["key"])
}

func TestCBORNestedMapViaAny(t *testing.T) {
	c, err := New("cbor")
	require.NoError(t, err)

	original := map[string]any{
		"outer": map[string]any{
			"inner": map[string]any{
				"deep": true,
			},
		},
	}

	data, err := c.Marshal(original)
	require.NoError(t, err)

	var got any
	err = c.Unmarshal(data, &got)
	require.NoError(t, err)

	m, ok := got.(map[string]any)
	require.True(t, ok)

	outer, ok := m["outer"].(map[string]any)
	require.True(t, ok, "outer should be map[string]any, got %T", m["outer"])

	inner, ok := outer["inner"].(map[string]any)
	require.True(t, ok, "inner should be map[string]any, got %T", outer["inner"])

	assert.Equal(t, true, inner["deep"])
}

func normalize(v any) any {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int8:
		return float64(val)
	case int16:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	case uint:
		return float64(val)
	case uint8:
		return float64(val)
	case uint16:
		return float64(val)
	case uint32:
		return float64(val)
	case uint64:
		return float64(val)
	case float32:
		return float64(val)
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, v := range val {
			out[k] = normalize(v)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, v := range val {
			out[i] = normalize(v)
		}
		return out
	default:
		return v
	}
}
