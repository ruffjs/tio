package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/manifoldco/promptui"
	"ruff.io/tio/internal/mqtttest"
	"ruff.io/tio/thing/api"
)

var (
	httpUrl  = "http://127.0.0.1:9000"
	userName = "admin"

	password = "public"
	thingId  = "example"

	serverMqUser     = "$biz"
	serverMqPassword = "public"
	mqttClient       *mqtttest.DeviceClient
)

func main() {
	connectTioByMqtt()

	// receive thing's properties report
	receiveThingsProperties()
	// receive thing's connect and disconnect message
	receiveThingsPresence()

	prompt := promptui.Select{
		Label: "Select",
		Items: []string{
			"Create Example Thing",
			"Delete Example Thing",
			"Modify Brightness by set `Shadow` desired field",
			"Flash light by invoke thing `Direct Method`",
		},
	}

	for {
		index, result, err := prompt.Run()

		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		fmt.Printf("You choose %d %q\n", index, result)
		switch index {
		case 0:
			createExampleThing()
		case 1:
			deleteExampleThing()
		case 2:
			setBrightnessByShadow()
		case 3:
			flashLightByDirectMethod()
		}
	}
}

func connectTioByMqtt() {
	ctx := context.Background()
	cld := fmt.Sprintf("%s-%d", serverMqUser, rand.Intn(100))
	mqttClient = mqtttest.NewDeviceClient("tcp://localhost:1883", cld, serverMqUser, serverMqPassword)
	err := mqttClient.Connect(ctx)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func receiveThingsPresence() {
	topic := "$iothub/things/+/presence"
	err := mqttClient.Subscribe(topic, 0, func(c mqtt.Client, m mqtt.Message) {
		slog.Info("[Receive Things presence]", "payload", m.Payload())
	})
	if err != nil {
		log.Fatalf("subscribe error %v", err)
	}
}

func receiveThingsProperties() {
	topic := "$iothub/things/+/messages/property"
	err := mqttClient.Subscribe(topic, 0, func(c mqtt.Client, m mqtt.Message) {
		slog.Info("[Receive Things Properties]", "payload", m.Payload())
	})
	if err != nil {
		log.Fatalf("subscribe error %v", err)
	}
}

func createExampleThing() {
	createThReq := api.CreateReq{ThingId: "example", Password: "example"}
	b, _ := json.Marshal(createThReq)

	slog.Info("create thing", "method", http.MethodPost, "url", fmt.Sprintf("%s/api/v1/things", httpUrl), "body", string(b))

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/things", httpUrl), bytes.NewBuffer(b))
	req.SetBasicAuth(userName, password)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("create thing error", "error", err)
	}
	body, _ := io.ReadAll(resp.Body)
	slog.Info(string(body))
}

func deleteExampleThing() {
	slog.Info("delete thing", "method", http.MethodDelete, "url", fmt.Sprintf("%s/api/v1/things/%s", httpUrl, thingId))

	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/things/%s", httpUrl, thingId), bytes.NewBuffer(nil))
	req.SetBasicAuth(userName, password)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("create thing error", "error", err)
	}
	body, _ := io.ReadAll(resp.Body)
	slog.Info(string(body))
}

// setBrightnessByShadow Notify light to adjust brightness by set `desired` of `Shadow`
func setBrightnessByShadow() {
	randBrt := rand.Intn(100)
	methodBody := strings.NewReader(
		fmt.Sprintf(`{
		"clientToken": "test-%d",
		"state": {
			"desired": {
				"brightness": %d
			}
		}
	}`, time.Now().UnixMicro(), randBrt))

	slog.Info("set brightness", "method", http.MethodPut, "url", fmt.Sprintf("%s/api/v1/things/%s/shadows/default/state/desired", httpUrl, thingId), "body", methodBody)

	req, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/v1/things/%s/shadows/default/state/desired", httpUrl, thingId), methodBody)
	req.SetBasicAuth(userName, password)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Modify Brightness error", "error", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	slog.Info(string(body))
}

// flashLightByDirectMethod Flash light 2 times by invoke `Direct Method`
func flashLightByDirectMethod() {
	methodBody := strings.NewReader(`{
		"respTimeout": 3,
		"data": {
			"times": 2
		}
	}`)
	slog.Info("flash light", "method", http.MethodPost, "url", fmt.Sprintf("%s/api/v1/things/%s/methods/%s", httpUrl, thingId, "flash"), "body", methodBody)
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/v1/things/%s/methods/%s", httpUrl, thingId, "flash"), methodBody)
	req.SetBasicAuth(userName, password)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Flash Light error", "error", err)
	}
	body, _ := io.ReadAll(resp.Body)
	slog.Info(string(body))
}
