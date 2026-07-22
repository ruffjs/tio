# Task 2: NATS Configuration Types

## Status: DONE

## Commit
- `895fcff` feat(config): add NATS configuration types and validation

## Changes

### config/config.go
- Added `ConnectorNats = "nats"` constant
- Added types: `NatsConfig`, `NatsServerConfig`, `NatsTLSConfig`, `NatsClientConfig`, `NatsTLSClientConfig`
  - Note: Named TLS types `NatsTLSConfig` / `NatsTLSClientConfig` to avoid collision with existing `TLSConfig` (used by MQTT client)
- Added `Nats NatsConfig` field to `Connector` struct
- Added `ValidateNatsConfig(cfg NatsConfig, dbType string) error` function

### config.default.yaml
- Added full `nats:` block under `connector:` with sensible single-node defaults

### config/config_test.go
- Added 12 test cases covering all validation rules

## Test Results
```
=== RUN   TestConfigMarshalOmitsSecrets          --- PASS
=== RUN   TestNatsConfigMarshalOmitsSecrets       --- PASS
=== RUN   TestValidateNatsConfig_ValidSingleNode   --- PASS
=== RUN   TestValidateNatsConfig_ValidMultiNodeWithMySQL --- PASS
=== RUN   TestValidateNatsConfig_EmptyServerName   --- PASS
=== RUN   TestValidateNatsConfig_EmptyClusterName  --- PASS
=== RUN   TestValidateNatsConfig_MultiNodeWithSQLite --- PASS
=== RUN   TestValidateNatsConfig_InvalidReplicaCountSingleNode --- PASS
=== RUN   TestValidateNatsConfig_InvalidReplicaCountMultiNode --- PASS
=== RUN   TestValidateNatsConfig_PlaintextAndTLSMqtt --- PASS
=== RUN   TestValidateNatsConfig_IncompleteRouteTLS --- PASS
=== RUN   TestValidateNatsConfig_EmptyClientUser   --- PASS (3 subtests)
=== RUN   TestValidateNatsConfig_EmptyStoreDir     --- PASS
PASS
```

Full suite: `go test ./...` — all packages pass, no regressions.

## Concerns
- **TLSConfig naming conflict**: The spec defined `TLSConfig` and `TLSClientConfig` but the existing codebase already has a `TLSConfig` struct (for MQTT client TLS). Renamed to `NatsTLSConfig` and `NatsTLSClientConfig` to avoid collision. This is additive-only as required.
- All legacy MQTT fields are preserved; no existing types or fields were modified or removed.
