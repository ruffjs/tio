package job

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func validCreateReq(op string, doc map[string]any) CreateReq {
	return CreateReq{
		Operation:    op,
		JobDoc:       doc,
		TargetConfig: TargetConfig{Type: TargetTypeThingId, Things: []string{"t1"}},
	}
}

func TestCreateReqDirectMethodDoc(t *testing.T) {
	doc := map[string]any{"method": "turnOn"}
	req := validCreateReq(SysOpDirectMethodPrefix+"turnOn", doc)
	require.NoError(t, req.valid())
	require.EqualValues(t, 30, doc["responseTimeout"])

	require.Error(t, validCreateReq(SysOpDirectMethodPrefix+"turnOn", map[string]any{}).valid())
	require.Error(t, validCreateReq(SysOpDirectMethodPrefix+"turnOn",
		map[string]any{"method": "x", "responseTimeout": "abc"}).valid())
	require.Error(t, validCreateReq(SysOpDirectMethodPrefix+"turnOn",
		map[string]any{"method": "x", "responseTimeout": 301}).valid())
}

func TestCreateReqUpdateShadowDoc(t *testing.T) {
	req := validCreateReq(SysOpUpdateShadowPrefix+"cfg",
		map[string]any{"state": map[string]any{"desired": map[string]any{"a": 1}}})
	require.NoError(t, req.valid())

	require.Error(t, validCreateReq(SysOpUpdateShadowPrefix+"cfg", map[string]any{}).valid())
	require.Error(t, validCreateReq(SysOpUpdateShadowPrefix+"cfg",
		map[string]any{"state": map[string]any{}}).valid())
}
