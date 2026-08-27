import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import ts from "typescript";

const scriptsRoot = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(scriptsRoot, "..");

const { createHomeDirectoryStore } = await loadDirectoryStore();

test("initial failure stays distinct from an empty successful directory and retry recovers", async () => {
  const requests = [];
  const errors = [];
  const store = createHomeDirectoryStore({
    load: (signal) => {
      const request = deferred();
      requests.push({ ...request, signal });
      return request.promise;
    },
    reportError: (error) => errors.push(error),
  });

  store.refresh();
  assert.deepEqual(pickStatus(store.getSnapshot()), {
    hasError: false,
    hasLoaded: false,
    isLoading: true,
  });

  requests[0].reject(new Error("bootstrap timed out"));
  await flushPromises();
  assert.deepEqual(pickStatus(store.getSnapshot()), {
    hasError: true,
    hasLoaded: false,
    isLoading: false,
  });
  assert.equal(errors.length, 1);

  store.refresh();
  assert.deepEqual(pickStatus(store.getSnapshot()), {
    hasError: false,
    hasLoaded: false,
    isLoading: true,
  });
  requests[1].resolve(directoryPayload("recovered"));
  await flushPromises();
  assert.deepEqual(pickStatus(store.getSnapshot()), {
    hasError: false,
    hasLoaded: true,
    isLoading: false,
  });
  assert.equal(store.getSnapshot().agents[0].id, "recovered");
});

test("failed refresh keeps the last successful directory available", async () => {
  const requests = [];
  const store = createHomeDirectoryStore({
    load: (signal) => {
      const request = deferred();
      requests.push({ ...request, signal });
      return request.promise;
    },
    reportError: () => {},
  });

  store.refresh();
  const firstPayload = directoryPayload("cached");
  requests[0].resolve(firstPayload);
  await flushPromises();
  const successfulSnapshot = store.getSnapshot();

  store.refresh();
  assert.equal(store.getSnapshot().isLoading, false);
  requests[1].reject(new Error("offline"));
  await flushPromises();

  const staleSnapshot = store.getSnapshot();
  assert.deepEqual(pickStatus(staleSnapshot), {
    hasError: true,
    hasLoaded: true,
    isLoading: false,
  });
  assert.strictEqual(staleSnapshot.agents, successfulSnapshot.agents);
  assert.strictEqual(staleSnapshot.conversations, successfulSnapshot.conversations);
  assert.strictEqual(staleSnapshot.rooms, successfulSnapshot.rooms);
});

test("repeated invalidations coalesce into one follow-up without aborting the active request", async () => {
  const requests = [];
  const store = createHomeDirectoryStore({
    load: (signal) => {
      const request = deferred();
      requests.push({ ...request, signal });
      return request.promise;
    },
    reportError: () => {},
  });

  store.refresh();
  store.refresh();
  store.refresh();
  store.refresh();
  assert.equal(requests.length, 1);
  assert.equal(requests[0].signal.aborted, false);

  requests[0].resolve(directoryPayload("first"));
  await flushPromises();
  assert.equal(requests.length, 2);
  assert.equal(requests[1].signal.aborted, false);

  requests[1].resolve(directoryPayload("follow-up"));
  await flushPromises();
  assert.equal(requests.length, 2);
  assert.equal(store.getSnapshot().agents[0].id, "follow-up");
});

test("last subscriber cleanup aborts I/O without surfacing a user error", async () => {
  const requests = [];
  const errors = [];
  const store = createHomeDirectoryStore({
    load: (signal) => {
      const request = deferred();
      requests.push({ ...request, signal });
      return request.promise;
    },
    reportError: (error) => errors.push(error),
  });
  const unsubscribe = store.subscribe(() => {});

  store.refresh();
  store.refresh();
  unsubscribe();
  assert.equal(requests[0].signal.aborted, true);
  requests[0].reject(new DOMException("aborted", "AbortError"));
  await flushPromises();

  assert.equal(store.getSnapshot().hasError, false);
  assert.equal(errors.length, 0);
  assert.equal(requests.length, 1);
});

async function loadDirectoryStore() {
  const sourcePath = path.join(
    webRoot,
    "src/features/home/home-directory-store.ts",
  );
  const source = await readFile(sourcePath, "utf8");
  const transpiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2023,
    },
    fileName: sourcePath,
  }).outputText;
  const encoded = Buffer.from(transpiled).toString("base64");
  return import(`data:text/javascript;base64,${encoded}`);
}

function deferred() {
  let reject;
  let resolve;
  const promise = new Promise((promiseResolve, promiseReject) => {
    resolve = promiseResolve;
    reject = promiseReject;
  });
  return { promise, reject, resolve };
}

function directoryPayload(id) {
  return {
    agents: [{ id, name: id }],
    conversations: [{
      last_activity: "2026-08-17T00:00:00Z",
      room_type: "dm",
      session_key: id,
      title: id,
    }],
    rooms: [{ id, room_type: "dm" }],
  };
}

function pickStatus(snapshot) {
  return {
    hasError: snapshot.hasError,
    hasLoaded: snapshot.hasLoaded,
    isLoading: snapshot.isLoading,
  };
}

async function flushPromises() {
  await new Promise((resolve) => setImmediate(resolve));
}
