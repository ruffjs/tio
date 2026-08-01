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

	mq "ruff.io/tio/internal/mqtttest"
	"ruff.io/tio/shadow"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
	rest "ruff.io/tio/pkg/restapi"
	shadowApi "ruff.io/tio/shadow/api"
)

func newThingClient(ctx context.Context, thingId string, t *testing.T) *mq.DeviceClient {
	th := crateThing(thingId)
	mqClient := newThingMqttClient(ctx, th.Id, th.AuthValue)

	err := mqClient.Connect(ctx)
	require.NoError(t, err)
	return mqClient
}

func TestMethodInvoke(t *testing.T) {
	skipIfNotLegacy(t)
	ctx, cancel := context.WithCancel(context.Background())
	thingId := ID()
	methodName := "hello"

	thingClient := newThingClient(ctx, thingId, t)
	go func() {
		_ = thingClient.Subscribe(shadow.TopicMethodRequest(thingId, methodName), 0, func(c mqtt.Client, m mqtt.Message) {
			var req shadow.MethodReq
			err := testCodec.Unmarshal(m.Payload(), &req)
			require.NoError(t, err, "device unable to unmarshal method request")
			slog.Debug("device receive method request", "request", req)
			resp := shadow.MethodResp{
				ClientToken: req.ClientToken,
				Data:        req.Data,
				Message:     "OK from device",
				Code:        200,
			}
			b, _ := testCodec.Marshal(resp)
			pubErr := thingClient.Publish(shadow.TopicMethodResponse(thingId, methodName), 0, false, b)
			require.NoError(t, pubErr, "device unable to publish method response")
		})
	}()

	methodBody := strings.NewReader(`{
		"respTimeout": 1,
		"data": {
			"hello": "world"
		}
	}`)
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/v1/things/%s/methods/%s", httpSvr.URL, thingId, methodName), methodBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpSvr.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "method invoke response status error")
	var respBody rest.Resp[shadowApi.MethodInvokeResp]
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err, "can not decode method response body")
	slog.Info("method response body", "response", respBody)
	require.Equal(t, http.StatusOK, respBody.Code, "method invoke response status error")
	respData := respBody.Data.Data.(map[string]interface{})
	require.Equal(t, "world", respData["hello"])

	thingClient.Disconnect()
	cancel()
}
