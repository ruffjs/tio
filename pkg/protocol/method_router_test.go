package protocol

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	connmock "ruff.io/tio/connector/mock"
	"ruff.io/tio/pkg/codec"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
)

type stubLegacyHandler struct {
	called bool
	resp   shadow.MethodResp
	err    error
}

func (s *stubLegacyHandler) InitMethodHandler(context.Context) error { return nil }

func (s *stubLegacyHandler) InvokeMethod(context.Context, shadow.MethodReqMsg) (shadow.MethodResp, error) {
	s.called = true
	return s.resp, s.err
}

func TestMethodRouterDispatch(t *testing.T) {
	ctx := t.Context()
	mockConn := connmock.NewMockConnector()
	require.NoError(t, mockConn.Start(ctx))
	c, err := codec.New("json")
	require.NoError(t, err)
	profiles := staticProfiles{
		"thing-legacy": {MessageProtocol: "legacy", PayloadEncoding: "json"},
		"thing-simple": {MessageProtocol: "simple", PayloadEncoding: "json"},
	}
	simple, err := NewSimpleHandler(mockConn, c, newTestShadowSvc(), profiles)
	require.NoError(t, err)
	require.NoError(t, simple.Start(ctx))
	defer simple.Stop()

	legacy := &stubLegacyHandler{resp: shadow.MethodResp{Code: 201, Message: "legacy"}}
	router := &MethodRouter{Profiles: profiles, Legacy: legacy, Simple: simple}

	resp, err := router.InvokeMethod(ctx, shadow.MethodReqMsg{ThingId: "thing-legacy", Method: "echo", RespTimeout: 1})
	require.NoError(t, err)
	require.Equal(t, 201, resp.Code)
	require.True(t, legacy.called)

	legacy.called = false
	_, err = router.InvokeMethod(ctx, shadow.MethodReqMsg{ThingId: "thing-simple", Method: "echo", RespTimeout: 1})
	require.ErrorIs(t, err, model.ErrDirectMethodThingOffline)
	require.False(t, legacy.called)

	_, err = router.InvokeMethod(ctx, shadow.MethodReqMsg{ThingId: "thing-missing", Method: "echo", RespTimeout: 1})
	require.ErrorIs(t, err, model.ErrNotFound)
}
