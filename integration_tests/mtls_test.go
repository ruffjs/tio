//go:build integration

package integration_tests

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/shadow"
	"ruff.io/tio/thing"
)

func TestThingConnectWithCertificateAndPasswordCoexist(t *testing.T) {
	certThingID := "mtls-client"
	passwordThingID := ID()
	password := "public-password"

	_, err := thingSvc.Create(context.Background(), thing.Thing{
		Id:       certThingID,
		Enabled:  true,
		AuthType: thing.AuthTypeCertificate,
	}, nil, true)
	require.NoError(t, err)

	_, err = thingSvc.Create(context.Background(), thing.Thing{
		Id:        passwordThingID,
		Enabled:   true,
		AuthType:  thing.AuthTypePassword,
		AuthValue: password,
	}, nil, true)
	require.NoError(t, err)

	certClient := newThingMTLSClient(certThingID)
	require.NoError(t, certClient.Connect(context.Background()))
	t.Cleanup(certClient.Disconnect)

	passwordClient := newThingMqttClient(context.Background(), passwordThingID, password)
	require.NoError(t, passwordClient.Connect(context.Background()))
	t.Cleanup(passwordClient.Disconnect)

	waitConnected(t, certThingID)
	waitConnected(t, passwordThingID)
}

func TestThingRejectsInvalidAuthenticationPaths(t *testing.T) {
	tests := []struct {
		name       string
		setupThing func(t *testing.T, thingID string)
		newClient  func(t *testing.T, thingID string) clientWithDisconnect
	}{
		{
			name: "password thing rejects certificate connection",
			setupThing: func(t *testing.T, thingID string) {
				_, err := thingSvc.Create(context.Background(), thing.Thing{
					Id:        thingID,
					Enabled:   true,
					AuthType:  thing.AuthTypePassword,
					AuthValue: "public-password",
				}, nil, true)
				require.NoError(t, err)
			},
			newClient: func(t *testing.T, thingID string) clientWithDisconnect {
				return clientWithDisconnect{Client: newThingMTLSClient(thingID)}
			},
		},
		{
			name: "certificate thing rejects password connection",
			setupThing: func(t *testing.T, thingID string) {
				_, err := thingSvc.Create(context.Background(), thing.Thing{
					Id:       thingID,
					Enabled:  true,
					AuthType: thing.AuthTypeCertificate,
				}, nil, true)
				require.NoError(t, err)
			},
			newClient: func(t *testing.T, thingID string) clientWithDisconnect {
				return clientWithDisconnect{Client: newThingMqttClient(context.Background(), thingID, "public-password")}
			},
		},
		{
			name: "mtls port rejects client without certificate",
			setupThing: func(t *testing.T, thingID string) {
				_, err := thingSvc.Create(context.Background(), thing.Thing{
					Id:       thingID,
					Enabled:  true,
					AuthType: thing.AuthTypeCertificate,
				}, nil, true)
				require.NoError(t, err)
			},
			newClient: func(t *testing.T, thingID string) clientWithDisconnect {
				return clientWithDisconnect{Client: newThingTLSClientWithoutCertificate(thingID)}
			},
		},
		{
			name: "certificate thing rejects mismatched username",
			setupThing: func(t *testing.T, thingID string) {
				_, err := thingSvc.Create(context.Background(), thing.Thing{
					Id:       thingID,
					Enabled:  true,
					AuthType: thing.AuthTypeCertificate,
				}, nil, true)
				require.NoError(t, err)
			},
			newClient: func(t *testing.T, thingID string) clientWithDisconnect {
				certDir := filepath.Join("..", "demos", "mtls", "certs")
				return clientWithDisconnect{Client: newThingMTLSClientWithUsername(
					thingID,
					"wrong-username",
					filepath.Join(certDir, "client-cert.pem"),
					filepath.Join(certDir, "client-key.pem"),
					filepath.Join(certDir, "ca.pem"),
					"localhost",
				)}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thingID := "mtls-client"
			tt.setupThing(t, thingID)
			client := tt.newClient(t, thingID)

			err := client.Connect(context.Background())
			require.Error(t, err)
			waitDisconnected(t, thingID)
		})
	}
}

