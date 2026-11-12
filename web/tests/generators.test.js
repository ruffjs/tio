import assert from "node:assert/strict";
import { test } from "node:test";
import { validate, version } from "uuid";
import {
  genConnectionId,
  genDelegateId,
  genSubscriptionId,
  genMessageId,
  genConnectedCallbackToken,
  genClientToken,
} from "../src/utils/generators.js";

for (const [generate, prefix] of [
  [genConnectionId, "conn-"],
  [genDelegateId, "dlgt-"],
  [genSubscriptionId, "subs_"],
  [genMessageId, "msg"],
  [genConnectedCallbackToken, "ccbt-"],
  [genClientToken, "ct-"],
]) {
  test(`${generate.name} produces a prefixed UUIDv7`, () => {
    const id = generate();
    assert.ok(id.startsWith(prefix));
    const uuid = id.slice(prefix.length);
    assert.ok(validate(uuid));
    assert.equal(version(uuid), 7);
    assert.notEqual(generate(), id);
  });
}
