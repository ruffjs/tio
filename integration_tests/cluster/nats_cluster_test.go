//go:build integration

package cluster_test

import (
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	natsConn "ruff.io/tio/connector/nats"
	"ruff.io/tio/config"

	server "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

type clusterSetup struct {
	servers   []*natsConn.NatsServer
	clients   []*nats.Conn
	storeDirs []string
	clusterPorts []int
}

type allowAllAuth struct {
	appAcc *server.Account
}

func (a *allowAllAuth) Check(c server.ClientAuthentication) bool {
	if a.appAcc != nil {
		opts := c.GetOpts()
		c.RegisterUser(&server.User{
			Username: opts.Username,
			Account:  a.appAcc,
		})
	}
	return true
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func startCluster(t *testing.T) *clusterSetup {
	t.Helper()
	const n = 3
	storeDirs := make([]string, n)
	for i := range storeDirs {
		storeDirs[i] = t.TempDir()
	}

	clusterPorts := make([]int, n)
	for i := range clusterPorts {
		clusterPorts[i] = freePort(t)
	}

	routeURLs := make([]string, n)
	for i := range routeURLs {
		routeURLs[i] = fmt.Sprintf("nats://127.0.0.1:%d", clusterPorts[i])
	}

	servers := make([]*natsConn.NatsServer, n)
	for i := range n {
		var routes []string
		for j, u := range routeURLs {
			if j != i {
				routes = append(routes, u)
			}
		}
		cfg := config.NatsServerConfig{
			ServerName:         fmt.Sprintf("node-%d", i+1),
			ClusterName:        "test-cluster",
			Port:               -1,
			ClusterPort:        clusterPorts[i],
			Routes:             routes,
			StoreDir:           storeDirs[i],
			JetStreamMaxMemory: 64 * 1024 * 1024,
			JetStreamMaxStore:  128 * 1024 * 1024,
		}
		authn := &allowAllAuth{}
		s, err := natsConn.StartNatsServer(cfg, authn)
		require.NoError(t, err)
		appAcc, _ := s.Server().LookupAccount(natsConn.AppAccountName)
		authn.appAcc = appAcc
		servers[i] = s
	}

	clients := make([]*nats.Conn, n)
	for i := range servers {
		nc, err := nats.Connect(servers[i].ClientURL(), nats.Name(fmt.Sprintf("test-client-%d", i+1)))
		require.NoError(t, err)
		clients[i] = nc
	}

	for i := range servers {
		require.Eventually(t, func() bool {
			return servers[i].Server().NumRoutes() >= n-1
		}, 15*time.Second, 100*time.Millisecond, "node %d should form cluster routes", i+1)
	}

	time.Sleep(time.Second)

	cs := &clusterSetup{servers: servers, clients: clients, storeDirs: storeDirs, clusterPorts: clusterPorts}
	t.Cleanup(func() {
		for _, nc := range clients {
			nc.Close()
		}
		for _, s := range servers {
			s.Shutdown()
		}
	})
	return cs
}

func jsConn(t *testing.T, nc *nats.Conn) nats.JetStreamContext {
	t.Helper()
	js, err := nc.JetStream()
	require.NoError(t, err)
	return js
}

func TestNATSClusterFormation(t *testing.T) {
	cs := startCluster(t)

	for i, s := range cs.servers {
		require.GreaterOrEqual(t, s.Server().NumRoutes(), 2, "node %d should have routes to other nodes", i+1)
		v, err := s.Server().Varz(nil)
		require.NoError(t, err)
		require.Equal(t, "test-cluster", v.Cluster.Name)
	}
}

func TestClusterQueueSubscribe(t *testing.T) {
	cs := startCluster(t)

	var count1, count2 int32
	_, err := cs.clients[1].QueueSubscribe("test.queue", "workers", func(m *nats.Msg) {
		atomic.AddInt32(&count1, 1)
	})
	require.NoError(t, err)

	_, err = cs.clients[2].QueueSubscribe("test.queue", "workers", func(m *nats.Msg) {
		atomic.AddInt32(&count2, 1)
	})
	require.NoError(t, err)
	cs.clients[1].Flush()
	cs.clients[2].Flush()
	time.Sleep(200 * time.Millisecond)

	const total = 10
	for i := 0; i < total; i++ {
		err = cs.clients[0].Publish("test.queue", []byte(fmt.Sprintf("msg-%d", i)))
		require.NoError(t, err)
	}
	cs.clients[0].Flush()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&count1)+atomic.LoadInt32(&count2) >= total
	}, 5*time.Second, 50*time.Millisecond, "all messages should be received by exactly one subscriber")

	require.Equal(t, int32(total), atomic.LoadInt32(&count1)+atomic.LoadInt32(&count2))
}

