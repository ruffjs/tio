package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/manifoldco/promptui"
	"ruff.io/tio/config"
	"ruff.io/tio/connector/mqtt/client"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/thing/api"
)

var (
	httpUrl  = "http://127.0.0.1:9000"
	userName = "admin"

	password = "public"
	thingId  = "example"

	serverMqUser     = "$biz"
	serverMqPassword = "public"
	mqttClient       client.Client
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
	cfg := config.MqttClientConfig{ClientId: cld, User: serverMqUser, Password: serverMqPassword, Host: "localhost", Port: 1883}
	mqttClient = client.NewClient(cfg)
	err := mqttClient.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func receiveThingsPresence() {
	topic := "$iothub/things/+/presence"
	err := mqttClient.Subscribe(context.Background(), topic, 0, func(c mqtt.Client, m mqtt.Message) {
		log.Infof("[Receive Things presence] %s", m.Payload())
	})
	if err != nil {
		log.Fatalf("subscribe error %v", err)
	}
}

func receiveThingsProperties() {
	topic := "$iothub/things/+/messages/property"
	err := mqttClient.Subscribe(context.Background(), topic, 0, func(c mqtt.Client, m mqtt.Message) {
		log.Infof("[Receive Things Properties] %s", m.Payload())
		// Do something more, eg: save properties to TSDB; trigger an alert by some rule
		// ...
	})
	if err != nil {
		log.Fatalf("subscribe error %v", err)
	}
}

func createExampleThing() {
	createThReq := api.CreateReq{ThingId: "example", Password: "example"}
	b, _ := json.Marshal(createThReq)

	log.Infof("%s %s Body: %v", http.MethodPost, fmt.Sprintf("%s/api/v1/things", httpUrl), bytes.NewBuffer(b))

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/things", httpUrl), bytes.NewBuffer(b))
	req.SetBasicAuth(userName, password)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("create thing error:", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	log.Info(string(body))
}

func deleteExampleThing() {
	log.Infof("%s %s Body: %v", http.MethodDelete, fmt.Sprintf("%s/api/v1/things/%s", httpUrl, thingId), nil)

	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/things/%s", httpUrl, thingId), bytes.NewBuffer(nil))
	req.SetBasicAuth(userName, password)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("create thing error:", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	log.Info(string(body))
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

	log.Infof("%s %s Body: %v", http.MethodPut,
		fmt.Sprintf("%s/api/v1/things/%s/shadows/default/state/desired", httpUrl, thingId), methodBody)

	req, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/v1/things/%s/shadows/default/state/desired", httpUrl, thingId), methodBody)
	req.SetBasicAuth(userName, password)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("Modify Brightness error:", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	log.Info(string(body))
}

// flashLightByDirectMethod Flash light 2 times by invoke `Direct Method`
func flashLightByDirectMethod() {
	methodBody := strings.NewReader(`{
		"respTimeout": 3,
		"data": {
			"times": 2
		}
	}`)
	log.Infof("%s %s Body: %v", http.MethodPost,
		fmt.Sprintf("%s/api/v1/things/%s/methods/%s", httpUrl, thingId, "flash"), methodBody)
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/v1/things/%s/methods/%s", httpUrl, thingId, "flash"), methodBody)
	req.SetBasicAuth(userName, password)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("Flash Light error:", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	log.Info(string(body))
}
