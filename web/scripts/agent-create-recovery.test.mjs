import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { importLeafTypeScriptModule } from "./import-leaf-typescript-module.mjs";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

test("Agent creation storage keeps only request identity and never blocks creation", async () => {
  const values = new Map();
  const localStorage = {
    getItem: (key) => values.get(key) ?? null,
    removeItem: (key) => values.delete(key),
    setItem: (key, value) => values.set(key, value),
  };
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: { localStorage },
  });
  const intent = await importLeafTypeScriptModule(
    webRoot,
    "src/store/agent/agent-creation-intent.ts",
  );

  intent.saveAgentCreationRequestId("owner:a", "web-create:one");
  assert.equal(intent.readAgentCreationRequestId("owner:a"), "web-create:one");
  assert.equal(intent.readAgentCreationRequestId("owner:b"), null);
  const persisted = JSON.parse([...values.values()][0]);
  assert.deepEqual(Object.keys(persisted).sort(), ["requestId", "version"]);
  assert.equal("name" in persisted, false);
  assert.equal("digest" in persisted, false);
  assert.equal("body" in persisted, false);
  assert.equal("transportRequestId" in persisted, false);
  localStorage.setItem = () => {
    throw new Error("lock service unavailable");
  };
  assert.doesNotThrow(() => {
    intent.saveAgentCreationRequestId("owner:c", "web-create:not-persisted");
  });
});
