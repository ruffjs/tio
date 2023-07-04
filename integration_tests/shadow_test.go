package integration_tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/pkg/log"
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

	// thing subscribe and response
	thingClient := newThingClient(ctx, thingId, t)
	err := thingClient.Subscribe(ctx, shadow.TopicDeltaStateOf(thingId), 0, func(c mqtt.Client, m mqtt.Message) {
		var req shadow.DeltaStateNotice
		err := json.Unmarshal(m.Payload(), &req)
		require.NoError(t, err, "device unable to unmarshal delta state")
		log.Debugf("device receive delta state: %#v", req)
		require.Equal(t, req.State["color"], "red-for-set-desired", "delta state is not valid")
	})
	require.NoError(t, err)
	err = thingClient.Subscribe(ctx, shadow.TopicStateUpdatedOf(thingId), 0, func(c mqtt.Client, m mqtt.Message) {
		var req shadow.StateUpdatedNotice
		err := json.Unmarshal(m.Payload(), &req)
		require.NoError(t, err, "device unable to unmarshal state update notice")
		log.Debugf("device receive state update notice: %#v", req)
		require.Equal(t, req.Current.State.Desired["color"], "red-for-set-desired", "state update notice is not valid")
		require.Equal(t, req.Previous.State.Desired["color"], nil, "state update notice is not valid")
	})
	require.NoError(t, err)

	// method invoke by http api
	req, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/v1/things/%s/shadows/default/state/desired", httpSvr.URL, thingId), methodBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpSvr.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "set shadow desired response status error")
	var respBody rest.Resp[any]
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err, "can not decode shadow desired response body")
	log.Infof("shadow desired body %#v", respBody)
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

	// thing subscribe and response
	thingClient := newThingClient(ctx, thingId, t)
	err := thingClient.Subscribe(ctx, shadow.TopicDeltaStateOf(thingId), 0, func(c mqtt.Client, m mqtt.Message) {
		var n shadow.DeltaStateNotice
		err := json.Unmarshal(m.Payload(), &n)
		require.NoError(t, err, "device unable to unmarshal delta state")
		log.Debugf("device receive delta state: %#v", n)
	})
	require.NoError(t, err)
	err = thingClient.Subscribe(ctx, shadow.TopicStateUpdatedOf(thingId), 0, func(c mqtt.Client, m mqtt.Message) {
		var n shadow.StateUpdatedNotice
		err := json.Unmarshal(m.Payload(), &n)
		require.NoError(t, err, "device unable to unmarshal state update notice")
		log.Debugf("device received state update notice: %#v, %s", n, string(m.Payload()))
		require.Equal(t, stateReq.State.Reported["color"], n.Current.State.Reported["color"],
			"state update notice is not valid")
		require.Equal(t, nil, n.Previous.State.Reported["color"],
			"state update notice is not valid")
	})
	require.NoError(t, err)

	err = thingClient.Subscribe(ctx, shadow.TopicUpdateAcceptedOf(thingId), 1, func(c mqtt.Client, m mqtt.Message) {
		var resp shadow.StateAcceptedResp
		err := json.Unmarshal(m.Payload(), &resp)
		require.NoError(t, err, "device unable to unmarshal accepted message")
		log.Debugf("device received state update accepted message: %#v", resp)
		require.Equal(t, stateReq.ClientToken, resp.ClientToken, "client token mismatch")
	})
	require.NoError(t, err)

	pubTk := thingClient.Publish(shadow.TopicUpdateOf(thingId), 1, false, stateReqBytes)
	pubTk.Wait()
	require.NoError(t, pubTk.Error())

	thingClient.Disconnect()
	cancel()
}
