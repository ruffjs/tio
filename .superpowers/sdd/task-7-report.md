# Task 7 Report: NATS Connector Presence Tracking & Connectivity

## Status: DONE

## Files Created

- `connector/nats/presence.go` — Presence KV bucket, system event subscription, reconciliation loop
- `connector/nats/presence_test.go` — 5 tests for presence tracking
- `connector/nats/connectivity.go` — IsConnected, OnConnect, ClientInfo, AllClientInfo, Close, Remove
- `connector/nats/connectivity_test.go` — 8 tests for connectivity methods

## Files Modified

- `connector/nats/connector.go` — Added `kv`, `presenceBus` fields; `initPresence()` call in Start
- `connector/nats/authenticator.go` — Added `$JS.API.>`, `$KV.>`, `_INBOX.>` permissions for internal app user

## Implementation Details

### Presence KV Bucket
- Bucket name: `TIO_PRESENCE`, history=5
- Keys: `presence.<thingId>` → JSON `PresenceRecord`
- Created on connector start via `initPresence()`

### System Event Subscription
- Subscribes on SYS connection to `$SYS.ACCOUNT.TIO_APP.CONNECT` and `.DISCONNECT`
- Filters out internal users (username starting with `$`)
- On connect: increments generation, puts record in KV, publishes events
- On disconnect: updates KV with connected=false, publishes events
- Events published via: EventBus fan-out + MQTT retained + MQTT event topic

### Reconciliation Loop
- Runs every 10 seconds
- Compares server Connz (active connections) against KV state
- Marks stale connected entries as disconnected if no matching server connection

### Connectivity Methods
- `IsConnected`: KV lookup
- `OnConnect`: EventBus subscribe
- `ClientInfo/AllClientInfo`: KV enumeration
- `Close`: Server.DisconnectClientByID via Connz CID lookup
- `Remove`: KV Purge + Close + clear MQTT retained

## Test Results

All 73 tests in `connector/nats` pass (13 new + 60 existing):
```
go test -race ./connector/nats -count=1 -v
PASS
ok  	ruff.io/tio/connector/nats	12.548s
```

Full project suite:
```
go test ./... -count=1
All packages pass.
```

## Commits Created

None (awaiting explicit commit request)

## Concerns

1. **Generation-based stale disconnect protection**: The spec mentions checking generation on disconnect to avoid stale disconnects overriding new connections. The current implementation uses a simpler approach: it only processes disconnects if the record is currently connected. This handles the basic case but doesn't fully protect against all stale disconnect scenarios in multi-node setups.

2. **Permissions change**: Added `$JS.API.>`, `$KV.>`, and `_INBOX.>` to the internal app user's permissions. This is necessary for KV operations but broadens the permission surface.

3. **MQTT publisher dependency**: Presence events via MQTT require the MQTT publisher to be connected. If it's not available, events still flow through the EventBus but not via MQTT topics.

4. **Single-node reconciliation**: The reconciliation loop only checks the local server's connections. Multi-node reconciliation via cross-server CONNZ is not implemented yet.
