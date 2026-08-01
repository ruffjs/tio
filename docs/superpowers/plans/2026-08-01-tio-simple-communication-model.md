# Tio Simple Communication Model Implementation Plan

> Source of truth: `tio-simple-model-design.md`.
>
> Implement tasks in order. Keep changes surgical and run the listed verification before moving on.

## Goal

Implement the simple device protocol while allowing one startup-time choice between it and the legacy protocol:

- simple topics: `tio/{thingId}/up|down|event|data`;
- incremental Shadow merge with recursive objects and `null` deletion;
- opaque Method parameters and replies;
- one immutable JSON or CBOR codec for every enabled device MQTT protocol;
- one `protocol.mode: legacy|simple` selection;
- transparent Event/Data delivery.

## Non-negotiable implementation constraints

- Construct one codec at startup and inject it. Do not add a mutable global codec or runtime `SetCodec`.
- Use exactly one subscription/dispatcher for `tio/+/up`; it handles `report`, `get`, and `reply`.
- Never start a goroutine per message or allow unbounded DB concurrency. Reuse one bounded `ants` pool for DB-backed `report`/`get` work; handle `reply` directly in the dispatcher.
- Keep one pure Shadow merge function shared by desired and reported updates.
- Keep Method `d` opaque. Tio only reads `t`, `id`, and, for calls, writes `d.m`.
- Event/Data bytes must never be decoded and re-encoded by the forwarding path.
- Do not add additional protocol switches or modes.
- Reuse the existing connector, Shadow service, HTTP container, and integration-test fixture.

## Encoding scope

| Payload | Codec behavior |
|---|---|
| Simple Control `up/down` | Tio decodes/encodes with the configured codec |
| Legacy Shadow | Tio decodes/encodes with the configured codec |
| Legacy Method | Tio decodes/encodes with the configured codec |
| Legacy NTP | Tio decodes/encodes with the configured codec |
| Device-visible legacy Presence | Tio encodes with the configured codec |
| Simple Event/Data | Raw bytes; device and consumer use the configured codec |
| User-configured Rule MQTT source/sink | Raw user payload; no automatic transcoding |
| HTTP API, database JSON, connector internal NATS records | Unchanged; JSON where currently used |

---

## Task 1: Add immutable codec and protocol configuration

**Files**

- Modify: `go.mod`, `go.sum`
- Create: `pkg/codec/codec.go`
- Create: `pkg/codec/codec_test.go`
- Modify: `config/config.go`
- Modify: `config.default.yaml`
- Modify: `config/config_test.go`

### Steps

- [ ] Add `github.com/fxamacker/cbor/v2`.
- [ ] Define the minimal interface:

  ```go
  type Codec interface {
      Marshal(any) ([]byte, error)
      Unmarshal([]byte, any) error
  }

  func New(name string) (Codec, error)
  ```

- [ ] Implement JSON with `encoding/json`.
- [ ] Implement CBOR with immutable `EncMode`/`DecMode`. Configure `DecOptions.DefaultMapType` as `map[string]any`; nested values decoded through `any` must also use string-key maps.
- [ ] Do not expose registry, setter, mutex, or package-global current codec.
- [ ] Add a named `config.Protocol` structure:

  ```yaml
  protocol:
    encoding: json
    mode: legacy
  ```

- [ ] Add `Protocol.Validate()` or equivalent startup validation:
  - encoding is exactly `json` or `cbor`;
  - mode is exactly `legacy` or `simple`.
- [ ] Keep `mode: legacy` in the embedded default YAML for backward compatibility.

### Tests

- [ ] Run the same nested envelope round-trip table against JSON and CBOR.
- [ ] Assert CBOR `map -> any` produces `map[string]any` at every nested level.
- [ ] Assert arrays, booleans, signed/unsigned integers, floats, strings, and `null` decode without changing the logical model.
- [ ] Assert unknown encoding and unknown/empty mode are rejected.

### Verify

```bash
go test ./pkg/codec ./config
```

---

## Task 2: Apply the codec to every Tio-owned legacy device MQTT payload

**Files**

