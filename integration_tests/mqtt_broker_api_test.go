//go:build integration

package integration_tests

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
	rest "ruff.io/tio/pkg/restapi"
)

func TestKickOutMQTTClient(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	client := mqtt.NewClient(mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("ws://127.0.0.1:%d/mqtt", wsPort())).
		SetClientID("mqtt-" + thingId).
		SetUsername(thingId).
		SetPassword("test-password").
		SetAutoReconnect(false))
	token := client.Connect()
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	t.Cleanup(func() { client.Disconnect(0) })
	waitConnected(t, thingId)

	kick := func(authenticated bool) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodDelete, httpSvr.URL+"/api/v1/mqttBroker/clients/"+thingId, nil)
		require.NoError(t, err)
		if authenticated {
			req.SetBasicAuth(cfg.API.BasicAuth.Name, cfg.API.BasicAuth.Password)
		}
		resp, err := httpSvr.Client().Do(req)
		require.NoError(t, err)
		t.Cleanup(func() { resp.Body.Close() })
		return resp
	}

	require.Equal(t, http.StatusUnauthorized, kick(false).StatusCode)
	require.True(t, client.IsConnectionOpen())

	for range 2 {
		resp := kick(true)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var body rest.Resp[struct {
			Deleted bool `json:"deleted"`
		}]
		require.NoError(t, json.UnmarshalRead(resp.Body, &body))
		require.Equal(t, http.StatusOK, body.Code)
		require.True(t, body.Data.Deleted)
		require.Eventually(t, func() bool { return !client.IsConnectionOpen() }, 5*time.Second, 50*time.Millisecond)
		waitDisconnected(t, thingId)
	}
}
