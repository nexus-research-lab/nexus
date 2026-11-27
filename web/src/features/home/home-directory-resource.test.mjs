// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

const { createHomeDirectoryStore } = await import("@/features/home/home-directory-store");

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


async function flushPromises() {
  await new Promise((resolve) => setImmediate(resolve));
}

test("live updates survive stale HTTP and deletion cannot resurrect a room", async () => {
  const requests = [];
  const store = createHomeDirectoryStore({ load: () => { const request = deferred(); requests.push(request); return request.promise; }, reportError: assert.fail });
  const payload = directoryPayload("room");
  Object.assign(payload.conversations[0], { room_id: "room", conversation_id: "conv", last_reply_preview: "old" });
  store.acceptAuthoritativePayload(payload);
  store.refresh();
  const timestamp = Date.parse("2026-09-20T00:00:00Z");
  assert.equal(store.applyRoomUpdate({ roomId: "room", conversationId: "conv", timestamp, preview: "new" }), true);
  requests[0].resolve(payload);
  await flushPromises();
  assert.equal(store.getSnapshot().conversations[0].last_reply_preview, "new");
  assert.equal(Date.parse(store.getSnapshot().conversations[0].last_activity), timestamp);
  assert.equal(requests.length, 1);
  store.refresh();
  store.applyRoomUpdate({ roomId: "room", timestamp: timestamp + 1, deleted: true });
  requests[1].resolve(payload);
  await flushPromises();
  store.applyRoomUpdate({ roomId: "room", timestamp: timestamp + 2, preview: "late" });
  assert.equal(store.getSnapshot().rooms.length, 0);
  assert.equal(store.getSnapshot().conversations.length, 0);
  store.resetOwnerScope();
  store.acceptAuthoritativePayload(payload);
  assert.equal(store.getSnapshot().conversations[0].last_reply_preview, "old");
});

test("room preview is shared but source activity and newer reconciliations stay isolated", () => {
  const store = createHomeDirectoryStore({ load: assert.fail, reportError: assert.fail });
  const payload = directoryPayload("room");
  Object.assign(payload.conversations[0], { room_id: "room", conversation_id: "first" });
  payload.conversations.push({ ...payload.conversations[0], conversation_id: "second" });
  store.acceptAuthoritativePayload(payload);
  const timestamp = Date.parse("2026-09-20T00:00:00Z");
  store.applyRoomUpdate({ roomId: "room", conversationId: "first", timestamp, preview: "reply" });
  store.applyRoomUpdate({ roomId: "room", conversationId: "first", timestamp: timestamp - 1, preview: "old replay" });
  assert.deepEqual(store.getSnapshot().conversations.map((item) => item.last_reply_preview), ["reply", "reply"]);
  assert.equal(store.getSnapshot().conversations[1].last_activity, payload.conversations[1].last_activity);
  const oldRevision = store.getRevision();
  store.applyRoomUpdate({ roomId: "room", timestamp: timestamp + 1, preview: "" });
  const fresh = { ...payload, conversations: payload.conversations.map((item) => ({ ...item, last_reply_preview: "authoritative" })) };
  store.acceptAuthoritativePayload(fresh, store.getRevision());
  store.acceptAuthoritativePayload(payload, oldRevision);
  assert.equal(store.getSnapshot().conversations[0].last_reply_preview, "authoritative");
});

test("versioned invalidation skips directories already loaded at or after that revision", async () => {
  const requests = [];
  const store = createHomeDirectoryStore({ load: () => { const request = deferred(); requests.push(request); return request.promise; }, reportError: assert.fail });
  store.refresh();
  // 首屏请求在途时到达的订阅核对排队，响应已覆盖该版本则不再重读。
  store.refresh(5);
  requests[0].resolve({ ...directoryPayload("first"), directory_revision: 5 });
  await flushPromises();
  assert.equal(requests.length, 1);
  store.refresh(5);
  assert.equal(requests.length, 1);
  store.refresh(6);
  assert.equal(requests.length, 2);
  // 在途期间版本化与无版本失效合并后必须无条件重读。
  store.refresh(6);
  store.refresh();
  requests[1].resolve({ ...directoryPayload("second"), directory_revision: 6 });
  await flushPromises();
  assert.equal(requests.length, 3);
  requests[2].resolve(directoryPayload("legacy"));
  await flushPromises();
  // 未标注版本的快照不能用于跳过后续版本化失效。
  store.refresh(1);
  assert.equal(requests.length, 4);
});
