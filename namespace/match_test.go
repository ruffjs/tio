package namespace_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/config"
	"ruff.io/tio/namespace"
	"ruff.io/tio/shadow"
)

func TestMatchNamespaces(t *testing.T) {
	namespaces := []config.Namespace{
		{Name: "factory-a", Tags: map[string]string{"factory": "a"}},
		{Name: "meter-a", Tags: map[string]string{"factory": "a", "product": "meter"}},
		{Name: "invalid/name", Tags: map[string]string{"factory": "a"}},
	}

	got := namespace.MatchNamespaces(namespaces, shadow.TagsValue{
		"factory": "a",
		"product": "meter",
	})

	require.ElementsMatch(t, []string{"factory-a", "meter-a"}, got)
}

func TestNamespaceForUser(t *testing.T) {
	namespaces := []config.Namespace{{
		Name: "biz",
		Users: []config.UserPassword{{
			Name:     "$biz",
			Password: "secret",
		}},
	}}

	ns, ok := namespace.NamespaceForUser(namespaces, "$biz", "secret")
	require.True(t, ok)
	require.Equal(t, "biz", ns)

	_, ok = namespace.NamespaceForUser(namespaces, "$biz", "wrong")
	require.False(t, ok)
}
