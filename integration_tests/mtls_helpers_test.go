//go:build integration

package integration_tests

import (
	"context"
	"fmt"

	mq "ruff.io/tio/internal/mqtttest"
	tioconnector "ruff.io/tio/connector"
)

var connector = &testConnectorWrapper{}

type testConnectorWrapper struct{}

func (w *testConnectorWrapper) SubscribePresence(ctx context.Context) <-chan tioconnector.PresenceEvent {
	ch := natsConnector.SubscribePresence(ctx)
	out := make(chan tioconnector.PresenceEvent, 100)
	go func() {
		for e := range ch {
			out <- e
		}
	}()
	return out
}

func (w *testConnectorWrapper) ClientInfo(thingId string) (tioconnector.ClientInfo, error) {
	return natsConnector.ClientInfo(thingId)
}

func (w *testConnectorWrapper) Publish(topic string, qos byte, retained bool, payload []byte) error {
	if retained {
		return natsConnector.PublishRetained(topic, payload)
	}
	return natsConnector.PublishReliable(topic, payload)
}

func newThingMTLSClient(thingID string) *mq.DeviceClient {
	port := natsConnector.Server().MqttPort()
	certDir := "../demos/mtls/certs"
	c, err := mq.NewDeviceClientWithTLS(
		fmt.Sprintf("tls://127.0.0.1:%d", port),
		thingID, thingID, "",
		certDir+"/ca.pem",
		certDir+"/client-cert.pem",
		certDir+"/client-key.pem",
	)
	if err != nil {
		panic(err)
	}
	return c
}

func newThingTLSClientWithoutCertificate(thingID string) *mq.DeviceClient {
	port := natsConnector.Server().MqttPort()
	c, err := mq.NewDeviceClientWithTLS(
		fmt.Sprintf("tls://127.0.0.1:%d", port),
		thingID, thingID, "test-password",
		"", "", "",
	)
	if err != nil {
		panic(err)
	}
	return c
}

func newThingMTLSClientWithUsername(thingID, username, certFile, keyFile, caFile, serverName string) *mq.DeviceClient {
	port := natsConnector.Server().MqttPort()
	c, err := mq.NewDeviceClientWithTLS(
		fmt.Sprintf("tls://127.0.0.1:%d", port),
		thingID, username, "",
		caFile, certFile, keyFile,
	)
	if err != nil {
		panic(err)
	}
	return c
}

func newThingMTLSClientWithCertFiles(thingID, certFile, keyFile, caFile, serverName string) *mq.DeviceClient {
	port := natsConnector.Server().MqttPort()
	c, err := mq.NewDeviceClientWithTLS(
		fmt.Sprintf("tls://127.0.0.1:%d", port),
		thingID, thingID, "",
		caFile, certFile, keyFile,
	)
	if err != nil {
		panic(err)
	}
	return c
}

func newThingMTLSClientWithServerName(thingID, serverName string) *mq.DeviceClient {
	port := natsConnector.Server().MqttPort()
	c, err := mq.NewDeviceClientWithTLS(
		fmt.Sprintf("tls://127.0.0.1:%d", port),
		thingID, thingID, "",
		"../demos/mtls/certs/ca.pem",
		"../demos/mtls/certs/client-cert.pem",
		"../demos/mtls/certs/client-key.pem",
	)
	if err != nil {
		panic(err)
	}
	return c
}