- Modify: `shadow/shadow.go`, `shadow/shadow_test.go`
- Modify: `shadow/method.go`, `shadow/method_test.go`
- Modify: `ntp/ntp.go`, `ntp/ntp_test.go`
- Modify: `connector/nats/connector.go`
- Modify: `connector/nats/presence.go`, `connector/nats/presence_test.go`
- Modify constructor call sites in `cmd/tio/main.go` and `integration_tests/setup_test.go`

### Steps

- [ ] Inject `codec.Codec` into `NewShadowHandler`, `NewMethodHandler`, and `NewNtpHandler`.
- [ ] Replace only MQTT payload `json.Marshal`/`json.Unmarshal` calls in those handlers with the injected codec.
- [ ] Pass the device codec and whether the selected mode is legacy into the NATS connector (or a small presence-specific dependency). Use it only for device-visible Presence payloads.
- [ ] Suppress device-visible legacy Presence publications in simple mode.
- [ ] In `publishMqttPresence`, handle codec marshal errors explicitly: log the error and return without publishing.
- [ ] Leave connector cluster records, disconnect-control messages, JetStream records, DB serialization, HTTP, and OpenAPI JSON unchanged.
- [ ] Preserve existing legacy topic names and message shapes under both encodings.

### Tests

- [ ] Convert existing handler tests to a JSON/CBOR table; do not duplicate test bodies.
- [ ] Verify legacy Shadow request and every Shadow response/notice round-trip in both codecs.
- [ ] Verify legacy Method request/reply and NTP request/reply in both codecs.
- [ ] Verify Presence payload is JSON or CBOR according to the injected codec and is not published in simple mode.

### Verify

```bash
go test ./shadow ./ntp ./connector/nats
```

---

## Task 3: Make Shadow merge and version semantics deterministic

**Files**

- Modify: `shadow/merge.go`, `shadow/merge_test.go`
- Modify: `shadow/service.go`, `shadow/service_test.go`

### Steps

- [ ] Add one pure merge operation that returns a new value and whether it changed; it must not mutate either input.
- [ ] Implement exactly these rules:
  - root patch must be an object;
  - absent fields remain unchanged;
  - object patches recurse;
  - if the old value is absent or non-object, an object patch starts from an empty object;
  - scalar and array values replace the previous value;
  - `null` deletes a field and is never persisted;
  - deleting an absent field is a no-op;
  - `{}` preserves an existing object and creates/replaces with an empty object otherwise.
- [ ] Keep metadata handling as a thin wrapper around the one merge result; do not create separate desired/reported merge implementations.
- [ ] In `setState`, compare the actual merged state:
  - changed desired: increment version once and persist;
  - unchanged desired: keep version, skip metadata mutation, `repo.Update`, and cache invalidation, but preserve the legacy no-op notification described below;
  - reported: never increment desired version;
  - preserve legacy accepted/rejected behavior expected by existing clients.
- [ ] Keep the current legacy notification contract: `notifyStateUpdate` still runs for a successful no-op desired update. The simple subscriber compares `Previous.State.Desired` with `Current.State.Desired` and filters that notification instead of changing legacy behavior.
- [ ] Use this control flow for the internal result; adapt names to the existing transaction code rather than introducing another state layer:

  ```go
  func (s *shadowSvc) setState(...) (Shadow, MetaValue, bool, error) {
      var pre Shadow
      var persisted *Shadow
      var updatedMeta MetaValue
      var changed bool

      err := s.repo.ExecWithTx(func(repo Repo) error {
          current, err := repo.Get(ctx, thingId)
          if err != nil { return err }
          pre = cloneShadow(*current)

          merged, didChange, err := MergePatch(selectedState(*current), selectedPatch(req))
          if err != nil { return err }
          changed = didChange

          if isDesired && !changed {
              // No persistence or cache invalidation. Keep a result so the
              // existing post-transaction legacy notification still runs.
              persisted = current
              return nil
          }

          updatedMeta = applyMergedStateAndMetadata(current, merged, req)
          if isDesired && changed { current.Version++ }

          persisted, err = repo.Update(ctx, thingId, req.Version, *current)
          return err
      })
      if err != nil { return Shadow{}, nil, false, err }

      // after commit, for legacy compatibility:
      s.notifyDeltaState(thingId, req.ClientToken, persisted)
      s.notifyStateUpdate(thingId, req.ClientToken, &pre, persisted) // also for successful no-op
      return *persisted, updatedMeta, changed, nil
  }
  ```

  `SetDesired` and `SetReported` keep their public signatures and ignore the internal `changed` result. Do not add a second notification API. This early return applies only to desired; reported updates retain their current persistence and metadata behavior even when their state value is unchanged.
