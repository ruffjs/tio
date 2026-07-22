package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/internal/mqtttest"
	"ruff.io/tio/shadow"
	"ruff.io/tio/thing/api"
)

const (
	testHTTPUrl      = "http://127.0.0.1:9000"
	testHTTPUser     = "admin"
	testHTTPPassword = "public"
	testThingId      = "test-light-demo"
	testThingPass    = "test-light-demo"
	testMqttBroker   = "tcp://localhost:1883"
	testBizUser      = "$biz"
	testBizPassword  = "public"
)

func TestLightDemo(t *testing.T) {
	ctx := context.Background()

	// Cleanup first
	deleteThing(t, testThingId)

	// Step 1: Create thing
	t.Run("Step1_CreateThing", func(t *testing.T) {
		createThing(t, testThingId, testThingPass)
	})

	var serverClient *mqtttest.DeviceClient
	var deviceClient *mqtttest.DeviceClient

	// Step 2: Server connects as $biz
	t.Run("Step2_ServerConnect", func(t *testing.T) {
		serverClient = connectServer(t, ctx)
	})

	// Step 3: Device connects as thing
	t.Run("Step3_DeviceConnect", func(t *testing.T) {
		deviceClient = connectDevice(t, ctx)
	})

	// Step 4: Server subscribes to presence and properties BEFORE device reports
	t.Run("Step4_ServerSubscribeTopics", func(t *testing.T) {
		subscribePresence(t, serverClient)
		subscribeProperties(t, serverClient)
		time.Sleep(200 * time.Millisecond)
	})

	// Step 5: Device subscribes to delta and method BEFORE server triggers them
	t.Run("Step5_DeviceSubscribeTopics", func(t *testing.T) {
		subscribeDeviceTopics(t, deviceClient)
		time.Sleep(200 * time.Millisecond)
	})

	// Step 6: Device reports shadow state
	t.Run("Step6_DeviceReportShadow", func(t *testing.T) {
		reportShadow(t, deviceClient)
		time.Sleep(500 * time.Millisecond)
	})

	// Step 7: Server sets brightness via shadow desired
	t.Run("Step7_ModifyBrightnessByShadow", func(t *testing.T) {
		setBrightnessByShadow(t, 75)
		time.Sleep(1 * time.Second)
	})

	// Step 8: Verify device receives delta
	t.Run("Step8_VerifyDeltaReceived", func(t *testing.T) {
		verifyDeltaReceived(t)
	})

	// Step 9: Server invokes flash method
	t.Run("Step9_FlashLightByDirectMethod", func(t *testing.T) {
		flashLightByDirectMethod(t, 2)
		time.Sleep(1 * time.Second)
	})

	// Step 10: Verify device receives method request
	t.Run("Step10_VerifyMethodReceived", func(t *testing.T) {
		verifyMethodReceived(t)
	})

	// Step 11: Device reports property
	t.Run("Step11_DeviceReportsProperty", func(t *testing.T) {
		reportProperty(t, deviceClient)
		time.Sleep(1 * time.Second)
	})

	// Step 12: Verify server receives property
	t.Run("Step12_VerifyPropertyReceived", func(t *testing.T) {
		verifyPropertyReceived(t)
	})

	// Step 13: Cleanup
	t.Run("Step13_Cleanup", func(t *testing.T) {
		if deviceClient != nil {
			deviceClient.Disconnect()
		}
		if serverClient != nil {
			serverClient.Disconnect()
		}
		deleteThing(t, testThingId)
	})
}

var (
	deltaReceived    = make(chan shadow.DeltaStateNotice, 10)
	methodReceived   = make(chan shadow.MethodReq, 10)
	propertyReceived = make(chan string, 10)
)

func createThing(t *testing.T, thingId, password string) {
	createThReq := api.CreateReq{ThingId: thingId, Password: password}
	b, _ := json.Marshal(createThReq)

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/things", testHTTPUrl), bytes.NewBuffer(b))
	req.SetBasicAuth(testHTTPUser, testHTTPPassword)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "create thing failed: %s", string(body))
}

func deleteThing(t *testing.T, thingId string) {
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/things/%s", testHTTPUrl, thingId), nil)
	req.SetBasicAuth(testHTTPUser, testHTTPPassword)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, _ := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
}

func connectServer(t *testing.T, ctx context.Context) *mqtttest.DeviceClient {
	clientId := fmt.Sprintf("%s-%d", testBizUser, time.Now().UnixNano()%1000)
	c := mqtttest.NewDeviceClient(testMqttBroker, clientId, testBizUser, testBizPassword)
	err := c.Connect(ctx)
	require.NoError(t, err)
	return c
}

func connectDevice(t *testing.T, ctx context.Context) *mqtttest.DeviceClient {
	c := mqtttest.NewDeviceClient(testMqttBroker, testThingId, testThingId, testThingPass)
	err := c.Connect(ctx)
	require.NoError(t, err)
	return c
}

func subscribePresence(t *testing.T, client *mqtttest.DeviceClient) {
	err := client.Subscribe("$iothub/things/+/presence", 0, func(c mqtt.Client, m mqtt.Message) {
	})
	require.NoError(t, err)
}

