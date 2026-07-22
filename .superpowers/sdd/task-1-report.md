# Task 1 Report: Topic/Subject Conversion with Validation

## Status
DONE

## Commits Created
- `b80de76` — feat(connector/nats): add MQTT topic to NATS subject conversion

## Test Results

### `go test ./connector/nats -run Topic -count=1 -v`
All 4 test functions pass (55 subtests total):
- `TestMqttSubscriptionToNatsSubjects` — 24 subtests (10 valid + 14 invalid)
- `TestMqttPublishTopicToNatsSubject` — 15 subtests (3 valid + 12 invalid)
- `TestNatsSubjectToMqttTopic` — 12 subtests (3 valid + 9 invalid)
- `TestRoundTrip` — 4 subtests

### `go test ./...`
All tests pass across the entire project. No regressions.

### `git diff --check`
Clean — no whitespace errors.

## Files Created
- `connector/nats/topic.go` — 3 exported functions + internal helpers
- `connector/nats/topic_test.go` — table-driven tests with 55 subtests

## Key Implementation Details
- `MqttSubscriptionToNatsSubjects`: converts `foo/#` → `["foo", "foo.>"]` (two subscriptions), `#` → `[">"]`
- `MqttPublishTopicToNatsSubject`: rejects all wildcards, converts `/` → `.`
- `NatsSubjectToMqttTopic`: concrete subjects only, converts `.` → `/`
- All validation rules enforced: empty, leading/trailing separators, empty levels, dots in levels, whitespace, NATS wildcards in MQTT, partial `+`/`#` levels

## Concerns
None.
