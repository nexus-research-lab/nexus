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

test("authoritative reconciliation replaces stale data and fences an older refresh", async () => {
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
  const reconciled = store.acceptAuthoritativePayload(directoryPayload("reconciled"));
  assert.equal(requests[0].signal.aborted, true);
  assert.equal(reconciled.agents[0].id, "reconciled");

  requests[0].resolve(directoryPayload("stale"));
  await flushPromises();
  assert.equal(store.getSnapshot().agents[0].id, "reconciled");
  assert.equal(store.getSnapshot().hasError, false);
});

test("owner scope reset clears the directory and fences an older response", async () => {
  const requests = [];
  const store = createHomeDirectoryStore({
    load: (signal) => {
      const request = deferred();
      requests.push({ ...request, signal });
      return request.promise;
    },
    reportError: () => {},
  });

  store.acceptAuthoritativePayload(directoryPayload("owner-a"));
  store.refresh();
  store.resetOwnerScope();

  assert.equal(requests[0].signal.aborted, true);
  assert.deepEqual(store.getSnapshot(), {
    agents: [],
    conversations: [],
    hasError: false,
    hasLoaded: false,
    isLoading: true,
    rooms: [],
  });

  requests[0].resolve(directoryPayload("owner-a-late"));
  await flushPromises();
  assert.deepEqual(store.getSnapshot().agents, []);
});

test("directory consumers expose blocking initial failure and non-blocking stale failure", async () => {
  const [
    launcherPage,
    chatSidebar,
    contactsSidebar,
    launcherApi,
    notificationResource,
    refreshNotice,
  ] = await Promise.all([
    readFile(path.join(webRoot, "src/pages/launcher/launcher-page.tsx"), "utf8"),
    readFile(path.join(webRoot, "src/features/home/sidebar/chat-sidebar-panel.tsx"), "utf8"),
    readFile(path.join(webRoot, "src/features/home/sidebar/contacts-sidebar-panel.tsx"), "utf8"),
    readFile(path.join(webRoot, "src/lib/api/launcher-api.ts"), "utf8"),
    readFile(
      path.join(
        webRoot,
        "src/features/home/notifications/use-chat-completion-notifications.ts",
      ),
      "utf8",
    ),
    readFile(
      path.join(webRoot, "src/features/home/home-directory-refresh-error-notice.tsx"),
      "utf8",
    ),
  ]);

  assert.match(launcherPage, /hasError && !hasLoaded/);
  assert.match(launcherPage, /hasError && hasLoaded/);
  assert.match(launcherPage, /HomeDirectoryRefreshErrorNotice/);
  assert.match(chatSidebar, /hasError && controller\.list\.hasLoaded/);
  assert.match(contactsSidebar, /hasError && hasLoaded/);
  assert.match(launcherApi, /signal\?: AbortSignal/);
  assert.match(launcherApi, /signal,/);
  assert.match(notificationResource, /if \(!directory\.hasLoaded\)/);
  assert.doesNotMatch(notificationResource, /directory\.isLoading/);
  assert.match(refreshNotice, /<UiInlineNotice/);
  assert.doesNotMatch(refreshNotice, /<button\b|rounded-\[|font-(?:medium|semibold)/);
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
