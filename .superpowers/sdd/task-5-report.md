# Task 5: Private MQTT QoS/Retained Publisher

## Status
DONE

## Commits Created
- `7337dac` feat(connector/nats): add private MQTT QoS 1/retained publisher

## Files Changed
- `connector/nats/mqtt_publisher.go` (new) — `mqttPublisher` struct with `Connect`, `Publish`, `Disconnect`
- `connector/nats/mqtt_publisher_test.go` (new) — 9 tests covering all required scenarios

## Implementation Summary

### `mqttPublisher`
- Connects to `tcp://127.0.0.1:<mqttPort>` via Paho with client ID `$tio-mqtt-pub-<serverName>`
- Auto-reconnect enabled with 1s retry interval
- `Publish` validates topic via `MqttPublishTopicToNatsSubject`, then publishes through Paho with 5s token timeout
- All Paho types are internal to `connector/nats/` — no Paho types in public signatures
- `Disconnect` performs clean disconnect with 250ms quiesce

### Tests (9 passing)
| Test | Description |
|------|-------------|
| `TestMQTTPublisher_Connect` | Basic connect succeeds |
| `TestMQTTPublisher_ConnectContextCancel` | Context cancellation returns error |
| `TestMQTTPublisher_QoS1Delivery` | QoS 1 publish → subscriber receives at QoS 1 |
| `TestMQTTPublisher_QoS0Delivery` | QoS 0 publish → subscriber receives message |
| `TestMQTTPublisher_RetainedMessage` | Retained message delivered to late subscriber |
| `TestMQTTPublisher_ClearRetained` | Empty retained payload clears retained message |
| `TestMQTTPublisher_PublishAfterDisconnect` | Publish after disconnect returns error |
| `TestMQTTPublisher_InvalidTopic` | Invalid topics (empty, spaces, wildcards) rejected |
| `TestMQTTPublisher_Reconnect` | Auto-reconnect after server restart, publish succeeds |

### Test Infrastructure
- `mqttTestAuth` — test authenticator that assigns all clients to the APP account (with JetStream for MQTT retained/QoS)
- `startMqttTestServer` — helper that starts a NATS server with MQTT gateway and wires up auth
- `testMqttSubscriber` — creates a Paho MQTT subscriber for verification
- `freeTCPPort` — finds a free TCP port for the reconnect test

## Test Results
```
=== RUN   TestMQTTPublisher_Connect                    --- PASS (0.02s)
=== RUN   TestMQTTPublisher_ConnectContextCancel        --- PASS (0.00s)
=== RUN   TestMQTTPublisher_QoS1Delivery                --- PASS (0.03s)
=== RUN   TestMQTTPublisher_QoS0Delivery                --- PASS (0.02s)
=== RUN   TestMQTTPublisher_RetainedMessage             --- PASS (0.52s)
=== RUN   TestMQTTPublisher_ClearRetained               --- PASS (1.83s)
=== RUN   TestMQTTPublisher_PublishAfterDisconnect      --- PASS (0.02s)
=== RUN   TestMQTTPublisher_InvalidTopic                --- PASS (0.02s)
=== RUN   TestMQTTPublisher_Reconnect                   --- PASS (1.14s)
PASS  ok  ruff.io/tio/connector/nats  4.017s
```

Full suite: `go test ./...` — all packages pass.

## Concerns
None.
