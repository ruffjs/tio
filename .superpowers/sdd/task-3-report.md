# Task 3 Report: Embedded NATS Server with APP/SYS Accounts

## Status: DONE

## Commits Created

- `046499b` - feat(connector/nats): add embedded NATS server with APP/SYS accounts

## Files Created

1. `connector/nats/server.go` - Embedded NATS server implementation
2. `connector/nats/server_test.go` - Comprehensive test suite (10 tests)
3. `connector/nats/test_helpers_test.go` - Shared test utilities

## Test Results

```
=== RUN   TestServerStartsAndIsRunning          --- PASS
=== RUN   TestServerAccountsExist               --- PASS
=== RUN   TestAccountIsolation                  --- PASS
=== RUN   TestAnonymousRejected                 --- PASS
=== RUN   TestJetStreamAvailable                --- PASS
=== RUN   TestServerName                        --- PASS
=== RUN   TestStoreDirUsed                      --- PASS
=== RUN   TestCleanShutdown                     --- PASS
=== RUN   TestRestartWithSameStoreDir           --- PASS
=== RUN   TestMqttGatewayPortListening          --- PASS
```

All 10 new tests pass. All existing tests in the package pass. Full project build succeeds.

## Implementation Notes

1. **Account JetStream**: Pre-created accounts in `Options.Accounts` do NOT auto-enable JetStream. After server start, `account.EnableJetStream(nil, nil)` must be called on the APP account.

2. **Cluster configuration**: Only applied when `ClusterPort != 0 || len(Routes) > 0`. In single-node mode, cluster config is skipped to avoid JetStream clustering mode which requires routes.

3. **MQTT gateway**: Configured when `MqttPort != 0`. Port -1 gives a random port in tests.

4. **WebSocket**: Only configured when port > 0 or == -1, because NATS server requires TLS for WebSocket listeners.

5. **StoreDir**: The server appends `/jetstream` to the configured StoreDir path.

6. **Authentication**: Custom authenticator uses `server.Authentication` interface. Test uses `accountAuth` that maps users to accounts via `RegisterUser()`.

## Concerns

- ClusterName config is ignored in single-node mode (no cluster port/routes). This is correct behavior.
- The `accountAuth` test helper requires account references to be set after server start (accounts are created during `StartNatsServer`). This is a test-specific pattern.
