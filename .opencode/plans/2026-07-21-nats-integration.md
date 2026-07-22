# NATS Integration Implementation Plan

> Design source: `docs/superpowers/specs/2026-07-21-nats-integration-design.md`
>
> Scope decision: Job is disabled in this phase. Do not initialize `JobCenter`, start its workers, or register Job APIs. Keep the Job package compiling and its unit tests passing.

## Delivery principles

- Keep every commit buildable and testable. Add new NATS code alongside the MQTT connector first; remove the old connector only in the atomic cutover task.
- Do not expose Paho through the new connector API. Production Tio may use it only inside the private MQTT publisher; tests and device demos may use a clearly named test/demo helper.
- Do not implement speculative compatibility layers. Temporary compatibility fields or adapters introduced for the migration must be removed in the cutover commit.
- Run `go test ./...` before and after each task. If the repository is not green at baseline, record the existing failures and ensure no task adds failures.
- Use real embedded-NATS connections for protocol, authentication, permissions, QoS, retained-message, and presence behavior. Unit mocks alone are not sufficient for those contracts.

## Definition of done

- APP and SYS clients connect to embedded NATS with least-privilege permissions; anonymous access is rejected.
- MQTT devices authenticate dynamically through Tio and receive per-device NATS permissions.
- Core NATS publish/subscribe and queue subscriptions replace the old connector API. Direct Method keeps its MQTT request/response correlation model; it does not use native NATS request/reply across the device boundary.
- MQTT device delivery preserves QoS 1 and retained-message behavior through the private Paho publisher.
- Presence is cluster-wide, fan-out, generation-safe, and reconciles node loss within the specified window.
- Shadow and NTP requests have one effective handler; direct-method responses remain broadcast.
- Rules use stable queue groups and NATS subjects.
- A three-node deployment has unique server names/store directories, one cluster name, route mTLS, and JetStream replicas appropriate to the topology.
- Job initialization, workers, and APIs are absent from the running server.
- `go test ./...`, focused race tests, and the NATS integration suite pass.

---

## Task 0: Establish the baseline and pin compatible dependencies

**Files:**

- Modify: `go.mod`
- Modify: `go.sum`

1. Capture the baseline:

   ```bash
   go test ./...
   git status --short
   ```

2. Pin versions compatible with the repository's Go 1.25 line. Do not use `@latest`:

   ```bash
   go mod edit -go=1.25.9
   go get github.com/nats-io/nats-server/v2@v2.12.8
   go get github.com/nats-io/nats.go@v1.51.0
   go mod tidy
   ```

   `nats-server/v2` 2.12.8 declares Go 1.25.9 and uses `nats.go` 1.51.0. Revisit the pins only if the project's supported Go version changes; do not silently pull a release requiring Go 1.26.

3. Verify:

   ```bash
   go test ./...
   go mod verify
   git diff --check
   ```

4. Commit only when the baseline remains green (or has only the recorded pre-existing failures).

---

## Task 1: Add topic/subject conversion and publish validation

**Files:**

- Create: `connector/nats/topic.go`
- Create: `connector/nats/topic_test.go`

Implement and test these explicit contracts:

- MQTT topic level separator `/` maps to NATS `.`.
- Subscription filter `+` maps to `*`.
- Subscription filter `foo/#` expands to both `foo` and `foo.>` so it includes the parent topic.
- `#` is valid only as the final complete level; `+` must occupy a complete level.
- Empty levels, leading/trailing separators, NATS wildcards supplied by callers, whitespace, and malformed UTF-8 are rejected.
- Publish topics reject both MQTT wildcards (`+`, `#`) and NATS wildcards (`*`, `>`).
- Subject-to-MQTT conversion is defined only for concrete subjects.

Prefer separate entry points so callers cannot accidentally use subscription semantics for publishing, for example:

```go
func MqttSubscriptionToNatsSubjects(filter string) ([]string, error)
func MqttPublishTopicToNatsSubject(topic string) (string, error)
func NatsSubjectToMqttTopic(subject string) (string, error)
```

Tests must include `foo/#`, `#`, `foo/+/bar`, exact topics, and all invalid forms. Do not add a deliberately failing placeholder test.

Verify:

```bash
go test ./connector/nats -run Topic -count=1
go test ./...
git diff --check
```

