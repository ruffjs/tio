# Presence Refactor Report (Steps 5-12)

## Status: DONE

## Files Changed

| File | Change |
|------|--------|
| `connector/mock/connector.go` | Removed `presenceBus`/`eventbus`, `SubscribePresence`; added `presenceHandler` field, `OnLocalPresence` method; `SimulatePresence` now takes `ClientInfo` and calls handler |
| `shadow/service.go` | Removed `syncConnStatus`, `toClientInfo`; added `HandleLocalPresence`; `Init` calls `doFirstSyncStatus` directly |
| `shadow/method.go` | Simplified `InvokeMethod` to return `ErrDirectMethodThingOffline` if offline; removed `waiting` map, `waitingResp`, `addWaiting`, `removeWaiting`, `subscribeThingOnline` |
| `job/runner.go` | Removed `SubscribePresence` call and event channel; added 2s retry ticker for offline thing re-enqueue |
| `shadow/mock/mock_connector.go` | Replaced `SubscribePresence` stub with `OnLocalPresence` no-op |
| `cmd/tio/main.go` | Registered `OnLocalPresence` callback between `Start` and `Init` |
| `connector/nats/presence.go` | Restored `Connz`-based reconcile check alongside server-alive check; added `server` import |
| `connector/nats/connectivity.go` | `Remove` now calls `Close` first, then `kv.Delete` (instead of `Purge`) to avoid race with disconnect handler |
| `shadow/method_test.go` | Removed `SimulatePresence`/waiting test cases; offline thing now expects `ErrDirectMethodThingOffline` |
| `connector/mock/connector_test.go` | Replaced `TestPresenceFanOut` with `TestOnLocalPresenceCallback` |
| `connector/nats/presence_test.go` | Replaced `TestPresenceMultipleSubscribersReceiveEvent` with `TestPresenceOnLocalPresenceCallback`; removed `Generation` checks; renamed generation tracking test |
| `integration_tests/mtls_helpers_test.go` | Removed `SubscribePresence` wrapper; added `IsConnected` method |
| `integration_tests/mtls_test.go` | Replaced `SubscribePresence` channel wait with `waitConnected` polling |
| `integration_tests/setup_test.go` | Registered `OnLocalPresence` callback |
| `thing/service_test.go` | Added `HandleLocalPresence` to `failingShadowSvc` mock; added `tioconn` import |

## Test Results

```
ok  	ruff.io/tio/auth
ok  	ruff.io/tio/config
ok  	ruff.io/tio/connector/mock
ok  	ruff.io/tio/connector/nats
ok  	ruff.io/tio/integration_tests
ok  	ruff.io/tio/job
ok  	ruff.io/tio/ntp
ok  	ruff.io/tio/pkg/cache
ok  	ruff.io/tio/pkg/eventbus
ok  	ruff.io/tio/pkg/model
ok  	ruff.io/tio/pkg/redissplit
ok  	ruff.io/tio/pkg/sqlparser
ok  	ruff.io/tio/rule
ok  	ruff.io/tio/rule/process
ok  	ruff.io/tio/rule/sink/amqp
ok  	ruff.io/tio/rule/sink/influxdb
ok  	ruff.io/tio/shadow
ok  	ruff.io/tio/thing
ok  	ruff.io/tio/thing/api
```

All packages pass. `go build ./...` and `go vet ./...` clean.

## Concerns

1. `connector/nats/presence.go` `reconcile()` was enhanced to use both `queryAliveServers()` (for multi-server) and `Connz` (for single-server stale detection). The original Steps 1-4 code only used server-alive checks, which broke `TestReconcileFixesStaleEntries` because the test server is always alive.

2. `connector/nats/connectivity.go` `Remove()` now calls `Close` before `kv.Delete` with a 100ms sleep to avoid the disconnect handler re-creating the KV entry after deletion. The sleep is a pragmatic fix; a more robust solution would use a tombstone flag.