func TestClusterBroadcastSubscribe(t *testing.T) {
	cs := startCluster(t)

	var count1, count2 int32
	_, err := cs.clients[1].Subscribe("test.broadcast", func(m *nats.Msg) {
		atomic.AddInt32(&count1, 1)
	})
	require.NoError(t, err)

	_, err = cs.clients[2].Subscribe("test.broadcast", func(m *nats.Msg) {
		atomic.AddInt32(&count2, 1)
	})
	require.NoError(t, err)
	cs.clients[1].Flush()
	cs.clients[2].Flush()
	time.Sleep(200 * time.Millisecond)

	const total = 10
	for i := 0; i < total; i++ {
		err = cs.clients[0].Publish("test.broadcast", []byte(fmt.Sprintf("msg-%d", i)))
		require.NoError(t, err)
	}
	cs.clients[0].Flush()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&count1) >= total && atomic.LoadInt32(&count2) >= total
	}, 5*time.Second, 50*time.Millisecond, "both subscribers should receive all messages")
}

func TestClusterPresenceKV(t *testing.T) {
	cs := startCluster(t)

	js0 := jsConn(t, cs.clients[0])
	kv, err := js0.CreateKeyValue(&nats.KeyValueConfig{
		Bucket:   "presence",
		Replicas: 3,
	})
	require.NoError(t, err)
	require.NotNil(t, kv)

	time.Sleep(2 * time.Second)

	_, err = kv.Put("device-1", []byte("online"))
	require.NoError(t, err)

	js1 := jsConn(t, cs.clients[1])
	require.Eventually(t, func() bool {
		e, err := js1.KeyValue("presence")
		if err != nil {
			return false
		}
		entry, err := e.Get("device-1")
		if err != nil {
			return false
		}
		return string(entry.Value()) == "online"
	}, 10*time.Second, 200*time.Millisecond, "node 2 should read value written by node 1")

	js2 := jsConn(t, cs.clients[2])
	require.Eventually(t, func() bool {
		e, err := js2.KeyValue("presence")
		if err != nil {
			return false
		}
		entry, err := e.Get("device-1")
		if err != nil {
			return false
		}
		return string(entry.Value()) == "online"
	}, 5*time.Second, 200*time.Millisecond, "node 3 should read value written by node 1")

	kv2, err := js1.KeyValue("presence")
	require.NoError(t, err)
	_, err = kv2.Put("device-1", []byte("offline"))
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		entry, err := kv.Get("device-1")
		if err != nil {
			return false
		}
		return string(entry.Value()) == "offline"
	}, 5*time.Second, 200*time.Millisecond, "node 1 should see update from node 2")

	require.Eventually(t, func() bool {
		e, err := js2.KeyValue("presence")
		if err != nil {
			return false
		}
		entry, err := e.Get("device-1")
		if err != nil {
			return false
		}
		return string(entry.Value()) == "offline"
	}, 5*time.Second, 200*time.Millisecond, "node 3 should see update from node 2")
}