---

## Task 2: Add NATS configuration without breaking the existing runtime

**Files:**

- Modify: `config/config.go`
- Modify: `config/config_test.go`
- Modify: configuration examples and test fixtures that instantiate `config.Config`

Add the spec's NATS configuration types and validation additively. Preserve legacy MQTT fields until Task 8 so intermediate commits remain green.

Validation must reject:

- empty or reused server identity where uniqueness can be determined locally;
- multi-node routes with SQLite storage;
- a cluster with inconsistent/missing cluster name;
- missing or shared JetStream store directories in a multi-node deployment;
- invalid replica counts (single node must use 1; production three-node topology uses 3);
- MQTT gateway enabled without exactly one listener mode for the node;
- both plaintext and TLS MQTT listeners on the same node;
- route TLS configured without complete CA/certificate/key inputs.

Do not replace the entire config file or reformat unrelated configuration.

Verify:

```bash
go test ./config -count=1
go test ./...
git diff --check
```

---

## Task 3: Start an embedded NATS server with APP/SYS accounts

**Files:**

- Create: `connector/nats/server.go`
- Create: `connector/nats/server_test.go`
- Create: `connector/nats/test_helpers_test.go`

Implement server construction and lifecycle separately from the connector. Tests must provide an explicit test authenticator; never start the server with anonymous/no authentication.

Required behavior:

- JetStream enabled with the configured store directory.
- Separate APP and SYS accounts.
- Unique `ServerName`, shared `ClusterName`, configured routes, and route mTLS.
- One optional MQTT listener per node, either plaintext or TLS.
- Clean readiness wait and shutdown.
- No global mutable NATS server options.

Tests must assert account isolation, anonymous rejection, JetStream availability, server/cluster names, store directory usage, listener mode validation, and clean shutdown. Add a restart test using the same node store to catch persistence/configuration mistakes.

Verify:

```bash
go test ./connector/nats -run 'Server|Account|JetStream' -count=1
go test ./...
git diff --check
```

---

## Task 4: Implement dynamic authentication and least-privilege permissions

**Files:**

- Create: `connector/auth.go` (shared protocol-neutral auth contracts)
- Create: `connector/nats/authenticator.go`
- Create: `connector/nats/authenticator_test.go`
- Modify: `auth/mqtt_auth.go`
- Modify: `auth/mqtt_auth_test.go`

Define the shared `AuthContext`, `AuthResult`, `AuthzFn`, and ACL/binding result in `connector`, not in `connector/nats`, to avoid an import cycle and keep domain authentication independent of the transport implementation.

`NatsAuthenticator` implements only `server.Authentication`:

```go
func (a *NatsAuthenticator) Check(c server.ClientAuthentication) bool
```

`server.ClientAuthentication` is supplied by NATS; it is not implemented by `NatsAuthenticator`. Read credentials, TLS state, connection type, and remote address from `c`, invoke the configured auth function, create a per-connection `server.User`, and bind it with `c.RegisterUser(user)`.

Cover distinct principals and permissions for:

- internal APP client;
- internal SYS client;
- private MQTT publisher (publish-only; no subscribe permission);
- dynamic device password authentication;
- device mTLS authentication;
- provisioning credentials;
- normal authenticated device credentials.

Permissions must prevent cross-thing access and access to `$SYS`, `$JS`, `$KV`, and `$tio.control` unless explicitly required by that internal principal. Include the MQTT gateway's required `$MQTT.sub.>` response subjects without granting arbitrary system access.

Use real NATS and MQTT connections to prove allow/deny behavior. Also unit-test auth timeouts and errors, but do not treat mock-only tests as permission coverage.

Binding/ACL changes are applied on connection. When a gateway binding changes, force that gateway connection to reconnect so its registered permissions are regenerated.

Verify:

```bash
go test ./connector/nats -run 'Auth|Permission|Provision|TLS' -count=1
go test ./auth -count=1
go test ./...
git diff --check
```

---

## Task 5: Implement the private MQTT QoS/retained publisher

**Files:**

- Create: `connector/nats/mqtt_publisher.go`
- Create: `connector/nats/mqtt_publisher_test.go`

Wrap Paho privately; no Paho type may appear in a public connector signature. The publisher authenticates as the dedicated internal MQTT publisher principal and supports:

- QoS 0 publish;
- QoS 1 publish with completion/error propagation;
- retained publish;
- zero-byte retained publish to clear retained state;
- bounded connect/publish timeouts;
- reconnect and clean shutdown.

Integration tests must verify behavior from an MQTT client's perspective:

- a QoS 1 subscriber receives at QoS 1;
- a late subscriber receives the retained message;
- clearing retained state prevents later delivery;
- publish failures and timeouts are returned;
- the publisher cannot subscribe or publish outside its allowed device-delivery subjects.

Receiving the message through a NATS subscription alone does not prove MQTT QoS or retained semantics.

Verify:

```bash
go test ./connector/nats -run 'MQTTPublisher|QoS|Retain' -count=1
go test ./...
git diff --check
```

---

## Task 6: Implement the NATS connector core alongside the legacy connector

**Files:**

- Create: `connector/nats/connector.go`
- Create: `connector/nats/connector_test.go`
- Create: `connector/nats/subscription.go`
- Create: `connector/nats/subscription_test.go`

Use two-phase initialization to break the authentication/service dependency cycle:

1. `NewNatsConnector` constructs resources but does not accept clients.
2. Construct `thingSvc` with the connector.
3. `ConfigureAuth` installs the fully constructed auth callbacks.
4. `Start` starts the server/listeners and internal APP/SYS/MQTT clients.

Until Task 8, do not add a compile-time assertion against the old `connector.Connector` interface.

Implement:

- `Publish` through APP NATS;
- `PublishReliable` and `PublishRetained` through the private MQTT publisher;
- broadcast `Subscribe`;
- load-balanced `QueueSubscribe`;
- idempotent unsubscribe and automatic unsubscribe on context cancellation;
- connection draining and deterministic shutdown.

`Message.Topic()` remains the public accessor. Convert concrete NATS subjects back to MQTT topics at the connector boundary.

Tests must prove:

- two broadcast subscribers both receive;
- two queue subscribers collectively receive each message exactly once;
- `foo/#` receives both `foo` and descendants;
- all publish APIs reject wildcard topics;
- repeated subscribe/cancel returns server subscription count to baseline;
- `Start` before `ConfigureAuth` fails closed;
- `Close` is idempotent.

Verify with the race detector:

```bash
go test -race ./connector/nats -run 'Connector|Subscribe|Queue' -count=1
go test ./...
git diff --check
```

---

## Task 7: Implement cluster-wide presence and generation-safe disconnect

**Files:**

- Create: `connector/nats/presence.go`
- Create: `connector/nats/presence_test.go`
- Create: `connector/nats/connectivity.go`
- Create: `connector/nats/connectivity_test.go`

Presence requirements:

- Subscribe via SYS to client connect/disconnect events and server-shutdown events.
- Persist device ownership/generation in JetStream KV using CAS.
- Only the successful CAS winner publishes the resulting presence transition and mutates the local/eventbus view.
- `SubscribePresence(ctx)` is fan-out: every subscriber receives every transition.
- Use a bounded eventbus. On slow-consumer overflow, trigger a full state sync instead of blocking SYS ingestion indefinitely.
- Reconcile every 10 seconds. Mark disconnected only after two consecutive misses.
- Handle server/node shutdown immediately when safe, while preserving generation checks.

`Connectivity.Close` publishes `$tio.control.<serverId>.disconnect`. The owning server resolves the current connection and invokes `DisconnectClientByID`. The control message must include/validate the observed generation so a delayed close cannot disconnect a newer reconnect. `Remove` also removes persisted state with CAS semantics.

Tests must cover:

- three presence subscribers all receive the same connect/disconnect events;
- duplicate SYS events produce one state transition;
- stale disconnect after reconnect does not delete the new generation;
- delayed cross-node `Close` does not disconnect a newer generation;
- owner-node loss becomes disconnected after two missed reconciliation rounds (target: about 20 seconds);
- slow subscriber recovery performs a full sync;
- context cancellation removes presence subscriptions and goroutines;
- concurrent connect/disconnect/CAS races under `go test -race`.

Verify:

```bash
go test -race ./connector/nats -run 'Presence|Connectivity|Generation|Reconcile' -count=1
go test ./...
git diff --check
```

---

