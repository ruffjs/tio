package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPageQueryBounds(t *testing.T) {
	q := PageQuery{PageIndex: 0, PageSize: -1}
	require.Equal(t, 10, q.Limit())
	require.Equal(t, 0, q.Offset())
	require.Equal(t, 1, q.PageIndex)
	require.Equal(t, 10, q.PageSize)

	q = PageQuery{PageIndex: 3, PageSize: 20000}
	require.Equal(t, 10000, q.Limit())
	require.Equal(t, 20000, q.Offset())
}
