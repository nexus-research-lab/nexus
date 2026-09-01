import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({
  configFile: false,
  logLevel: "silent",
  resolve: { alias: { "@": path.join(webRoot, "src") } },
  root: webRoot,
  server: { middlewareMode: true },
});

test.after(async () => {
  await server.close();
});

test("old owner Provider availability cannot publish or clear the new owner request", async () => {
  const { ProviderAvailabilityResource } = await server.ssrLoadModule(
    "/src/hooks/capability/provider-availability-resource.ts",
  );
  const requests = [];
  const events = [];
  let currentGeneration = 1;
  const resource = new ProviderAvailabilityResource({
    isGenerationCurrent: (generation) => generation === currentGeneration,
    load: () => {
      const request = deferred();
      requests.push(request);
      return request.promise;
    },
    reportError: (error) => {
      throw error;
    },
  });
  resource.subscribe((event) => events.push(event));

  const ownerARequest = resource.fetch(1, "nxs");
  assert.strictEqual(resource.fetch(1, "nxs"), ownerARequest);
  assert.equal(requests.length, 1, "same owner/runtime must stay single-flight");

  currentGeneration = 2;
  const ownerBRequest = resource.fetch(2, "nxs");
  assert.equal(requests.length, 2, "a new owner must never reuse the old promise");

  requests[0].resolve(true);
  await ownerARequest;
  assert.deepEqual(events, []);
  assert.equal(resource.read(2, "nxs"), undefined);
  assert.strictEqual(
    resource.fetch(2, "nxs"),
    ownerBRequest,
    "the old promise finally must not delete the new same-runtime request",
  );
  await resource.invalidate(1, "nxs");
  assert.strictEqual(
    resource.fetch(2, "nxs"),
    ownerBRequest,
    "a late old hook must not roll the resource generation back or clear B",
  );
  assert.equal(requests.length, 2);

  requests[1].resolve(false);
  await ownerBRequest;
  assert.deepEqual(events, [{
    generation: 2,
    runtimeKind: "nxs",
    value: false,
  }]);
  assert.equal(resource.read(2, "nxs"), false);
});

function deferred() {
  let reject;
  let resolve;
  const promise = new Promise((promiseResolve, promiseReject) => {
    reject = promiseReject;
    resolve = promiseResolve;
  });
  return { promise, reject, resolve };
}
