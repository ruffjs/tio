package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"ruff.io/tio/config"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/shadow"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	mq "ruff.io/tio/connector/mqtt"
	"ruff.io/tio/connector/mqtt/client"
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

var mqttClient client.Client
var ctx context.Context

func main() {
	// glg.Get().SetLevel(glg.INFO)
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
	cfg := config.MqttClientConfig{ClientId: thingId, User: thingId, Password: password, Host: "localhost", Port: 1883}
	mqttClient = client.NewClient(cfg)
	err := mqttClient.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func receiveShadowGetResp() {
	log.Info("[Receive Shadow Get Response] subscribe")

	topicReq := fmt.Sprintf("$iothub/things/%s/shadows/name/default/get/+", thingId)
	accepted := "accepted"
	rejected := "rejected"

	err := mqttClient.Subscribe(ctx, topicReq, 0, func(c mqtt.Client, m mqtt.Message) {
		go func() {
			var acceptedResp shadow.StateAcceptedResp
			var rejectedResp shadow.ErrResp

			// Server accepted shadow get request
			if strings.HasSuffix(m.Topic(), accepted) {
				_ = json.Unmarshal(m.Payload(), &acceptedResp)
				log.Infof("[Receive Shadow Get Response] get accepted: \n%s", toJsonStr(acceptedResp))
				if len(acceptedResp.State.Delta) > 0 {
					doControlOrConfigByDelta(acceptedResp.State.Delta)
					updateShadowReported(lightState)
				}
			}

			if strings.HasSuffix(m.Topic(), rejected) {
				_ = json.Unmarshal(m.Payload(), &rejectedResp)
				log.Errorf("[Receive Shadow Get Response] get rejected, code: %d , msg: %s",
					rejectedResp.Code, rejectedResp.Message)
				// Do something when get shadow rejected by code of the response, eg: try agin
				// ...
			}

		}()
	})

	if err != nil {
		log.Fatalf("mqtt subscribe error: %v", err)
	}
}

func receiveShadowUpdateResp() {
	log.Info("[Receive Shadow Update Response] subscribe")

	topicReq := fmt.Sprintf("$iothub/things/%s/shadows/name/default/update/+", thingId)
	accepted := "accepted"
	rejected := "rejected"

	err := mqttClient.Subscribe(ctx, topicReq, 0, func(c mqtt.Client, m mqtt.Message) {
		go func() {
			var acceptedResp shadow.StateAcceptedResp
			var rejectedResp shadow.ErrResp

			// Server accepted shadow get request
			if strings.HasSuffix(m.Topic(), accepted) {
				_ = json.Unmarshal(m.Payload(), &acceptedResp)
				log.Infof("[Receive Shadow Update Response] update accepted: \n%s", toJsonStr(acceptedResp))
			}

			if strings.HasSuffix(m.Topic(), rejected) {
				_ = json.Unmarshal(m.Payload(), &rejectedResp)
				log.Errorf("[Receive Shadow Update Response] update rejected, code: %d , msg: %s",
					rejectedResp.Code, rejectedResp.Message)
				// Do something when update shadow rejected by code of the response, eg: try agin
				// ...
			}

		}()
	})

	if err != nil {
		log.Fatalf("mqtt subscribe error: %v", err)
	}
}

// updateShadowReported Report device state by update `Shadow desired`
func updateShadowReported(payload map[string]any) {
	log.Infof("[LightState] Report shadow desired: power: %s, brightness: %v",
		lightState["power"], lightState["brightness"])

	r := shadow.StateReq{
		ClientToken: fmt.Sprintf("tk-%d", time.Now().UnixMicro()),
		State:       shadow.StateDR{Reported: payload},
	}
	reqJson, _ := json.Marshal(r)
	log.Infof("[Set Shadow Reported] \n%s", toJsonStr(r))
	topic := fmt.Sprintf("$iothub/things/%s/shadows/name/default/update", thingId)
	mqttClient.Publish(topic, mq.DefaultQos, false, reqJson)
}

// receiveShadowDeltaNotice Receive shadow delta notify for device control and configuration
func receiveShadowDeltaNotice() {
	topic := fmt.Sprintf("$iothub/things/%s/shadows/name/default/update/delta", thingId)

	err := mqttClient.Subscribe(ctx, topic, 0, func(c mqtt.Client, m mqtt.Message) {
		go func() {
			var deltaNotice shadow.DeltaStateNotice
			err := json.Unmarshal(m.Payload(), &deltaNotice)
			if err != nil {
				log.Errorf("Invalid message payload for method response")
				return
			}
			log.Infof("[Receive Shadow Delta] receive: %+v", toJsonStr(deltaNotice))
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
	log.Info("[Receive Method Request] subscribe method request: make the light flash once")

	topicReq := fmt.Sprintf("$iothub/things/%s/methods/%s/req", thingId, "flash")
	topicResp := fmt.Sprintf("$iothub/things/%s/methods/%s/resp", thingId, "flash")

	log.Infof("=== %s \n%s", topicReq, topicResp)

	err := mqttClient.Subscribe(ctx, topicReq, 0, func(c mqtt.Client, m mqtt.Message) {
		go func() {
			var req shadow.MethodReq
			var resp shadow.MethodResp
			err := json.Unmarshal(m.Payload(), &req)
			if err == nil {
				if m, ok := req.Data.(map[string]any); ok {
					if times, ok := m["times"]; ok {
						c := int(times.(float64))
						log.Infof("[Receive Method Request] \n%s", toJsonStr(req))
						log.Infof("[Receive Method Request] flash light %d times", c)

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
				log.Errorf("[Receive Method Request] device unable to unmarshal method request body %s", m.Payload())
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
			time.Sleep(3 * time.Second)
			// mock for some state change
			lightState["voltage"] = rand.Intn(30-6) + 5

			// report
			data, _ := json.Marshal(map[string]any{"power": lightState["power"], "voltage": lightState["voltage"]})
			tk := mqttClient.Publish(topic, mq.DefaultQos, false, data)
			tk.Wait()
			if tk.Error() != nil {
				log.Errorf("[Report Property] error: %v", tk.Error())
			} else {
				log.Infof("[Report Property] %s %s", topic, data)
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
				log.Infof("[Receive Shadow Delta] adjust brightness to %v", v)
				// Record the state of the light
				lightState[k] = v
			case "power":
				// Control light on/off
				if v == "on" {
					log.Info("[Receive Shadow Delta] turn on light")
				} else {
					log.Info("[Receive Shadow Delta] turn off light")
				}
				// Record the state of the light
				lightState[k] = v

			// Conifg light
			case "sunriseTime", "sunsetTime":
				conf[k] = v

			default:
				log.Infof("[Receive Shadow Delta] shadow delta field %q", k)
			}
		} else {
			log.Errorf("[Receive Shadow Delta] unkown shadow delta field %q", k)
		}
	}
}

func flashLight(times int) {
	if times <= 0 {
		return
	}

	toggle := func() {
		if lightState["power"] == "off" {
			log.Infof("[Light State] on")
			lightState["power"] = "on"
		} else {
			log.Infof("[Light State] off")
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
