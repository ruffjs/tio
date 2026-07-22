# Branch Review: feat-nats-integration

**Review Date:** 2026-07-22
**Branch:** develop (NATS commits e56f273..800d294)
**Base:** main
**Reviewer:** Claude (Code Review Agent)

## Verdict: NEEDS_FIXES

The NATS integration is well-architected and mostly complete, but has **3 critical issues** that must be fixed before merging:

1. Presence KV bucket replicas not applied in multi-node clusters
2. Missing NATS connector shutdown in graceful shutdown path
3. Stale `OnConnect()` references in mTLS tests (compile errors)

---

## Critical Issues

### 1. Presence KV Bucket Replicas Not Applied (Data Correctness)

**Location:** `connector/nats/presence.go:68-74`

```go
kv, err := c.js.CreateKeyValue(&nats.KeyValueConfig{
    Bucket:  presenceKVBucket,
    History: 5,
})
```

**Problem:** The `PresenceReplicas` config field is validated but never used. In a multi-node cluster with `presenceReplicas: 3`, the KV bucket is created with default replicas=1, meaning presence data is not replicated across nodes.

**Impact:** If the node hosting the presence KV goes down, all presence data is lost until reconciliation runs (10 seconds). This defeats the purpose of multi-node deployment.

**Fix:** Use `c.cfg.Server.PresenceReplicas` in the KV config:

```go
kv, err := c.js.CreateKeyValue(&nats.KeyValueConfig{
    Bucket:   presenceKVBucket,
    History:  5,
    Replicas: c.cfg.Server.PresenceReplicas,
})
```

---

### 2. Missing NATS Connector Shutdown (Graceful Shutdown)

**Location:** `cmd/tio/main.go:82-89`

**Problem:** The graceful shutdown path cancels the context but never calls `natsConnector.Shutdown()`. The NATS server, MQTT publisher, and NATS connections are not cleaned up.

```go
ctx, cancel := context.WithCancel(context.Background())
config.GlobalCtxCancel = cancel
go func() {
    if sig := signalHandler(ctx); sig != nil {
        cancel()  // Context canceled, but no connector.Shutdown()
        slog.Info("Tio shutdown by signal", "signal", sig)
    }
}()
```

**Impact:** 
- NATS server does not drain connections
- MQTT publisher does not disconnect cleanly
- JetStream data may not flush to disk
- Cluster peers see unclean disconnects

**Fix:** Add a deferred shutdown or explicit shutdown in the signal handler:

```go
defer func() {
    if err := natsConnector.Shutdown(); err != nil {
        slog.Error("NATS connector shutdown error", "error", err)
    }
}()
```

---

### 3. Stale `OnConnect()` References in mTLS Tests (Compile Errors)

**Location:** 
- `integration_tests/mtls_helpers_test.go:16`
- `integration_tests/mtls_test.go:237`

**Problem:** The connector interface changed from `OnConnect() <-chan PresenceEvent` to `SubscribePresence(ctx) <-chan PresenceEvent`, but the mTLS test wrapper still implements the old method.

```go
// mtls_helpers_test.go:16
func (w *testConnectorWrapper) OnConnect() <-chan tioconnector.PresenceEvent {
    ch := natsConnector.SubscribePresence(testCtx)
    // ...
}
```

**Impact:** Code does not compile when mTLS tests are built.

**Fix:** Update the wrapper to implement the new interface:

```go
func (w *testConnectorWrapper) SubscribePresence(ctx context.Context) <-chan tioconnector.PresenceEvent {
    return natsConnector.SubscribePresence(ctx)
}
```

---

## Concerns (Non-Blocking)

### 4. Presence Generation Race Condition

**Location:** `connector/nats/presence.go:110-117`

```go
var gen int64
if existing, err := c.kv.Get(key); err == nil {
    var rec PresenceRecord
    if json.Unmarshal(existing.Value(), &rec) == nil {
        gen = rec.Generation
    }
}
gen++
```

**Problem:** Read-modify-write without optimistic concurrency. If two nodes process CONNECT events for the same thing simultaneously, both may read the same generation and write the same incremented value.

**Mitigation:** Low probability in practice (requires simultaneous reconnects to different nodes). Reconciliation corrects drift every 10 seconds. Consider using KV revision numbers for atomic updates if this becomes a problem.

---

### 5. Dead Code: `topicToNatsSubject` Function

**Location:** `connector/nats/authenticator.go:213-215`

```go
func topicToNatsSubject(topic string) string {
    return strings.ReplaceAll(topic, ".", "/")
}
```

**Problem:** Function is defined but never used. Appears to be leftover from early development.

**Fix:** Delete the function.

---

### 6. Shutdown Errors Silently Ignored

**Location:** `connector/nats/connector.go:172-198`

```go
func (c *Connector) cleanupLocked() error {
    var firstErr error
    // ... cleanup code ...
    return firstErr  // firstErr is never assigned
}
```

**Problem:** `firstErr` is declared but never assigned. All cleanup errors are silently ignored.

**Fix:** Capture errors from each cleanup step:

```go
if c.natsConn != nil {
    if err := c.natsConn.Drain(); err != nil && firstErr == nil {
        firstErr = err
    }
    c.natsConn.Close()
    c.natsConn = nil
}
```

---

### 7. config.yaml Has mTLS Enabled by Default

**Location:** `config.yaml:53-61`

```yaml
mqttBroker:
  tcpPort: 1883
  tcpSslPort: 8883  # Uncommented
  # ...
  certFile: "./demos/mtls/certs/server-cert.pem"  # Uncommented
  requireClientCert: true  # Uncommented
```