- [ ] Keep optimistic version validation. Add a concurrent update test that waits with `WaitGroup`/`require.Eventually`; do not use assertions inside detached goroutines or fixed sleeps.
- [ ] Use the existing `Service.SubscribeUpdate` notification for the simple desired bridge later. Do not add a second notifier interface.

### Tests

- [ ] Table-test every merge rule, including nested deletion, array replacement, object-over-scalar, scalar-over-object, no-op update, and input immutability.
- [ ] Verify desired changes increment exactly once.
- [ ] Verify identical desired patches and deletion of absent keys do not increment.
- [ ] With a spy/mock repository and cache, verify a no-op desired update performs no `repo.Update` and no cache invalidation, while `notifyStateUpdate` still fires once with equal previous/current desired state.
- [ ] Verify reported changes persist without incrementing desired version.
- [ ] Run existing delta, metadata, accepted, rejected, and legacy Shadow tests unchanged where behavior is not intentionally revised.

### Verify

```bash
go test ./shadow
go test ./thing
```

---

## Task 4: Implement one simple-protocol handler

**Files**

- Create: `pkg/protocol/simple.go`
- Create: `pkg/protocol/simple_test.go`
- Create: `shadow/simple.go`
- Create: `shadow/simple_test.go`

### Protocol primitives

- [ ] Define exact topic builders and an exact parser for three-level topics. Reject missing/extra levels and wildcard thing IDs in concrete messages.
- [ ] Define the compact envelope:

  ```go
  type ControlMessage struct {
      Type string `json:"t"`
      ID   string `json:"id,omitempty"`
      Data any    `json:"d,omitempty"`
  }
  ```

- [ ] Keep JSON tags as the common field model used by both codecs.

### Single handler

- [ ] Implement one `SimpleHandler` holding only:
  - connector;
  - immutable codec;
  - Shadow service;
  - one bounded `ants` pool shared by DB-backed report/get work;
  - one mutex-protected pending-call map.
- [ ] `Start(ctx)` creates exactly one queue subscription to `tio/+/up`.
- [ ] The subscription decodes once and switches on `t`:
  - `report`: validate `d.version` and object `d.state`, then submit `SetReported` work to the bounded pool;
  - `get`: submit the Shadow read and complete `set` publication to the same bounded pool;
  - `reply`: correlate `thingId + id` and deliver the complete opaque `d`;
  - unknown/invalid messages: reject and log the reason; never silently ignore.
- [ ] Reuse the existing Shadow `ants` pattern and its established concurrency limit rather than creating a second pool abstraction. Release the pool on context cancellation.
- [ ] Do not create report/get channels, a goroutine per message, or separate report/get pools.
- [ ] Register one existing `Service.SubscribeUpdate` callback. Publish the full current desired state and version as `set` only when both conditions hold:
  - previous/current desired are different;
  - `DeltaState(current desired, current reported)` is non-empty.
- [ ] If desired changes to equal reported, persist it and increment version but do not actively publish `set`.
- [ ] A device `report` never triggers `set`; a device `get` always triggers a complete `set`, regardless of desired change or desired/reported delta.
- [ ] Implement `Invoke(ctx, thingId, method, params, timeout) (any, error)`:
  - require the thing to be online;
  - generate a UUID call ID;
  - shallow-copy params and set `d.m` after the copy so callers cannot override the method;
  - add one buffered pending channel before publish;
  - publish `call` to `down`;
  - remove pending state on reply, publish error, timeout, or context cancellation;
  - return the complete reply `d` without parsing business fields.

### Tests