func subscribeProperties(t *testing.T, client *mqtttest.DeviceClient) {
	err := client.Subscribe("$iothub/things/+/messages/property", 0, func(c mqtt.Client, m mqtt.Message) {
		if strings.Contains(m.Topic(), testThingId) {
			select {
			case propertyReceived <- string(m.Payload()):
			default:
			}
		}
	})
	require.NoError(t, err)
}

func subscribeDeviceTopics(t *testing.T, client *mqtttest.DeviceClient) {
	deltaTopic := fmt.Sprintf("$iothub/things/%s/shadows/name/default/update/delta", testThingId)
	err := client.Subscribe(deltaTopic, 0, func(c mqtt.Client, m mqtt.Message) {
		var deltaNotice shadow.DeltaStateNotice
		if err := json.Unmarshal(m.Payload(), &deltaNotice); err == nil {
			select {
			case deltaReceived <- deltaNotice:
			default:
			}
		}
	})
	require.NoError(t, err)

	methodTopic := fmt.Sprintf("$iothub/things/%s/methods/%s/req", testThingId, "flash")
	methodRespTopic := fmt.Sprintf("$iothub/things/%s/methods/%s/resp", testThingId, "flash")
	err = client.Subscribe(methodTopic, 0, func(c mqtt.Client, m mqtt.Message) {
		var req shadow.MethodReq
		if err := json.Unmarshal(m.Payload(), &req); err == nil {
			if data, ok := req.Data.(map[string]any); ok {
				if times, ok := data["times"]; ok {
					timesInt := int(times.(float64))
					resp := shadow.MethodResp{
						ClientToken: req.ClientToken,
						Data:        fmt.Sprintf("light flash %d times", timesInt),
						Message:     "OK from test device",
						Code:        200,
					}
					b, _ := json.Marshal(resp)
					client.Publish(methodRespTopic, 0, false, b)
				}
			}
			select {
			case methodReceived <- req:
			default:
			}
		}
	})
	require.NoError(t, err)
}

func reportShadow(t *testing.T, client *mqtttest.DeviceClient) {
	lightState := map[string]any{
		"brightness": 0,
		"power":      "off",
		"voltage":    220,
	}
	r := shadow.StateReq{
		ClientToken: fmt.Sprintf("tk-%d", time.Now().UnixMicro()),
		State:       shadow.StateDR{Reported: lightState},
	}
	reqJson, _ := json.Marshal(r)
	topic := fmt.Sprintf("$iothub/things/%s/shadows/name/default/update", testThingId)
	err := client.Publish(topic, 1, false, reqJson)
	require.NoError(t, err)
}

func setBrightnessByShadow(t *testing.T, brightness int) {
	methodBody := strings.NewReader(
		fmt.Sprintf(`{
		"clientToken": "test-%d",
		"state": {
			"desired": {
				"brightness": %d
			}
		}
	}`, time.Now().UnixMicro(), brightness))

	req, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/v1/things/%s/shadows/default/state/desired", testHTTPUrl, testThingId), methodBody)
	req.SetBasicAuth(testHTTPUser, testHTTPPassword)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "set brightness failed: %s", string(body))
}

func verifyDeltaReceived(t *testing.T) {
	select {
	case delta := <-deltaReceived:
		assert.NotNil(t, delta.State)
		if brightness, ok := delta.State["brightness"]; ok {
			assert.Equal(t, float64(75), brightness)
			t.Logf("Received delta: brightness=%v", brightness)
		}
	case <-time.After(3 * time.Second):
		t.Error("Delta notification not received")
	}
}

func flashLightByDirectMethod(t *testing.T, times int) {
	methodBody := strings.NewReader(fmt.Sprintf(`{
		"respTimeout": 3,
		"data": {
			"times": %d
		}
	}`, times))

	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/v1/things/%s/methods/%s", testHTTPUrl, testThingId, "flash"), methodBody)
	req.SetBasicAuth(testHTTPUser, testHTTPPassword)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "flash light failed: %s", string(body))
}

func verifyMethodReceived(t *testing.T) {
	select {
	case req := <-methodReceived:
		t.Logf("Received method request: clientToken=%s", req.ClientToken)
		if data, ok := req.Data.(map[string]any); ok {
			if times, ok := data["times"]; ok {
				assert.Equal(t, float64(2), times)
			}
		}
	case <-time.After(3 * time.Second):
		t.Error("Method request not received")
	}
}

func reportProperty(t *testing.T, client *mqtttest.DeviceClient) {
	data, _ := json.Marshal(map[string]any{"power": "on", "voltage": 225})
	topic := fmt.Sprintf("$iothub/things/%s/messages/property", testThingId)
	err := client.Publish(topic, 1, false, data)
	require.NoError(t, err)
}

func verifyPropertyReceived(t *testing.T) {
	select {
	case payload := <-propertyReceived:
		t.Logf("Received property: %s", payload)
		var prop map[string]any
		err := json.Unmarshal([]byte(payload), &prop)
		require.NoError(t, err)
		assert.Equal(t, "on", prop["power"])
	case <-time.After(3 * time.Second):
		t.Error("Property report not received")
	}
}
