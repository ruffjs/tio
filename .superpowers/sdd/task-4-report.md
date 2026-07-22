# Task 4 Report: Dynamic NATS Authentication with Least-Privilege Permissions

## Status
**DONE**

## Summary
Successfully implemented dynamic authentication and least-privilege permissions for the NATS-based connector in the tio IoT hub project. The implementation includes:

1. **Shared protocol-neutral auth contracts** (`connector/auth.go`)
   - `AuthContext`, `AuthResult`, `AuthzFn`, `AclFn`, `BindingGetter` types
   - Protocol-agnostic, no NATS or MQTT dependencies

2. **Updated MQTT auth to use shared types** (`auth/mqtt_auth.go`)
   - Migrated from `embed.AuthContext` to `connector.AuthContext`
   - Removed MQTT-specific `Clean` session check from auth logic
   - Clean session check moved to embed layer (`auth_hook.go`)

3. **NATS authenticator with least-privilege permissions** (`connector/nats/authenticator.go`)
   - Internal users: `$tio-app`, `$tio-sys`, `$tio-mqtt-publisher`
   - Dynamic thing authentication via `connector.AuthzFn`
   - Per-connection permissions using `c.RegisterUser()`
   - Thing permissions restricted to own topics only
   - System topics (`$SYS.>`) isolated to SYS account

4. **Comprehensive test coverage** (`connector/nats/authenticator_test.go`)
   - 9 test cases covering all authentication scenarios
   - Permission isolation tests
   - Anonymous rejection tests
   - All tests passing

## Files Created
- `connector/auth.go` - Shared auth contracts (26 lines)
- `connector/nats/authenticator.go` - NATS authenticator implementation (207 lines)
- `connector/nats/authenticator_test.go` - Test suite (247 lines)

## Files Modified
- `auth/mqtt_auth.go` - Migrated to shared types
- `auth/mqtt_auth_test.go` - Updated tests for shared types
- `auth/acl.go` - Updated to use `connector.BindingGetter`
- `connector/mqtt/embed/embed_broker.go` - Removed duplicate auth types
- `connector/mqtt/embed/auth_hook.go` - Updated to use shared types, added Clean session check
- `connector/mqtt/embed/auth_hook_test.go` - Updated tests
- `connector/mqtt/embed/embed_broker_test.go` - Updated tests
- `connector/mqtt/embed/embed_broker_wss_test.go` - Updated tests
- `cmd/tio/main.go` - Updated to use `connector.AuthzFn`

## Test Results
All tests passing:
```
✓ go test ./auth -count=1 -v
✓ go test ./connector/nats -run 'Auth|Permission' -count=1 -v
✓ go test ./connector/mqtt/embed -count=1
✓ go test ./... -count=1
```

## Key Design Decisions

1. **Protocol-neutral auth contracts**: The `connector.AuthContext` does not include MQTT-specific fields like `Clean`. This keeps the contracts reusable across protocols.

2. **Clean session check at protocol layer**: The MQTT-specific clean session validation is now in `auth_hook.go` where it belongs, not in the shared auth logic.

3. **Per-connection permissions**: Using `c.RegisterUser()` allows each connection to have its own permission set based on the authenticated thing ID.

4. **Least-privilege for things**: Each thing can only publish/subscribe to its own topics (`$iothub/things/<thingId>/...` and `$iothub/user/things/<thingId>/...`).

5. **Internal user separation**: Three internal users with distinct roles:
   - `$tio-app`: Broad app-level access
   - `$tio-sys`: System monitoring only
   - `$tio-mqtt-publisher`: Publish-only for MQTT gateway

## Commit Messages
```
feat(connector): add shared protocol-neutral auth contracts
feat(connector/nats): add dynamic NATS authentication with least-privilege permissions
```

## Concerns
None. Implementation is complete and all tests pass.