- [ ] Run all handler tests for JSON and CBOR.
- [ ] Verify a reply received through the real dispatcher completes `Invoke`; tests must not call a private reply handler directly.
- [ ] Verify nested CBOR `d` values remain string-key maps.
- [ ] Verify `report`, `get`, desired-to-`set`, offline call, timeout, cancellation, late reply, duplicate reply, and concurrent distinct calls.
- [ ] Verify every successful/failed call leaves no pending entry.
- [ ] Verify one handler registration creates only one `tio/+/up` subscription.
- [ ] Verify DB-backed handler concurrency never exceeds the pool limit and no goroutine-per-message path exists.

### Verify

```bash
go test ./pkg/protocol ./shadow
go test -race ./shadow -run 'TestSimple|TestMerge'
```

---

## Task 5: Enforce protocol mode and directional ACLs

**Files**

- Modify: `auth/acl.go`, `auth/acl_test.go`
- Modify: `cmd/tio/main.go`
- Modify: `shadow/api/http.go`, `shadow/api/http_test.go`
- Create: `shadow/api/simple_method.go`, `shadow/api/simple_method_test.go`

### ACL

- [ ] Pass the selected protocol mode into `TopicAcl`.
- [ ] Match simple topics exactly; prefix-only authorization is forbidden.
- [ ] For a device itself or a gateway acting for a bound thing:
  - publish allowed only to `up`, `event`, and `data`;
  - subscribe allowed only to `down`.
- [ ] Reject all simple device access in legacy mode.
- [ ] Reject legacy reserved-topic device access in simple mode.
- [ ] Preserve explicit super-user access and existing gateway binding rules.
- [ ] Add positive and negative tests for direction, other thing IDs, invalid suffixes, extra levels, wildcards, both modes, and gateway bindings.

### HTTP routes

- [ ] Keep shared Thing and Shadow HTTP routes available for either selected protocol.
- [ ] Register the existing legacy Method route only when legacy is enabled.
- [ ] Register `POST /api/v1/things/{id}/invoke` only when simple is enabled.
- [ ] The simple request contains `method`, opaque `params`, and optional timeout. Its response returns the complete opaque reply `d`.
- [ ] Use the HTTP request context for cancellation.
- [ ] Add routes to the existing `thingWs` before applying its filters; do not append the same metrics/logging/auth filters more than once.

### Startup wiring

- [ ] Read and validate protocol mode and encoding before constructing protocol components.
- [ ] Construct one codec and pass the same instance to the NATS connector and every enabled handler.
- [ ] Use one explicit startup branch: when legacy is selected, start only legacy Shadow, Method, NTP, Presence, and legacy Method HTTP paths.
- [ ] In the other branch, start only one `SimpleHandler`, simple Event/Data access, and its HTTP Method route.
- [ ] Do not instantiate or register unselected protocol handlers merely to leave them idle.

### Verify

```bash
go test ./auth ./shadow/api ./cmd/tio
go test ./...
```

---

## Task 6: Verify transparent Event/Data delivery

**Files**

- Modify or create focused tests under `connector/nats/`
- Modify: `integration_tests/setup_test.go`
- Create: `integration_tests/simple_protocol_test.go`

### Steps

- [ ] Keep Event/Data forwarding as the connector's existing MQTT-topic to NATS-subject mapping; add no application decoder or forwarding worker.
- [ ] Document the corresponding NATS subjects (`tio.{thingId}.event` and `tio.{thingId}.data`) for consumers.
- [ ] In tests, subscribe as the business/data consumer before the device publishes.
- [ ] Compare received bytes with the exact published byte slice for both JSON and CBOR payloads.
- [ ] Read Shadow before and after Event/Data publication and assert it is unchanged.
- [ ] Verify QoS 1 Event and QoS 0 Data through MQTT.
- [ ] Verify ACL denial prevents device publication to simple topics in legacy mode.

### Verify

```bash
go test ./connector/nats
```

---

## Task 7: End-to-end acceptance tests in the existing fixture

**Files**

- Modify: `integration_tests/setup_test.go`
- Modify existing integration MQTT tests to use the test codec where they exercise device protocol payloads
- Create: `integration_tests/simple_protocol_test.go`

### Fixture rules

