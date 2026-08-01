//go:build integration

package integration_tests

import (
	"fmt"
	"testing"

	tioconnector "ruff.io/tio/connector"
	mq "ruff.io/tio/internal/mqtttest"
)

var mtlsConnector = &testConnectorWrapper{}

func requireMTLSFixture(t *testing.T) {
	t.Helper()
	if cfg.Connector.Nats.Server.MqttTLS.CertFile == "" {
		t.Skip("set TIO_TEST_MQTT_TLS=1 to run mTLS integration tests")
	}
}

type testConnectorWrapper struct{}

func (w *testConnectorWrapper) IsConnected(thingId string) (bool, error) {
	return natsConnector.IsConnected(thingId)
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
		"localhost",
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
		"", "", "", "localhost",
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
		caFile, certFile, keyFile, serverName,
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
		caFile, certFile, keyFile, serverName,
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
		serverName,
	)
	if err != nil {
		panic(err)
	}
	return c
}
