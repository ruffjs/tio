# Task 10: Single-Node NATS Acceptance Tests

## Status: DONE

## Commits created
- `9b569fd` — test: add single-node NATS acceptance tests

## Summary

Created comprehensive single-node acceptance tests for the NATS-based tio IoT hub, including a full integration test setup that starts the complete tio stack (NATS embedded server + MQTT gateway + HTTP API + shadow/thing services).

## Changes

### New files
- `integration_tests/nats_single_node_test.go` — 15 acceptance tests covering auth, messaging, shadow, presence, topic validation, and API
- `integration_tests/mtls_helpers_test.go` — helpers for mTLS integration tests (build tag: integration)

### Modified files
- `connector/nats/server.go` — added `MqttPort()` accessor for test use
- `connector/nats/authenticator.go` — **fixed bug**: NATS permissions used MQTT topic syntax (`/` separator) instead of NATS subject syntax (`.` separator). Also added presence event subscribe permissions and superuser broad-access permissions.
- `connector/nats/authenticator_test.go` — updated permission expectations to match new NATS subject format
- `integration_tests/setup_test.go` — complete rewrite: starts in-memory SQLite, NATS embedded server, shadow/thing services, HTTP API via httptest.Server
- `integration_tests/shadow_test.go` — fixed Subscribe calls to use 3-arg API (no ctx)
- `integration_tests/method_test.go` — fixed variable shadowing (`err :=` → `pubErr :=`)
- `integration_tests/config-test.yaml` — updated for NATS connector config
- `integration_tests/mtls_test.go` — fixed Subscribe calls to use 3-arg API

### Bug fix in authenticator
The `thingPermissions()` function was generating NATS permissions using MQTT topic syntax (`$iothub/things/{id}/>`). Since NATS uses `.` as the subject separator and the MQTT gateway translates `/` → `.`, the permissions never matched actual subjects. Fixed to use `$iothub.things.{id}.>`.

Also added:
- `$iothub.events.things.>` to device subscribe permissions (for presence events)
- Superuser broad-access permissions (`$iothub.>` for pub/sub)

## Test Results

### Unit tests (no build tag): `go test ./...`
All 19 packages pass, including:
- `ruff.io/tio/integration_tests` — 15 tests (all PASS)
- `ruff.io/tio/connector/nats` — all tests PASS (including updated permission test)
- `ruff.io/tio/shadow` — all tests PASS

### Integration tests (with build tag): `go test -tags=integration ./integration_tests`
Compiles successfully (mTLS tests require actual TLS certs to run).

### Test list (all passing)
| Test | Description |
|------|-------------|
| TestPasswordAuth | Valid password connects |
| TestInvalidPasswordRejected | Wrong password denied |
| TestCrossThingAccessDenied | Thing A can't subscribe to Thing B's topics |
| TestQoS1Delivery | PublishReliable delivers at QoS 1 |
| TestRetainedDelivery | Retained message delivered to late subscriber |
| TestRetainedClear | Empty retained payload clears retained |
| TestBroadcastSubscribe | Two subscribers receive same message |
| TestQueueSubscribe | Messages delivered to single subscriber |
| TestShadowSetDesiredAcceptance | Set desired via API, device gets delta |
| TestShadowSetReportedAcceptance | Device reports state, gets accepted |
| TestPresenceConnectDisconnect | Connect/disconnect events received |
| TestMultiplePresenceSubscribers | Multiple observers get events |
| TestWildcardSubscription | `#` receives exact and sub-level msgs |
| TestInvalidPublishTopic | Wildcards in publish return error |
| TestJobApiAbsent | Job endpoint returns 404 |