func TestThingRejectsInvalidClientCertificates(t *testing.T) {
	tests := []struct {
		name      string
		certFiles func(t *testing.T, thingID string) (string, string, string)
	}{
		{
			name: "client certificate signed by wrong ca",
			certFiles: func(t *testing.T, thingID string) (string, string, string) {
				return mustCreateTempClientCertFiles(
					t,
					thingID,
					time.Now().Add(-time.Hour),
					time.Now().Add(time.Hour),
					false,
				)
			},
		},
		{
			name: "expired client certificate",
			certFiles: func(t *testing.T, thingID string) (string, string, string) {
				return mustCreateTempClientCertFiles(
					t,
					thingID,
					time.Now().Add(-2*time.Hour),
					time.Now().Add(-time.Hour),
					true,
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thingID := "mtls-client"
			_, err := thingSvc.Create(context.Background(), thing.Thing{
				Id:       thingID,
				Enabled:  true,
				AuthType: thing.AuthTypeCertificate,
			}, nil, true)
			require.NoError(t, err)

			certFile, keyFile, caFile := tt.certFiles(t, thingID)
			client := newThingMTLSClientWithCertFiles(thingID, certFile, keyFile, caFile, "localhost")

			err = client.Connect(context.Background())
			require.Error(t, err)
			waitDisconnected(t, thingID)
		})
	}
}

func TestDeviceRejectsInvalidServerCertificate(t *testing.T) {
	tests := []struct {
		name       string
		serverName string
	}{
		{
			name:       "server certificate name mismatch",
			serverName: "not-tio.example",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thingID := "mtls-client"
			_, err := thingSvc.Create(context.Background(), thing.Thing{
				Id:       thingID,
				Enabled:  true,
				AuthType: thing.AuthTypeCertificate,
			}, nil, true)
			require.NoError(t, err)

			client := newThingMTLSClientWithServerName(thingID, tt.serverName)
			err = client.Connect(context.Background())
			require.Error(t, err)
			waitDisconnected(t, thingID)
		})
	}
}

func TestCertificateConnectionUsesAuthenticatedThingIDForPresenceAndACL(t *testing.T) {
	certThingID := "mtls-client"
	otherThingID := ID()

	_, err := thingSvc.Create(context.Background(), thing.Thing{
		Id:       certThingID,
		Enabled:  true,
		AuthType: thing.AuthTypeCertificate,
	}, nil, true)
	require.NoError(t, err)

	_, err = thingSvc.Create(context.Background(), thing.Thing{
		Id:       otherThingID,
		Enabled:  true,
		AuthType: thing.AuthTypeCertificate,
	}, nil, true)
	require.NoError(t, err)

	certClient := newThingMTLSClient(certThingID)
	require.NoError(t, certClient.Connect(context.Background()))
	t.Cleanup(certClient.Disconnect)

	waitConnected(t, certThingID)

	info, err := mtlsConnector.ClientInfo(certThingID)
	require.NoError(t, err)
	require.Equal(t, certThingID, info.Username)

	var allowedTopicPayload []byte
	var deniedTopicPayload []byte
	err = certClient.Subscribe(shadow.TopicDeltaStateOf(certThingID), 0, func(_ mqtt.Client, m mqtt.Message) {
		allowedTopicPayload = append([]byte(nil), m.Payload()...)
	})
	require.NoError(t, err)

	err = certClient.Subscribe(shadow.TopicDeltaStateOf(otherThingID), 0, func(_ mqtt.Client, m mqtt.Message) {
		deniedTopicPayload = append([]byte(nil), m.Payload()...)
	})
	require.NoError(t, err)

	payload, err := json.Marshal(shadow.DeltaStateNotice{
		Version: 1,
		State:   shadow.StateValue{"on": true},
	})
	require.NoError(t, err)

	require.NoError(t, mtlsConnector.Publish(shadow.TopicDeltaStateOf(certThingID), 0, false, payload))
	require.NoError(t, mtlsConnector.Publish(shadow.TopicDeltaStateOf(otherThingID), 0, false, payload))
	require.Eventually(t, func() bool {
		return len(allowedTopicPayload) > 0
	}, 3*time.Second, 50*time.Millisecond)
	require.Never(t, func() bool {
		return len(deniedTopicPayload) > 0
	}, 500*time.Millisecond, 50*time.Millisecond)
}

type clientWithDisconnect struct {
	Client interface {
		Connect(context.Context) error
		Disconnect()
	}
}

func (c clientWithDisconnect) Connect(ctx context.Context) error {
	return c.Client.Connect(ctx)
}
