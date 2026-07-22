# Task 6 — Core NATS Connector

## Status
DONE

## Commits
- `a7d3a04` — feat(connector/nats): add core connector with two-phase initialization
- `926804e` — feat(connector/nats): add broadcast and queue subscriptions with lifecycle management

## Files Created
- `connector/nats/connector.go` — main Connector struct with two-phase initialization
- `connector/nats/subscription.go` — broadcast and queue subscription logic
- `connector/nats/connector_test.go` — lifecycle and publish tests
- `connector/nats/subscription_test.go` — subscription tests

## Test Results
All 17 new tests pass:

```
TestConnectorFullLifecycle
TestStartBeforeConfigureFails
TestConfigureAuthTwiceFails
TestPublishNatsCore
TestPublishReliableAndRetained
TestConnectorCleanShutdown
TestPublishRejectsWildcardTopic
TestConcurrentPublishSubscribe
TestBroadcastSubscribeBothReceive
TestQueueSubscribeDistributesMessages
TestHashSubscriptionReceivesDescendants
TestPublishRejectsWildcard
TestContextCancellationUnsubscribes
TestRepeatedSubscribeCancel
TestQueueSubscribeEmptyQueueFails
TestSubscribeInvalidTopicFails
TestConcurrentSubscribePublish
```

Full suite (`go test ./...`) — PASS, no regressions.

## Key Design Decisions
- MQTT port obtained from `Varz` after server start (handles ephemeral `-1` port)
- `cleanupLocked()` used by both `Shutdown()` and `Start()` error paths for consistent teardown
- Test topics use `$iothub/things/dev1/...` prefix to satisfy APP account permissions
- `defer` in `subscribe()` handles rollback on partial subscription failure
