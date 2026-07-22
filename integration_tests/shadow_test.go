//go:build integration

package integration_tests

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
	rest "ruff.io/tio/pkg/restapi"
	"ruff.io/tio/shadow"
)

func TestShadowSetDesired(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	thingId := ID()
	methodBody := strings.NewReader(`{
		"clientToken": "test-token",
		"state": {
			"desired": {
				"color": "red-for-set-desired"
			}
		}
	}`)

	thingClient := newThingClient(ctx, thingId, t)
	err := thingClient.Subscribe(shadow.TopicDeltaStateOf(thingId), 0, func(c mqtt.Client, m mqtt.Message) {
		var req shadow.DeltaStateNotice
		err := json.Unmarshal(m.Payload(), &req)
		require.NoError(t, err, "device unable to unmarshal delta state")
		slog.Debug("device receive delta state", "req", req)
		require.Equal(t, req.State["color"], "red-for-set-desired", "delta state is not valid")
	})
	require.NoError(t, err)
	err = thingClient.Subscribe(shadow.TopicStateUpdatedOf(thingId), 0, func(c mqtt.Client, m mqtt.Message) {
		var req shadow.StateUpdatedNotice
		err := json.Unmarshal(m.Payload(), &req)
		require.NoError(t, err, "device unable to unmarshal state update notice")
		slog.Debug("device receive state update notice", "req", req)
		require.Equal(t, req.Current.State.Desired["color"], "red-for-set-desired", "state update notice is not valid")
		require.Equal(t, req.Previous.State.Desired["color"], nil, "state update notice is not valid")
	})
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/v1/things/%s/shadows/default/state/desired", httpSvr.URL, thingId), methodBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpSvr.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "set shadow desired response status error")
	var respBody rest.Resp[any]
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err, "can not decode shadow desired response body")
	slog.Info("shadow desired body", "body", respBody)
	require.Equal(t, respBody.Code, http.StatusOK, "set shadow desired response code error")

	thingClient.Disconnect()
	cancel()
}

func TestShadowSetReported(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	thingId := ID()
	stateReq := shadow.StateReq{
		ClientToken: fmt.Sprintf("tk-%d", time.Now().UnixNano()),
		State: shadow.StateDR{
			Reported: shadow.StateValue{"color": "red-for-set-reported"},
		},
	}
	stateReqBytes, _ := json.Marshal(stateReq)

	thingClient := newThingClient(ctx, thingId, t)
	err := thingClient.Subscribe(shadow.TopicDeltaStateOf(thingId), 0, func(c mqtt.Client, m mqtt.Message) {
		var n shadow.DeltaStateNotice
		err := json.Unmarshal(m.Payload(), &n)
		require.NoError(t, err, "device unable to unmarshal delta state")
		slog.Debug("device receive delta state", "n", n)
	})
	require.NoError(t, err)
	err = thingClient.Subscribe(shadow.TopicStateUpdatedOf(thingId), 0, func(c mqtt.Client, m mqtt.Message) {
		var n shadow.StateUpdatedNotice
		err := json.Unmarshal(m.Payload(), &n)
		require.NoError(t, err, "device unable to unmarshal state update notice")
		slog.Debug("device received state update notice", "n", n, "payload", string(m.Payload()))
		require.Equal(t, stateReq.State.Reported["color"], n.Current.State.Reported["color"],
			"state update notice is not valid")
		require.Equal(t, nil, n.Previous.State.Reported["color"],
			"state update notice is not valid")
	})
	require.NoError(t, err)

	err = thingClient.Subscribe(shadow.TopicUpdateAcceptedOf(thingId), 1, func(c mqtt.Client, m mqtt.Message) {
		var resp shadow.StateAcceptedResp
		err := json.Unmarshal(m.Payload(), &resp)
		require.NoError(t, err, "device unable to unmarshal accepted message")
		slog.Debug("device received state update accepted message", "resp", resp)
		require.Equal(t, stateReq.ClientToken, resp.ClientToken, "client token mismatch")
	})
	require.NoError(t, err)

	err = thingClient.Publish(shadow.TopicUpdateOf(thingId), 1, false, stateReqBytes)
	require.NoError(t, err)

	thingClient.Disconnect()
	cancel()
}