## Task 8: Prepare protocol-neutral test fixtures before the cutover

**Files:**

- Create: `connector/mock/connector.go`
- Create: `connector/mock/message.go`
- Create: `connector/mock/subscription.go`
- Create: `internal/mqtttest/client.go`
- Create: tests for the new fixtures

Add a protocol-neutral connector mock implementing the future interface. Migrate no production code yet. It must support deterministic broadcast subscriptions, queue subscriptions, reliable/retained publish recording, presence fan-out, and context cancellation.

Add a repository-internal MQTT device helper only for integration tests and demos. It may use Paho, but it is not part of the production connector or server runtime API. Keep it minimal: connect/authenticate, subscribe with requested QoS, publish, retained publish, and close.

This task is additive and must leave the old suite green.

Verify:

```bash
go test ./connector/mock ./internal/mqtttest -count=1
go test ./...
git diff --check
```

---

## Task 9: Perform one atomic connector cutover

This is the only intentionally broad task. Do not commit an intermediate red state. Keep the migration in one working tree change until `go test ./...` passes.

**Core API/config/runtime:**

- Modify: `connector/connector.go`
- Modify: `config/config.go`
- Modify: `main.go`
- Modify: all construction/configuration tests

Change the public connector API to the spec, including `QueueSubscribe`, `SubscribePresence(ctx)`, and `Message.Topic()`. Remove temporary legacy MQTT configuration only after all callers use NATS configuration.

Wire startup in this exact order: construct NATS connector resources, construct services, configure auth, start NATS connector, then start consumers. Shut down in reverse order with draining.

Explicitly disable Job for this phase:

- do not construct or start `JobCenter`;
- do not construct/register Job manager services or WebSocket/HTTP Job APIs;
- keep Job package code and tests compiling;
- add an HTTP/router test proving the Job endpoint is absent.

**Consumers:**

- Modify: `shadow/**`
- Modify: `ntp/**`
- Modify: `method/**`
- Modify: `thing/**`
- Modify: `gateway/**`
- Modify: `rule/**`
- Modify: related tests

Use queue subscriptions for shadow and NTP request handlers. Use stable rule queue group `tio-rule-<ruleId>` for rule sources.

Direct Method deliberately does not use native NATS request/reply end to end: an MQTT device cannot consume the NATS `Reply` header or publish to a generated NATS inbox. Preserve this flow:

1. The API-handling instance registers a local pending entry keyed by `(thingId, clientToken)` before publishing.
2. It sends the device request with `PublishReliable` to the fixed MQTT method request topic.
3. The device echoes `clientToken` on the fixed MQTT method response topic.
4. Every Tio instance receives the response through ordinary broadcast `Subscribe`—never `QueueSubscribe`—and only the instance owning the matching local pending entry completes the waiter.

The long-lived response subscription must be established and flushed before the instance reports API readiness. Reject a duplicate in-flight `(thingId, clientToken)` instead of silently overwriting its waiter. Context cancellation or timeout must remove the pending entry. Receiving a response with no local waiter is normal on non-owning instances and should be ignored or debug-logged, not warned as an operational fault.

Resolve ACLs unequivocally: `TopicAcl` remains an MQTT-domain policy helper, but NATS does not call it on every publish. At authentication time, convert explicit authorized MQTT topic patterns and gateway bindings into the registered NATS user's permissions. Binding changes force gateway reconnect.

Migrate `rule/connector/mqtt.go` as well as MQTT rule sources/sinks; it imports the legacy client and must not be missed.

**Mocks, tests, and demos:**

Migrate all imports of `connector/mqtt/mock` to `connector/mock`, including Job, shadow, method, NTP, rule, and gateway tests. Migrate integration tests and device demos from `connector/mqtt/client` to `internal/mqtttest` or a small demo-local wrapper.

Before deletion, prove no remaining import exists:

```bash
rg 'connector/mqtt|mqtt/(client|mock)' --glob '*.go'
```

Then delete the legacy connector files explicitly (avoid a broad recursive delete), run `go mod tidy`, and verify:

```bash
go test -race ./connector/... ./auth/... ./shadow/... ./ntp/... ./method/... ./thing/... ./gateway/... ./rule/... ./job/...
go test ./...
go vet ./...
git diff --check
```

