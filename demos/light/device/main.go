package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"ruff.io/tio/shadow"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"ruff.io/tio/internal/mqtttest"
)

var (
	thingId  = "example"
	password = "example"

	lightState = map[string]any{
		"brightness": 0,
		"power":      "off",
		"voltage":    0,
	}

	conf = map[string]any{
		"sunriseTime": "6:00",
		"sunsetTime":  "18:00",
	}
)

var mqttClient *mqtttest.DeviceClient
var ctx context.Context

func main() {
	ctx = context.Background()

	// Connect
	connectTioByMqtt()

	// Subscribe shadow topics and direct method invoke topic
	receiveShadowGetResp()
	receiveShadowUpdateResp()
	receiveShadowDeltaNotice()
	receiveDirectMethodInvoke()

	// Report the current state of light when it boot, then server can get it's state by query Shadow
	updateShadowReported(lightState)

	// Some indicators should be reported regularly, for monitoring, statistics, alert, etc.
	regularlyReportState()

	select {}
}

func connectTioByMqtt() {
	ctx := context.Background()
	mqttClient = mqtttest.NewDeviceClient("tcp://localhost:1883", thingId, thingId, password)
	err := mqttClient.Connect(ctx)
	if err != nil {
		log.Fatalf("mqtt connect error: %v", err)
	}
}

func receiveShadowGetResp() {
	slog.Info("[Receive Shadow Get Response] subscribe")

	topicReq := fmt.Sprintf("$iothub/things/%s/shadows/name/default/get/+", thingId)
	accepted := "accepted"
	rejected := "rejected"

	err := mqttClient.Subscribe(topicReq, 0, func(c mqtt.Client, m mqtt.Message) {
		go func() {
			var acceptedResp shadow.StateAcceptedResp
			var rejectedResp shadow.ErrResp

			if strings.HasSuffix(m.Topic(), accepted) {
				_ = json.Unmarshal(m.Payload(), &acceptedResp)
				slog.Info("[Receive Shadow Get Response] get accepted", "response", toJsonStr(acceptedResp))
				if len(acceptedResp.State.Delta) > 0 {
					doControlOrConfigByDelta(acceptedResp.State.Delta)
					updateShadowReported(lightState)
				}
			}

			if strings.HasSuffix(m.Topic(), rejected) {
				_ = json.Unmarshal(m.Payload(), &rejectedResp)
				slog.Error("[Receive Shadow Get Response] get rejected", "code", rejectedResp.Code, "message", rejectedResp.Message)
			}

		}()
	})

	if err != nil {
		log.Fatalf("mqtt subscribe error: %v", err)
	}
}

func receiveShadowUpdateResp() {
	slog.Info("[Receive Shadow Update Response] subscribe")

	topicReq := fmt.Sprintf("$iothub/things/%s/shadows/name/default/update/+", thingId)
	accepted := "accepted"
	rejected := "rejected"

	err := mqttClient.Subscribe(topicReq, 0, func(c mqtt.Client, m mqtt.Message) {
		go func() {
			var acceptedResp shadow.StateAcceptedResp
			var rejectedResp shadow.ErrResp

			if strings.HasSuffix(m.Topic(), accepted) {
				_ = json.Unmarshal(m.Payload(), &acceptedResp)
				slog.Info("[Receive Shadow Update Response] update accepted", "response", toJsonStr(acceptedResp))
			}

			if strings.HasSuffix(m.Topic(), rejected) {
				_ = json.Unmarshal(m.Payload(), &rejectedResp)
				slog.Error("[Receive Shadow Update Response] update rejected", "code", rejectedResp.Code, "message", rejectedResp.Message)
			}

		}()
	})

	if err != nil {
		log.Fatalf("mqtt subscribe error: %v", err)
	}
}

// updateShadowReported Report device state by update `Shadow desired`
func updateShadowReported(payload map[string]any) {
	slog.Info("[LightState] Report shadow desired: power: %s, brightness: %v",
		lightState["power"], lightState["brightness"])

	r := shadow.StateReq{
		ClientToken: fmt.Sprintf("tk-%d", time.Now().UnixMicro()),
		State:       shadow.StateDR{Reported: payload},
	}
	reqJson, _ := json.Marshal(r)
	slog.Info("[Set Shadow Reported]", "request", toJsonStr(r))
	topic := fmt.Sprintf("$iothub/things/%s/shadows/name/default/update", thingId)
	mqttClient.Publish(topic, 1, false, reqJson)
}

// receiveShadowDeltaNotice Receive shadow delta notify for device control and configuration
func receiveShadowDeltaNotice() {
	topic := fmt.Sprintf("$iothub/things/%s/shadows/name/default/update/delta", thingId)

	err := mqttClient.Subscribe(topic, 0, func(c mqtt.Client, m mqtt.Message) {
		go func() {
			var deltaNotice shadow.DeltaStateNotice
			err := json.Unmarshal(m.Payload(), &deltaNotice)
			if err != nil {
				slog.Error("Invalid message payload for method response")
				return
			}
			slog.Info("[Receive Shadow Delta] receive", "deltaNotice", toJsonStr(deltaNotice))
			doControlOrConfigByDelta(deltaNotice.State)
			updateShadowReported(lightState)
		}()
	})

	if err != nil {
		log.Fatalf("mqtt subscribe error: %v", err)
	}
}