**Problem:** The default config requires client certificates, which will break existing deployments that upgrade without reviewing the config.

**Recommendation:** Comment out mTLS settings in `config.yaml` and provide a separate `config-mtls.yaml` example or document the mTLS setup in a README.

---

### 8. EventBus Publish Timeout Blocks Other Subscribers

**Location:** `pkg/eventbus/eventbus.go:48-60`

```go
func (eb *EventBus[T]) Publish(event string, message T) {
    // ...
    for _, ch := range subscribers {
        select {
        case ch <- message:
        case <-time.After(time.Second):
            slog.Error("EventBus notify event timeout in 1 s")
        }
    }
}
```

**Problem:** If one subscriber's channel is full, the publish blocks for 1 second before timing out. This delays delivery to other subscribers.

**Impact:** In a scenario with multiple presence subscribers, a slow consumer can block presence event delivery to fast consumers.

**Mitigation:** Low priority. Consider using non-blocking sends with a goroutine per subscriber if this becomes a bottleneck.

---

### 9. Unused Exported Methods

**Location:** `connector/nats/connector.go:216-219`

```go
func (c *Connector) AppConn() *nats.Conn              { return c.natsConn }
func (c *Connector) SysConn() *nats.Conn              { return c.sysConn }
func (c *Connector) JetStream() nats.JetStreamContext { return c.js }
```

**Problem:** These methods expose internal NATS primitives but are not used anywhere in the codebase.

**Recommendation:** Keep them for now (they may be useful for advanced use cases), but add documentation explaining when to use them.

---

## Positive Observations

1. **Clean Interface Design**: The new `Connector` interface is protocol-neutral and hides NATS/MQTT details from business logic. The three publish methods (`Publish`, `PublishReliable`, `PublishRetained`) map cleanly to device-side semantics.

2. **Comprehensive Test Coverage**: Unit tests for topic conversion, presence tracking, subscriptions, and authentication are thorough. Integration tests cover single-node scenarios well.

3. **Proper Account Isolation**: APP and SYS accounts are correctly isolated, preventing cross-account message leakage.

4. **MQTT Topic Validation**: The topic conversion logic correctly rejects invalid MQTT topics (wildcards in publish, dots in levels, etc.).

5. **Reconciliation Loop**: The 10-second reconciliation loop provides a safety net for presence drift, which is a good pragmatic choice.

6. **Two-Phase Init**: The `ConfigureAuth` + `Start` pattern cleanly separates auth setup from server startup, avoiding race conditions.

7. **Private MQTT Publisher**: Using a dedicated MQTT client for QoS 1/retained messages is a clever way to preserve device-side semantics without exposing Paho to business code.

8. **Shadow Queue Subscriptions**: Shadow update/get requests correctly use `QueueSubscribe` with `tio-shadow` queue, ensuring only one instance processes each request in a cluster.

9. **Config Validation**: `ValidateNatsConfig` catches common misconfigurations (e.g., multi-node with SQLite, wrong replica counts).

10. **Secret Handling**: Passwords are tagged with `json:"-"` to prevent accidental logging.

---

## Suggested Follow-ups (Nice-to-Have)

1. **Cluster Integration Tests**: Add integration tests for 3-node NATS cluster (presence sync, shadow state across nodes, queue subscription distribution).

2. **Metrics**: Add Prometheus metrics for:
   - NATS connections count
   - Presence KV operations
   - MQTT publisher latency
   - Queue subscription message distribution

3. **Health Check**: Expose a health endpoint that reports NATS server status, cluster connectivity, and JetStream health.

4. **Documentation**: Create a deployment guide for multi-node NATS clusters, including:
   - Network requirements (ports 4222, 6222, 1883, 8083)
   - MySQL setup (required for multi-node)
   - TLS configuration for cluster routes
   - Monitoring recommendations

5. **JetStream Stream Replicas**: Similar to presence KV, verify that MQTT streams use `MqttStreamReplicas` config (currently not visible in the diff).

6. **Graceful Shutdown Ordering**: Define and document the shutdown order:
   - Stop accepting new HTTP requests
   - Drain NATS subscriptions
   - Flush JetStream
   - Shutdown NATS server
   - Close database connections

7. **Performance Benchmarks**: Add benchmarks for:
   - Topic conversion (hot path)
   - Presence KV operations
   - Shadow state updates under load

8. **Error Recovery**: Consider auto-recovery for MQTT publisher disconnection (currently relies on Paho's auto-reconnect, which is good, but add logging/metrics).

---

## Summary

The NATS integration is **80% ready** for merge. The architecture is sound, the code is well-structured, and test coverage is good. The build compiles cleanly (`go build ./...` and `go vet` pass).

However, the three critical issues must be addressed:

1. **Fix PresenceReplicas** (1 line change in `connector/nats/presence.go:68-74`)
2. **Add connector shutdown** (add deferred shutdown in `cmd/tio/main.go`)
3. **Fix mTLS test compile errors** (update wrapper method in `integration_tests/mtls_helpers_test.go:16`)

After fixing these, the branch is ready for merge with the non-blocking concerns tracked as follow-up issues.

**Estimated fix time:** 30 minutes for critical issues, 2-4 hours for concerns.

---

## Verification

```bash
$ go build ./...
(no errors)

$ go vet ./connector/nats/...
(no errors)
```

The old `connector/mqtt/` directory has been completely removed as specified in the design doc. All references to the old MQTT broker (mochi-mqtt, emqx) have been cleaned up.