func TestClusterNodeLoss(t *testing.T) {
	cs := startCluster(t)

	var count2, count3 int32
	_, err := cs.clients[1].QueueSubscribe("test.nodeloss", "workers", func(m *nats.Msg) {
		atomic.AddInt32(&count2, 1)
	})
	require.NoError(t, err)
	_, err = cs.clients[2].QueueSubscribe("test.nodeloss", "workers", func(m *nats.Msg) {
		atomic.AddInt32(&count3, 1)
	})
	require.NoError(t, err)
	cs.clients[1].Flush()
	cs.clients[2].Flush()
	time.Sleep(200 * time.Millisecond)

	for i := 0; i < 5; i++ {
		err = cs.clients[0].Publish("test.nodeloss", []byte(fmt.Sprintf("before-%d", i)))
		require.NoError(t, err)
	}
	cs.clients[0].Flush()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&count2)+atomic.LoadInt32(&count3) >= 5
	}, 5*time.Second, 50*time.Millisecond, "all pre-loss messages should be received")

	cs.clients[1].Close()
	cs.servers[1].Shutdown()
	time.Sleep(2 * time.Second)

	for i := 0; i < 5; i++ {
		err = cs.clients[0].Publish("test.nodeloss", []byte(fmt.Sprintf("after-%d", i)))
		require.NoError(t, err)
	}
	cs.clients[0].Flush()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&count3) >= 5
	}, 10*time.Second, 100*time.Millisecond, "node 3 should receive all post-loss messages")

	total := atomic.LoadInt32(&count2) + atomic.LoadInt32(&count3)
	require.Equal(t, int32(10), total, "total received messages should equal total sent")
}

func TestClusterJetStreamKVSurvivesRestart(t *testing.T) {
	cs := startCluster(t)

	js0 := jsConn(t, cs.clients[0])
	kv, err := js0.CreateKeyValue(&nats.KeyValueConfig{
		Bucket:   "persist",
		Replicas: 3,
	})
	require.NoError(t, err)

	_, err = kv.Put("device-1", []byte("active"))
	require.NoError(t, err)

	cs.clients[0].Close()
	cs.servers[0].Shutdown()
	time.Sleep(2 * time.Second)

	js1 := jsConn(t, cs.clients[1])
	require.Eventually(t, func() bool {
		e, err := js1.KeyValue("persist")
		if err != nil {
			return false
		}
		entry, err := e.Get("device-1")
		if err != nil {
			return false
		}
		return string(entry.Value()) == "active"
	}, 10*time.Second, 200*time.Millisecond, "node 2 should have KV data while node 1 is down")

	storeDir := cs.storeDirs[0]
	clusterPort := freePort(t)

	cfg := config.NatsServerConfig{
		ServerName:  "node-1",
		ClusterName: "test-cluster",
		Port:        -1,
		ClusterPort: clusterPort,
		Routes: []string{
			fmt.Sprintf("nats://127.0.0.1:%d", cs.servers[1].Server().ClusterAddr().Port),
			fmt.Sprintf("nats://127.0.0.1:%d", cs.servers[2].Server().ClusterAddr().Port),
		},
		StoreDir:           storeDir,
		JetStreamMaxMemory: 64 * 1024 * 1024,
		JetStreamMaxStore:  128 * 1024 * 1024,
	}
	restartAuthn := &allowAllAuth{}
	restarted, err := natsConn.StartNatsServer(cfg, restartAuthn)
	require.NoError(t, err)
	restartAppAcc, _ := restarted.Server().LookupAccount(natsConn.AppAccountName)
	restartAuthn.appAcc = restartAppAcc
	cs.servers[0] = restarted

	require.Eventually(t, func() bool {
		return restarted.Server().NumRoutes() >= 2
	}, 15*time.Second, 200*time.Millisecond, "restarted node should rejoin cluster")

	time.Sleep(2 * time.Second)

	nc, err := nats.Connect(restarted.ClientURL(), nats.Name("reconnect-client"))
	require.NoError(t, err)
	defer nc.Close()

	jsR := jsConn(t, nc)
	require.Eventually(t, func() bool {
		e, err := jsR.KeyValue("persist")
		if err != nil {
			return false
		}
		entry, err := e.Get("device-1")
		if err != nil {
			return false
		}
		return string(entry.Value()) == "active"
	}, 10*time.Second, 200*time.Millisecond, "restarted node should recover KV data from store")
}
