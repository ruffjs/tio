//go:build integration

package cluster_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"

	"ruff.io/tio/config"
	"ruff.io/tio/connector"
	natsConn "ruff.io/tio/connector/nats"
	"ruff.io/tio/pkg/codec"
)

type connectorCluster struct {
	connectors []*natsConn.Connector
	mqttPorts  []int
}

type presenceRecord struct {
	ThingId    string `json:"thingId"`
	Connected  bool   `json:"connected"`
	ServerId   string `json:"serverId"`
	ClientId   string `json:"clientId"`
	RemoteAddr string `json:"remoteAddr"`
	Timestamp  int64  `json:"timestamp"`
}

func startConnectorCluster(t *testing.T, n int) *connectorCluster {
	t.Helper()
	if n < 2 {
		t.Fatal("cluster size must be at least 2")
	}

	clusterPorts := make([]int, n)
	for i := range clusterPorts {
		clusterPorts[i] = freePort(t)
	}

	routeURLs := make([]string, n)
	for i := range routeURLs {
		routeURLs[i] = fmt.Sprintf("nats://127.0.0.1:%d", clusterPorts[i])
	}

	storeDirs := make([]string, n)
	for i := range storeDirs {
		storeDirs[i] = t.TempDir()
	}

	deviceCodec, err := codec.New("json")
	require.NoError(t, err)

	connectors := make([]*natsConn.Connector, n)

	// Phase 1: Create all connectors and start servers only.
	// NATS MQTT gateway fails if cluster peers aren't available,
	// so all servers must be running before MQTT clients connect.
	for i := range n {
		var routes []string
		for j, u := range routeURLs {
			if j != i {
				routes = append(routes, u)
			}
		}

		cfg := config.NatsConfig{
			Server: config.NatsServerConfig{
				ServerName:         fmt.Sprintf("node-%d", i+1),
				ClusterName:        "test-cluster",
				Port:               -1,
				ClusterPort:        clusterPorts[i],
				MqttPort:           -1,
				Routes:             routes,
				StoreDir:           storeDirs[i],
				JetStreamMaxMemory: 64 * 1024 * 1024,
				JetStreamMaxStore:  128 * 1024 * 1024,
				MqttStreamReplicas: n,
				PresenceReplicas:   n,
			},
			AppClient:     config.NatsClientConfig{User: "$tio-app", Password: "public"},
			SystemClient:  config.NatsClientConfig{User: "$tio-sys", Password: "public"},
			MqttPublisher: config.NatsClientConfig{User: "$tio-mqtt-publisher", Password: "public"},
		}

		c, err := natsConn.NewConnector(cfg, deviceCodec, "legacy")
		require.NoError(t, err)

		authzFn := func(authCtx connector.AuthContext) (connector.AuthResult, bool) {
			return connector.AuthResult{Principal: authCtx.Username, AuthMethod: "test"}, true
		}
		err = c.ConfigureAuth(authzFn, nil)
		require.NoError(t, err)

		err = c.StartServerOnly()
		require.NoError(t, err, "connector %d StartServerOnly failed", i)

		connectors[i] = c
		time.Sleep(500 * time.Millisecond)
	}

	// Wait for cluster routes to form
	for i, c := range connectors {
		require.Eventually(t, func() bool {
			return c.Server().Server().NumRoutes() >= n-1
		}, 15*time.Second, 200*time.Millisecond, "node %d should form cluster routes", i+1)
	}

	// Give JetStream consensus time to stabilize before MQTT clients connect.
	// Without this delay, the MQTT stream creation fails on the first node
	// because JetStream hasn't finished leader election across the cluster.
	time.Sleep(5 * time.Second)

	// Phase 2: Start clients (NATS, MQTT publisher, presence, control)
	mqttPorts := make([]int, n)
	for i, c := range connectors {
		err := c.StartClients(context.Background())
		require.NoError(t, err, "connector %d StartClients failed", i)
		mqttPorts[i] = c.Server().MqttPort()
	}

	cc := &connectorCluster{connectors: connectors, mqttPorts: mqttPorts}
	t.Cleanup(func() {
		for _, c := range cc.connectors {
			if c != nil {
				_ = c.Shutdown()
			}
		}
	})
	return cc
}

func connectMqttDevice(t *testing.T, mqttPort int, thingId string, autoReconnect bool) mqtt.Client {
	t.Helper()
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", mqttPort))
	opts.SetClientID(thingId)
	opts.SetUsername(thingId)
	opts.SetPassword("test")
	opts.SetAutoReconnect(autoReconnect)
	opts.SetCleanSession(true)
	opts.SetConnectTimeout(5 * time.Second)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	require.True(t, token.Wait(), "MQTT connect timeout")
	require.NoError(t, token.Error(), "MQTT connect failed")
	return client
}