// receiveDirectMethodInvoke
//  1. subscribe the method request topic
//  2. do the method action when receive method request
//  3. send response like a http response
func receiveDirectMethodInvoke() {
	slog.Info("[Receive Method Request] subscribe method request: make the light flash once")

	topicReq := fmt.Sprintf("$iothub/things/%s/methods/%s/req", thingId, "flash")
	topicResp := fmt.Sprintf("$iothub/things/%s/methods/%s/resp", thingId, "flash")

	slog.Info("=== subscribe", "topicReq", topicReq, "topicResp", topicResp)

	err := mqttClient.Subscribe(topicReq, 0, func(c mqtt.Client, m mqtt.Message) {
		go func() {
			var req shadow.MethodReq
			var resp shadow.MethodResp
			err := json.Unmarshal(m.Payload(), &req)
			if err == nil {
				if m, ok := req.Data.(map[string]any); ok {
					if times, ok := m["times"]; ok {
						c := int(times.(float64))
						slog.Info("[Receive Method Request]", "req", toJsonStr(req))
						slog.Info("[Receive Method Request] flash light", "times", c)

						// Do the flash light action
						flashLight(c)
						resp = shadow.MethodResp{
							ClientToken: req.ClientToken, // must be the same clientToken for tio mapping request and response
							Data:        fmt.Sprintf("light flash %d times", c),
							Message:     "OK from device",
							Code:        200,
						}
					} else {
						resp = shadow.MethodResp{
							ClientToken: req.ClientToken,
							Data:        nil,
							Message:     fmt.Sprintf("wrong request body: %#v", req),
							Code:        400,
						}
					}
				}
			} else {
				slog.Error("[Receive Method Request] device unable to unmarshal method request body", "payload", m.Payload())
				resp = shadow.MethodResp{
					ClientToken: req.ClientToken,
					Data:        nil,
					Message:     fmt.Sprintf("wrong request body: %s", err),
					Code:        400,
				}
			}

			b, _ := json.Marshal(resp)
			mqttClient.Publish(topicResp, 0, false, b)
		}()
	})

	if err != nil {
		log.Fatalf("mqtt subscribe error: %v", err)
	}
}

func regularlyReportState() {
	topic := fmt.Sprintf("$iothub/things/%s/messages/property", thingId)

	go func() {
		for {
			time.Sleep(time.Minute)
			// mock for some state change
			lightState["voltage"] = rand.Intn(30-6) + 5

			// report
			data, _ := json.Marshal(map[string]any{"power": lightState["power"], "voltage": lightState["voltage"]})
			err := mqttClient.Publish(topic, 1, false, data)
			if err != nil {
				slog.Error("[Report Property] error", "error", err)
			} else {
				slog.Info("[Report Property]", "topic", topic, "data", data)
			}
		}
	}()
}

func doControlOrConfigByDelta(shadowDelta map[string]any) {
	for k, v := range shadowDelta {
		if lightState[k] != nil {
			switch k {

			// Control light
			case "brightness":
				// Adjust the brightness of the light
				slog.Info("[Receive Shadow Delta] adjust brightness to", "value", v)
				// Record the state of the light
				lightState[k] = v
			case "power":
				// Control light on/off
				if v == "on" {
					slog.Info("[Receive Shadow Delta] turn on light")
				} else {
					slog.Info("[Receive Shadow Delta] turn off light")
				}
				// Record the state of the light
				lightState[k] = v

			// Conifg light
			case "sunriseTime", "sunsetTime":
				conf[k] = v

			// OTA
			// Also can be other field name as you want
			case "firmware":

			// The OTA process can also be achieved by Shadow, You can try to write the code. The basic idea is:
			//  1. The server sets the `Shadow Desired` field "firmware" by http api to
			//     { "version": "1.2.3", "packageFile": "http://demo.com/pkg_v1.2.3" }
			//  2. When the device receives a Shadow Delta notifying
			// 		 that the value of this field has been updated,
			// 		 it pulls the firmware from the `packageFile` address,
			// 		 updates, and reports the progress to the `Shadow Reported` field "firmware" : {"processPercent": 20 }
			//  3. When the device has successfully or unsuccessfully upgrade,
			//     it reports the result to the `Shadow Reported` "firmware" field, like:
			//     {"version": "1.2.3", "packageFile": "http://demo.com/pkg_v1.2.3", "result": "done"}
			//  4. In this process, the server and the upper business system can query
			//     the current status and results of the OTA task through Shadow

			default:
				slog.Info("[Receive Shadow Delta] shadow delta field", "field", k)
			}
		} else {
			slog.Error("[Receive Shadow Delta] unkown shadow delta field", "field", k)
		}
	}
}

func flashLight(times int) {
	if times <= 0 {
		return
	}

	toggle := func() {
		if lightState["power"] == "off" {
			slog.Info("[Light State] on")
			lightState["power"] = "on"
		} else {
			slog.Info("[Light State] off")
			lightState["power"] = "off"
		}
	}
	for i := 0; i < times; i++ {
		toggle()
		time.Sleep(time.Second)
		toggle()
	}
}

func toJsonStr(v any) string {
	s, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalf("Marshal value %v error %v", v, err)
	}
	return string(s)
}
