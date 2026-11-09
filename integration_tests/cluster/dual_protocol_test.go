//go:build integration

package cluster_test

import (
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/connector"
	dbmock "ruff.io/tio/db/mock"
	"ruff.io/tio/pkg/codec"
	"ruff.io/tio/pkg/protocol"
	"ruff.io/tio/pkg/uuid"
	"ruff.io/tio/shadow"
	"ruff.io/tio/thing"
)

func TestDualProfileAndMethodAcrossNodes(t *testing.T) {
	db := dbmock.NewSqliteConnTest()
	require.NoError(t, db.AutoMigrate(&thing.Entity{}, &shadow.Entity{}, &shadow.ConnStatusEntity{}))
	services := []shadow.Service{
		shadow.NewTestSvc(shadow.NewShadowRepo(db), nil, shadow.Config{}),
		shadow.NewTestSvc(shadow.NewShadowRepo(db), nil, shadow.Config{}),
	}
	thingServices := []thing.Service{
		thing.NewSvc(thing.NewThingRepo(db), uuid.New(), services[0], nil),
		thing.NewSvc(thing.NewThingRepo(db), uuid.New(), services[1], nil),
	}
	profiles := []thing.DeviceProfiles{{Service: thingServices[0], DefaultCodec: "json"}, {Service: thingServices[1], DefaultCodec: "json"}}
	_, err := thingServices[0].Create(t.Context(), thing.Thing{
		Id: "dual-cluster-device", Enabled: true, MessageProtocol: "simple", PayloadEncoding: "cbor",
	}, nil, false)
	require.NoError(t, err)
	cluster := startConnectorClusterWithMode(t, 2, "dual")
	c, err := codec.New("json")
	require.NoError(t, err)
	handlers := make([]*protocol.SimpleHandler, 2)
	for i, conn := range cluster.connectors {
		handlers[i], err = protocol.NewSimpleHandler(conn, c, services[i], profiles[i])
		require.NoError(t, err)
		require.NoError(t, handlers[i].Start(t.Context()))
		defer handlers[i].Stop()
	}
	client := connectMqttDevice(t, cluster.mqttPorts[1], "dual-cluster-device", false)
	defer client.Disconnect(250)
	waitForPresence(t, cluster.connectors[0], "dual-cluster-device", true, 5*time.Second)
	for _, p := range profiles {
		got, err := p.GetDeviceProfile(t.Context(), "dual-cluster-device")
		require.NoError(t, err)
		require.Equal(t, connector.DeviceProfile{MessageProtocol: "simple", PayloadEncoding: "cbor"}, got)
	}
	cbor, err := codec.New("cbor")
	require.NoError(t, err)
	token := client.Subscribe("tio/dual-cluster-device/down/method_req", 1, func(_ mqtt.Client, m mqtt.Message) {
		var req protocol.MethodReq
		if cbor.Unmarshal(m.Payload(), &req) != nil {
			return
		}
		raw, err := cbor.Marshal(protocol.MethodResp{ID: req.ID, Code: 200, Data: "cluster-reply"})
		if err == nil {
			client.Publish("tio/dual-cluster-device/up/method_resp", 1, false, raw)
		}
	})
	require.True(t, token.WaitTimeout(5*time.Second))
	require.NoError(t, token.Error())
	// Both nodes can originate calls; responses must reach the node holding pending state.
	for i := range handlers {
		router := &protocol.MethodRouter{Profiles: profiles[i], Simple: handlers[i]}
		for range 3 {
			resp, err := router.Invoke(t.Context(), "dual-cluster-device", "echo", nil, 3*time.Second)
			require.NoError(t, err)
			require.Equal(t, 200, resp.Code)
			require.Equal(t, "cluster-reply", resp.Data)
		}
	}
}
