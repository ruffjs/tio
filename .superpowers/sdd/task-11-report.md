# Task 11 Report: NATS Cluster Acceptance Tests & Docker Compose Deployment

## Status: DONE

## Commits
- `af5973a` test: add three-node NATS cluster acceptance tests
- `800d294` docs: add docker-compose deployment example for NATS cluster

## Summary

Created three-node NATS cluster acceptance tests and a Docker Compose deployment example for the tio IoT hub.

## Files Created

### 1. `integration_tests/cluster/nats_cluster_test.go`
Six acceptance tests that spin up 3 embedded NATS servers in-process:
- **TestNATSClusterFormation** - Verifies 3 nodes form a cluster with routes to each other
- **TestClusterQueueSubscribe** - Verifies queue groups distribute across nodes (exactly one receives)
- **TestClusterBroadcastSubscribe** - Verifies broadcast delivery to all nodes
- **TestClusterPresenceKV** - Verifies JetStream KV is shared across nodes (read/write from any node)
- **TestClusterNodeLoss** - Verifies service continues when one node dies (queue subscribers redistribute)
- **TestClusterJetStreamKVSurvivesRestart** - Verifies KV data persists after node shutdown and restart

### 2. `deployments/nats-cluster/docker-compose.yml`
Three-node tio cluster with MySQL, proper port mappings, health checks, and persistent volumes.

### 3. `deployments/nats-cluster/configs/node-{1,2,3}.yaml`
Per-node configuration files with unique server names, cluster routes via Docker DNS, R=3 replicas, and shared MySQL.

### 4. `deployments/nats-cluster/README.md`
Architecture diagram, port table, start/stop instructions, TLS guidance, and test commands.

## Test Results

```
=== RUN   TestNATSClusterFormation          --- PASS (1.15s)
=== RUN   TestClusterQueueSubscribe         --- PASS (1.39s)
=== RUN   TestClusterBroadcastSubscribe     --- PASS (1.38s)
=== RUN   TestClusterPresenceKV             --- PASS (3.30s)
=== RUN   TestClusterNodeLoss               --- PASS (3.50s)
=== RUN   TestClusterJetStreamKVSurvivesRestart --- PASS (5.30s)
PASS  ok  ruff.io/tio/integration_tests/cluster  16.343s
```

Full suite (`go test ./...`) remains green - all existing tests still pass.

## Key Design Decisions

1. **Separate sub-package**: Tests live in `integration_tests/cluster/` (package `cluster_test`) with `//go:build integration` tag to avoid conflicts with the existing `TestMain` in the parent package.

2. **Pre-allocated cluster ports**: All 3 servers start simultaneously with pre-allocated cluster ports and full route configurations. This avoids JetStream cluster bootstrap failures that occur when a server starts without routes.

3. **APP account authenticator**: Clients are registered to the `TIO_APP` account (which has JetStream enabled) via a custom `allowAllAuth` that looks up the APP account after server start.

## Concerns

- Route pooling in NATS v2.12.8 means `NumRoutes()` returns ~8 per server (not 2) due to `PoolSize: 3`. Tests use `>= 2` assertions.
- The docker-compose deployment uses `tio:latest` image which must be built separately.
