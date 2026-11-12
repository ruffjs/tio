package uuid

import (
	"testing"
	stduuid "uuid"

	"github.com/stretchr/testify/require"
)

func TestIDGeneratesUUIDv7(t *testing.T) {
	provider := New()
	id, err := provider.ID()
	require.NoError(t, err)
	parsed, err := stduuid.Parse(id)
	require.NoError(t, err)
	require.Equal(t, byte(7), parsed[6]>>4)
	require.Equal(t, parsed.String(), id)
	next, err := provider.ID()
	require.NoError(t, err)
	require.NotEqual(t, id, next)
}
