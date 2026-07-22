package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"ruff.io/tio/config"
	"ruff.io/tio/connector/mqtt/client"
	"ruff.io/tio/shadow"
)

var (
	thingId    string // will be extracted from client certificate CN
	lightState = map[string]any{
		"brightness": 0,
		"power":      "off",
		"voltage":    0,
	}

	mqttClient client.Client
	ctx        context.Context
)

func main() {
	ctx = context.Background()

	// Connect with mTLS
	connectTioByMtls()

	// Subscribe shadow topics and direct method invoke topic
	receiveShadowGetResp()
	receiveShadowUpdateResp()
	receiveShadowDeltaNotice()
	receiveDirectMethodInvoke()

	// Report the current state of light when it boot
	updateShadowReported(lightState)

	select {}
}

// connectTioByMtls connects to MQTT broker using mutual TLS authentication
func connectTioByMtls() {
	_, currentFile, _, _ := runtime.Caller(0)
	baseDir := filepath.Clean(filepath.Join(filepath.Dir(currentFile), ".."))

	certDir := filepath.Join(baseDir, "certs")
	caCertPath := envOrDefault("TIO_MTLS_CA_FILE", filepath.Join(certDir, "ca.pem"))
	clientCertPath := envOrDefault("TIO_MTLS_CLIENT_CERT_FILE", filepath.Join(certDir, "client-cert.pem"))
	clientKeyPath := envOrDefault("TIO_MTLS_CLIENT_KEY_FILE", filepath.Join(certDir, "client-key.pem"))
	host := envOrDefault("TIO_MTLS_HOST", "localhost")
	serverName := envOrDefault("TIO_MTLS_SERVER_NAME", "localhost")
	port := envIntOrDefault("TIO_MTLS_PORT", 8883)

	slog.Info("mTLS certificate paths",
		"certDir", certDir,
		"caFile", caCertPath,
		"clientCertFile", clientCertPath,
		"clientKeyFile", clientKeyPath,
		"host", host,
		"port", port,
		"serverName", serverName)

	// Verify certificate files exist
	if _, err := os.Stat(caCertPath); err != nil {
		log.Fatalf("CA certificate file not found: %s", caCertPath)
	}
	if _, err := os.Stat(clientCertPath); err != nil {
		log.Fatalf("Client certificate file not found: %s", clientCertPath)
	}
	if _, err := os.Stat(clientKeyPath); err != nil {
		log.Fatalf("Client key file not found: %s", clientKeyPath)
	}

	cert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		log.Fatalf("Failed to load client certificate: %v", err)
	}

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		log.Fatalf("Failed to parse client certificate: %v", err)
	}

	thingId = x509Cert.Subject.CommonName
	slog.Info("Extracted thingId from client certificate CN", "thingId", thingId)

	cfg := config.MqttClientConfig{
		ClientId: thingId,
		Host:     host,
		Port:     port,
		TLS: &config.TLSConfig{
			CAFile:     caCertPath,
			CertFile:   clientCertPath,
			KeyFile:    clientKeyPath,
			ServerName: serverName,
		},
	}

	mqttClient = client.NewClient(cfg)

	mqttClient.OnConnect(func() {
		slog.Info("Successfully connected to MQTT broker using mutual TLS", "clientId", thingId)
	})

	err = mqttClient.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to MQTT broker with mTLS: %v", err)
	}

	slog.Info("mTLS connection established successfully")
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	port, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("invalid integer environment variable %s=%q: %v", key, value, err)
	}
	return port
}

func receiveShadowGetResp() {
	slog.Info("[Receive Shadow Get Response] subscribe")

	topicReq := fmt.Sprintf("$iothub/things/%s/shadows/name/default/get/+", thingId)
	accepted := "accepted"
	rejected := "rejected"

	err := mqttClient.Subscribe(ctx, topicReq, 0, func(c mqtt.Client, m mqtt.Message) {
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

	err := mqttClient.Subscribe(ctx, topicReq, 0, func(c mqtt.Client, m mqtt.Message) {
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

func receiveShadowDeltaNotice() {
	topic := fmt.Sprintf("$iothub/things/%s/shadows/name/default/update/delta", thingId)

	err := mqttClient.Subscribe(ctx, topic, 0, func(c mqtt.Client, m mqtt.Message) {
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

func receiveDirectMethodInvoke() {
	slog.Info("[Receive Method Request] subscribe method request: make the light flash once")

	topicReq := fmt.Sprintf("$iothub/things/%s/methods/%s/req", thingId, "flash")
	topicResp := fmt.Sprintf("$iothub/things/%s/methods/%s/resp", thingId, "flash")

	slog.Info("=== subscribe", "topicReq", topicReq, "topicResp", topicResp)

	err := mqttClient.Subscribe(ctx, topicReq, 0, func(c mqtt.Client, m mqtt.Message) {
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

						flashLight(c)
						resp = shadow.MethodResp{
							ClientToken: req.ClientToken,
							Data:        fmt.Sprintf("light flash %d times", c),
							Message:     "OK from device (mTLS)",
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

func doControlOrConfigByDelta(shadowDelta map[string]any) {
	for k, v := range shadowDelta {
		if lightState[k] != nil {
			switch k {
			case "brightness":
				slog.Info("[Receive Shadow Delta] adjust brightness to", "value", v)
				lightState[k] = v
			case "power":
				if v == "on" {
					slog.Info("[Receive Shadow Delta] turn on light")
				} else {
					slog.Info("[Receive Shadow Delta] turn off light")
				}
				lightState[k] = v
			default:
				slog.Info("[Receive Shadow Delta] shadow delta field", "field", k)
			}
		} else {
			slog.Error("[Receive Shadow Delta] unknown shadow delta field", "field", k)
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
