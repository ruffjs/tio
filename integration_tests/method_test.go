package integration_tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	mq "ruff.io/tio/connector/mqtt/client"
	"ruff.io/tio/shadow"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/pkg/log"
	rest "ruff.io/tio/pkg/restapi"
	shadowApi "ruff.io/tio/shadow/api"
)

func newThingClient(ctx context.Context, thingId string, t *testing.T) mq.Client {
	th := crateThing(thingId)
	mqClient := newThingMqttClient(ctx, th.Id, th.AuthValue)

	err := mqClient.Connect(ctx)
	require.NoError(t, err)
	return mqClient
}

func TestMethodInvoke(t *testing.T) {
	// t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	thingId := ID()
	methodName := "hello"

	// thing subscribe and response
	thingClient := newThingClient(ctx, thingId, t)
	go func() {
		_ = thingClient.Subscribe(ctx, shadow.TopicMethodRequest(thingId, methodName), 0, func(c mqtt.Client, m mqtt.Message) {
			var req shadow.MethodReq
			err := json.Unmarshal(m.Payload(), &req)
			require.NoError(t, err, "device unable to unmarshal method request")
			log.Debugf("device receive method request: %#v", req)
			resp := shadow.MethodResp{
				ClientToken: req.ClientToken,
				Data:        req.Data,
				Message:     "OK from device",
				Code:        200,
			}
			b, _ := json.Marshal(resp)
			tk := thingClient.Publish(shadow.TopicMethodResponse(thingId, methodName), 0, false, b)
			tk.Wait()
			require.NoError(t, tk.Error(), "device unable to publish method response")
		})
	}()

	// method invoke by http api
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
	log.Infof("method response body %#v", respBody)
	require.Equal(t, http.StatusOK, respBody.Code, "method invoke response status error")
	respData := respBody.Data.Data.(map[string]interface{})
	require.Equal(t, "world", respData["hello"])

	thingClient.Disconnect()
	cancel()
}
