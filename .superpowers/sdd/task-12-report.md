# Task 12: Final Cleanup — Migrate Demos & Remove Old MQTT Client

## Status: DONE

## Summary

Completed the final cleanup after the NATS connector cutover. Migrated all remaining consumers of `connector/mqtt/client` to the new `internal/mqtttest` package, then deleted the old connector.

## Changes

### Created
- `internal/mqtttest/client.go` — minimal Paho-based MQTT test helper with `DeviceClient` type, `NewDeviceClient()`, `NewDeviceClientWithTLS()`, and `OnConnect()` support

### Modified
- `demos/light/device/main.go` — replaced `connector/mqtt/client` with `internal/mqtttest`, adapted Subscribe (removed ctx param) and Publish (returns error instead of token)
- `demos/light/server/main.go` — same migration pattern
- `demos/mtls/device/main.go` — migrated to `NewDeviceClientWithTLS`, removed `config` dependency
- `integration_tests/method_test.go` — replaced import, adapted API calls

### Deleted
- `connector/mqtt/client/client.go` (entire `connector/mqtt/` directory removed)

## Verification

| Check | Result |
|-------|--------|
| `grep -rn 'connector/mqtt' --include='*.go'` | No matches |
| `connector/mqtt/` directory | Removed |
| `grep -rn 'NewJobCenter\|JobCenter\|jobWs' cmd/` | No matches |
| `go mod tidy` | Clean |
| `gofmt -w` | Clean |
| `go vet ./...` | Clean |
| `go test ./...` | All pass |
| `git diff --check` | Clean |

## Notes

- Integration tests behind `//go:build integration` tag reference helper functions (`newThingMqttClient`, `ID()`, `crateThing()`, etc.) that were removed in the cutover commit. The import migration is complete, but these tests require the full test harness to be rebuilt for the NATS connector before they can run.
- `pkg/eventbus/` retained as specified (used for in-process presence broadcast).