Only now add the compile-time assertion that `*nats.Connector` implements `connector.Connector`, and commit the cutover.

---

## Task 10: Add single-node acceptance tests

**Files:**

- Create or modify: `integration_tests/nats_single_node_test.go`
- Modify: integration test setup/helpers

Add black-box tests for the specification's observable behavior:

- dynamic password, mTLS, provisioning, and normal device authentication;
- cross-thing and system-subject authorization denial;
- QoS 1 delivery observed by MQTT;
- retained delivery to a late subscriber and retained clear;
- `foo/#` parent-topic behavior and invalid publish filters;
- shadow/NTP single effective handler;
- direct-method response broadcast;
- a Direct Method request accepted by node A completes when the device is connected through node B and publishes its response through node B;
- non-owning nodes receiving that response do not consume it away from node A and do not emit warning-level noise;
- duplicate in-flight `(thingId, clientToken)` is rejected, and timeout/cancellation removes its pending entry;
- all presence subscribers receive events;
- wildcard subscription cleanup returns subscription count to baseline;
- Job API is absent and no Job worker starts.

Do not rely on sleeps when a readiness event or bounded polling assertion is possible.

Verify:

```bash
go test -race ./integration_tests -run 'NATS|MQTT|Presence|Shadow|NTP|Method|JobDisabled' -count=1
go test ./...
```

---

## Task 11: Add three-node cluster and failure acceptance tests

**Files:**

- Create: `deployments/nats-cluster/docker-compose.yml`
- Create: `deployments/nats-cluster/README.md`
- Create: route/client certificate fixtures or a deterministic generation script
- Create or modify: `integration_tests/nats_cluster_test.go`

The deployment must define:

- three unique `ServerName` values;
- one shared `ClusterName`;
- separate persistent JetStream volumes/store directories;
- route mTLS with CA verification;
- one MQTT listener mode per node;
- shared MySQL storage (SQLite must be rejected for this topology);
- JetStream replicas = 3;
- health/readiness checks.

The cluster suite must prove:

- all three nodes form one cluster;
- APP/SYS account isolation remains intact across routes;
- queue consumers produce one effective shadow/NTP handler result;
- stable rule queue groups survive node restart;
- killing the device-owning node leads to disconnected presence after two reconciliation misses (about 20 seconds);
- a delayed cross-node close cannot disconnect a newer generation after reconnect;
- JetStream/KV state survives a node restart;
- startup rejects multi-node routes with SQLite.

Document exact setup, certificate generation, test command, teardown, and how to inspect failures. Tests must use independent temporary data/ports so local runs do not corrupt developer state.

Verify:

```bash
docker compose -f deployments/nats-cluster/docker-compose.yml config
go test -race ./integration_tests -run 'NATSCluster|NodeLoss|Generation|SQLiteRejected' -count=1
```

---

## Task 12: Final cleanup and documentation

**Files:**

- Modify: user/configuration documentation
- Modify: deployment documentation
- Delete: migration-only compatibility code

1. Remove legacy MQTT server configuration, adapters, constants, and tests that are obsolete after the cutover.
2. Confirm production code imports Paho only from the private MQTT publisher:

   ```bash
   rg 'paho|eclipse/paho' --glob '*.go'
   ```

   Test/demo helpers are allowed; any other production import requires an explicit design justification.

3. Confirm there are no legacy connector imports or accidental Job startup/routes:

   ```bash
   rg 'connector/mqtt|NewJobCenter|JobCenter|jobWs|job.*Register' --glob '*.go'
   ```

4. Run the full final gate:

   ```bash
   gofmt -w connector/nats/*.go connector/mock/*.go internal/mqtttest/*.go
   go mod tidy
   go test -race ./connector/... ./auth/... ./shadow/... ./ntp/... ./method/... ./thing/... ./gateway/... ./rule/... ./job/...
   go test ./...
   go vet ./...
   git diff --check
   ```

5. Review the final diff against the design document. Every changed line must be attributable to NATS integration, the connector cutover, required tests/docs, or disabling Job for this phase.

## Commit policy

- Tasks 0–8, 10–12 may be committed independently only when their verification gates are green.
- Task 9 is one atomic commit; do not commit the repository while interfaces and callers disagree.
- Never commit generated credentials, node data, JetStream stores, or local test databases.