- [ ] Use the embedded connector's dynamic MQTT port and `httpSvr.URL`; do not hard-code ports `1883` or `9000`.
- [ ] Select the test codec from `TIO_TEST_ENCODING=json|cbor` before starting the fixture and inject it exactly as production does.
- [ ] Start the selected protocol in `TestMain` using the production wiring rules.
- [ ] Keep HTTP request/response bodies JSON.
- [ ] Use channels, `WaitGroup`, and `require.Eventually`; do not use testing assertions from goroutines that can outlive the test.

### Acceptance coverage

- [ ] Incremental report updates reported state, recursively merges objects, replaces arrays, and deletes `null` fields.
- [ ] Desired update with a non-empty desired/reported delta publishes one complete `set`; an identical update or a new desired equal to reported publishes none.
- [ ] `get` always returns the current complete `set` without changing version.
- [ ] Method call carries arbitrary params; reply is correlated through the uplink dispatcher and returns arbitrary nested `d` unchanged.
- [ ] Timeout and HTTP cancellation clean pending calls.
- [ ] Event and Data reach their respective consumers byte-for-byte and do not change Shadow.
- [ ] JSON and CBOR runs assert the same logical state/message results.
- [ ] Protocol selection is covered with focused startup/ACL tests:
  - legacy only is valid and exposes no simple path;
  - simple only is valid and exposes no legacy path;
  - unknown and empty modes are rejected before connector startup.

### Verify

```bash
TIO_TEST_PROTOCOL=legacy TIO_TEST_ENCODING=json go test -tags=integration ./integration_tests -run TestLegacy -count=1
TIO_TEST_PROTOCOL=legacy TIO_TEST_ENCODING=cbor go test -tags=integration ./integration_tests -run TestLegacy -count=1
TIO_TEST_PROTOCOL=simple TIO_TEST_ENCODING=json go test -tags=integration ./integration_tests -run TestSimple -count=1
TIO_TEST_PROTOCOL=simple TIO_TEST_ENCODING=cbor go test -tags=integration ./integration_tests -run TestSimple -count=1
```

Then run the full suite:

```bash
go test ./...
go test -tags=integration ./integration_tests -count=1
git diff --check
```

---

## Task 8: Update user documentation

**Files**

- Create: `docs/simple-protocol.md`
- Modify existing configuration/protocol documentation where legacy JSON-only behavior is described

### Required content

- [ ] Four simple topics, directions, and QoS.
- [ ] Compact Control envelope and all five message types.
- [ ] Incremental desired/reported merge rules, nested example, array replacement, and `null` deletion.
- [ ] Full-state `set` and reconnect `get` behavior.
- [ ] Opaque Method request/reply `d` and timeout behavior.
- [ ] JSON/CBOR global scope, including legacy Shadow/Method/NTP/Presence and raw Event/Data responsibilities.
- [ ] `protocol.mode` values, backward-compatible legacy default, invalid mode handling, and complete route/topic effects.
- [ ] Event/Data NATS subjects and byte-preserving contract.
- [ ] Explicit statement that HTTP, storage, broker internals, and arbitrary Rule payloads are not automatically converted.

### Final verification

- [ ] Check every acceptance condition in `tio-simple-model-design.md` has at least one named test.
- [ ] Confirm no device-protocol handler imports `encoding/json` directly except the JSON codec implementation.
- [ ] Confirm no mutable global codec or runtime codec setter exists.
- [ ] Confirm only one production subscription matches `tio/+/up`.
- [ ] Confirm the unselected protocol registers neither device subscriptions nor protocol-specific HTTP routes.

Useful audit commands:

```bash
rg -n 'encoding/json|json\.(Marshal|Unmarshal)' shadow ntp connector/nats
rg -n 'tio/\+/up|TopicAllUp' . --glob '*.go'
rg -n 'SetCodec|currentCodec|globalCodec' . --glob '*.go'
git diff --check
```

## Definition of done

The implementation is complete only when:

1. all unit and integration commands above pass for JSON and CBOR;
2. all 12 design acceptance conditions have effective assertions, not publish-only smoke tests;
3. exactly one protocol is selected at startup, with no subscriptions, publications, or routes from the unselected protocol;
4. no Event/Data forwarding path changes payload bytes;
5. the implementation contains one codec instance, one simple uplink dispatcher, and one Shadow merge implementation.
