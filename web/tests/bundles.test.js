import assert from "node:assert/strict";
import { test } from "node:test";
import { build } from "vite";

// Run from web/: node --test tests/bundles.test.js
test("production bundles load routes and tools on demand", async () => {
  const result = await build({ build: { write: false }, logLevel: "silent" });
  const chunks = result.output.filter(file => file.type === "chunk");
  const byName = new Map(chunks.map(chunk => [chunk.fileName, chunk]));
  const reachable = (roots) => {
    const visited = new Set();
    const visit = name => {
      if (visited.has(name)) return;
      visited.add(name);
      for (const dependency of byName.get(name)?.imports ?? []) visit(dependency);
    };
    for (const root of roots) visit(root.fileName);
    return [...visited].map(name => byName.get(name)).filter(Boolean);
  };
  const modules = chunks => chunks.flatMap(chunk => Object.keys(chunk.modules).filter(id => !id.endsWith(".css")));
  const entry = chunks.find(chunk => chunk.isEntry);
  const initial = reachable([entry]);
  const initialModules = modules(initial);
  for (const deferred of [
    "/components/views/", "/components/tools/", "/rule/EditRule.vue",
    "/layout/AddThingForm.vue", "/mqtt/MqttConnForm.vue",
    "/node_modules/mqtt/", "/node_modules/json-editor-vue/",
    "/node_modules/vanilla-jsoneditor/",
  ]) {
    assert.ok(!initialModules.some(id => id.includes(deferred)), `initial bundle loads ${deferred}`);
  }
  const list = chunks.find(chunk => Object.keys(chunk.modules).some(id => id.endsWith("/views/List.vue")));
  const listModules = modules(reachable([list]));
  for (const deferred of ["/list/ThingsList.vue", "/node_modules/mqtt/", "/node_modules/vanilla-jsoneditor/"]) {
    assert.ok(!listModules.some(id => id.includes(deferred)), `device page eagerly loads ${deferred}`);
  }
  const rules = chunks.find(chunk => Object.keys(chunk.modules).some(id => id.endsWith("/views/Rule.vue")));
  assert.ok(!modules(reachable([rules])).some(id => id.endsWith("/rule/EditRule.vue")), "rules page eagerly loads editor");
  const bytes = chunks => chunks.reduce((sum, chunk) => sum + Buffer.byteLength(chunk.code), 0);
  console.log(`Entry JS: ${bytes([entry])} bytes; initial JS including imports: ${bytes(initial)} bytes; device list including imports: ${bytes(reachable([entry, list]))} bytes`);
});