func waitForPresence(t *testing.T, conn *natsConn.Connector, thingId string, connected bool, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		c, err := conn.IsConnected(thingId)
		if err != nil {
			return false
		}
		return c == connected
	}, timeout, 200*time.Millisecond, "presence for %q should become connected=%v", thingId, connected)
}

func getPresenceRecord(t *testing.T, conn *natsConn.Connector, thingId string) presenceRecord {
	t.Helper()
	kv, err := conn.JetStream().KeyValue("TIO_PRESENCE")
	require.NoError(t, err)
	entry, err := kv.Get("presence." + thingId)
	require.NoError(t, err)
	var rec presenceRecord
	err = json.Unmarshal(entry.Value(), &rec)
	require.NoError(t, err)
	return rec
}

func TestClusterPresenceCrossNode(t *testing.T) {
	cc := startConnectorCluster(t, 3)
	thingId := "dev-presence-001"

	device := connectMqttDevice(t, cc.mqttPorts[0], thingId, true)

	for _, c := range cc.connectors {
		waitForPresence(t, c, thingId, true, 10*time.Second)
	}
	for i, c := range cc.connectors {
		rec := getPresenceRecord(t, c, thingId)
		require.Equal(t, "node-1", rec.ServerId, "node %d should see ServerId=node-1", i+1)
	}

	device.Disconnect(100)

	for _, c := range cc.connectors {
		waitForPresence(t, c, thingId, false, 10*time.Second)
	}
}

func TestClusterControlDisconnect(t *testing.T) {
	cc := startConnectorCluster(t, 3)
	thingId := "dev-close-001"

	device := connectMqttDevice(t, cc.mqttPorts[0], thingId, false)
	waitForPresence(t, cc.connectors[0], thingId, true, 10*time.Second)

	err := cc.connectors[1].Close(thingId)
	require.NoError(t, err)

	for _, c := range cc.connectors {
		waitForPresence(t, c, thingId, false, 5*time.Second)
	}

	device.Disconnect(100)
}

func TestClusterRemove(t *testing.T) {
	cc := startConnectorCluster(t, 3)
	thingId := "dev-remove-001"

	device := connectMqttDevice(t, cc.mqttPorts[0], thingId, false)
	waitForPresence(t, cc.connectors[0], thingId, true, 10*time.Second)

	err := cc.connectors[1].Remove(thingId)
	require.NoError(t, err)

	// Remove deletes the KV key, but handleDisconnectEvent on the owning
	// node may race to write Connected=false before the deletion replicates.
	// Accept both outcomes: key deleted OR connected=false.
	require.Eventually(t, func() bool {
		kv, err := cc.connectors[1].JetStream().KeyValue("TIO_PRESENCE")
		if err != nil {
			return false
		}
		entry, err := kv.Get("presence." + thingId)
		if errors.Is(err, nats.ErrKeyNotFound) {
			return true
		}
		if err != nil {
			return false
		}
		var rec presenceRecord
		if json.Unmarshal(entry.Value(), &rec) == nil {
			return !rec.Connected
		}
		return false
	}, 10*time.Second, 200*time.Millisecond, "KV record should be deleted or connected=false")

	for _, c := range cc.connectors {
		connected, err := c.IsConnected(thingId)
		require.NoError(t, err)
		require.False(t, connected)
	}

	device.Disconnect(100)
}

func TestClusterReconciliation_FakeRecord(t *testing.T) {
	cc := startConnectorCluster(t, 3)

	fakeThingId := "dev-fake-001"
	kv, err := cc.connectors[1].JetStream().KeyValue("TIO_PRESENCE")
	require.NoError(t, err)

	fakeRec := presenceRecord{
		ThingId:   fakeThingId,
		Connected: true,
		ServerId:  "non-existent-node",
		Timestamp: time.Now().UnixMilli(),
	}
	data, err := json.Marshal(fakeRec)
	require.NoError(t, err)
	_, err = kv.Put("presence."+fakeThingId, data)
	require.NoError(t, err)

	waitForPresence(t, cc.connectors[1], fakeThingId, false, 20*time.Second)
	waitForPresence(t, cc.connectors[2], fakeThingId, false, 20*time.Second)
}

func TestClusterReconciliation_NodeShutdown(t *testing.T) {
	cc := startConnectorCluster(t, 3)
	thingId := "dev-reconcile-001"

	device := connectMqttDevice(t, cc.mqttPorts[0], thingId, false)

	for _, c := range cc.connectors {
		waitForPresence(t, c, thingId, true, 10*time.Second)
	}

	cc.connectors[0].Shutdown()
	cc.connectors[0] = nil

	waitForPresence(t, cc.connectors[1], thingId, false, 20*time.Second)
	waitForPresence(t, cc.connectors[2], thingId, false, 20*time.Second)

	device.Disconnect(100)
}
