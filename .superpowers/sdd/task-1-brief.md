# Task 1: Topic/Subject Conversion with Validation

## Context

This is the first implementation task for NATS integration in the tio IoT hub project. The project converts MQTT topics to NATS subjects for message routing. This task creates pure conversion functions with no external dependencies — it's a leaf dependency that other tasks build on.

## Project

- Module: `ruff.io/tio`
- Go version: 1.25
- Working directory: `/Users/liuyc/code/tio`
- Current baseline commit: `e56f273` (NATS deps pinned, all tests green)

## Files to Create

- `connector/nats/topic.go`
- `connector/nats/topic_test.go`

## Requirements

Implement three functions with these exact signatures:

```go
// MqttSubscriptionToNatsSubjects converts an MQTT subscription filter to NATS subjects.
// For "foo/#", returns ["foo", "foo.>"] because MQTT # matches the parent topic too.
// For other filters, returns a single-element slice.
func MqttSubscriptionToNatsSubjects(filter string) ([]string, error)

// MqttPublishTopicToNatsSubject converts a concrete MQTT publish topic to a NATS subject.
// Rejects any topic containing wildcards (+, #, *, >).
func MqttPublishTopicToNatsSubject(topic string) (string, error)

// NatsSubjectToMqttTopic converts a concrete NATS subject back to MQTT topic format.
// Only defined for concrete subjects (no wildcards).
func NatsSubjectToMqttTopic(subject string) (string, error)
```

## Conversion Rules

| MQTT | NATS | Notes |
|------|------|-------|
| `/` separator | `.` separator | Level separator |
| `+` (solo level) | `*` | Single-level wildcard |
| `#` (last level) | `>` | Multi-level wildcard |
| `foo/#` | `["foo", "foo.>"]` | Two subscriptions: parent + descendants |

## Validation Rules (MUST reject with error)

- Empty string
- Leading/trailing `/`
- Empty levels (consecutive `//`)
- Levels containing literal `.`
- Levels containing whitespace
- Levels containing `*` or `>` (NATS wildcards)
- `+` not occupying a complete level
- `#` not as the last complete level
- For publish topics: any wildcard (`+`, `#`, `*`, `>`) is rejected

## Test Cases Required

Must include:
- `foo/#` → `["foo", "foo.>"]`
- `#` → `["", ">"]` — wait, `#` alone should be `["", ">"]`? No. `#` means "everything". In MQTT, `#` alone matches all topics. In NATS, `>` matches all subjects. So `#` → `[">"]`.
- `foo/+/bar` → `["foo.*.bar"]`
- Exact topics: `$iothub/things/dev1/shadow/get` → `$iothub.things.dev1.shadow.get`
- All invalid forms listed above
- Round-trip: convert to NATS and back should return original

## Approach: TDD

1. Write the tests first (they should fail — functions don't exist yet)
2. Implement the minimal code to make tests pass
3. Run tests, verify all pass
4. Commit

## Verification Commands

```bash
go test ./connector/nats -run Topic -count=1 -v
go test ./...
git diff --check
```

## Report

Write your report to `/Users/liuyc/code/tio/.superpowers/sdd/task-1-report.md` with:
- Status: DONE / DONE_WITH_CONCERNS / NEEDS_CONTEXT / BLOCKED
- Commits created (short hashes)
- Test results (command + output summary)
- Any concerns

## Commit Message Format

```
feat(connector/nats): add MQTT topic to NATS subject conversion
```
