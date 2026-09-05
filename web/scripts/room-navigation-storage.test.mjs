// INPUT: Independent navigation modules sharing localStorage and an owner guard.
// OUTPUT: Pins and Room tabs survive stale-page saves, reload and same-owner sync.
// POS: Navigation persistence contract; no backend, browser profile or business writes.

import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const navigationKey = "nexus-room-navigation";
const ownerKey = "nexus-auth-owner-scope";
const items = new Map();
const originalWindow = globalThis.window;
let writes = 0;
let pageIndex = 0;
globalThis.window = { localStorage: {
  getItem: (key) => items.get(key) ?? null,
  setItem: (key, value) => { writes += 1; items.set(key, value); },
  removeItem: (key) => items.delete(key),
} };
const server = await createServer({
  configFile: false, logLevel: "silent", root: webRoot,
  resolve: { alias: { "@": path.join(webRoot, "src") } },
  server: { middlewareMode: true, hmr: false },
});

test.beforeEach(() => { items.clear(); items.set(ownerKey, "user-id:a"); writes = 0; });
test.after(async () => {
  await server.close();
  if (originalWindow === undefined) delete globalThis.window;
  else globalThis.window = originalWindow;
});

async function page(owner = "user-id:a", reset = false) {
  const module = await server.ssrLoadModule(`/src/store/room-navigation.ts?qa_page=${++pageIndex}`);
  if (reset) module.resetRoomNavigationOwnerScope();
  module.setRoomNavigationOwnerScope(owner, () => items.get(ownerKey) === owner);
  return { ...module, state: () => module.useRoomNavigationStore.getState() };
}

const pin = (id) => ({ room_id: `room-${id}`, conversation_id: id, session_key: `session-${id}`, title: id });
const saved = () => JSON.parse(items.get(navigationKey)).state;

test("stale page saves retain pins, other Room tabs and explicit removals after reload", async () => {
  const first = await page();
  const stale = await page();
  first.state().toggle_pinned_conversation(pin("first"));
  stale.state().remember_last_active_conversation("other-room", "other-conversation");
  assert.deepEqual(saved().pinned_conversations, [pin("first")]);
  assert.deepEqual(stale.state().pinned_conversations, [pin("first")]);

  first.state().toggle_pinned_conversation(pin("second"));
  assert.equal(saved().conversation_tabs_by_room["other-room"].active_conversation_id, "other-conversation");
  stale.state().unpin_conversation("room-first", "first");
  first.state().remember_last_active_conversation("new-room", "new-conversation");
  assert.deepEqual(saved().pinned_conversations, [pin("second")], "old memory must not resurrect an unpin");

  const refreshed = await page();
  assert.deepEqual(refreshed.state().pinned_conversations, [pin("second")]);
  assert.deepEqual(Object.keys(refreshed.state().conversation_tabs_by_room).sort(), ["new-room", "other-room"]);
});

test("same-owner storage signals reload current data without an echo write", async () => {
  const first = await page();
  const second = await page();
  first.state().toggle_pinned_conversation(pin("current"));
  const writesBeforeSync = writes;
  second.synchronizeRoomNavigationStorage({ key: navigationKey, newValue: "stale event payload" });
  assert.deepEqual(second.state().pinned_conversations, [pin("current")]);
  assert.equal(writes, writesBeforeSync);

  first.state().unpin_conversation("room-current", "current");
  second.synchronizeRoomNavigationStorage({ key: navigationKey });
  assert.deepEqual(second.state().pinned_conversations, []);
});

test("unchanged navigation commands keep their state reference and cannot trigger a render loop", async (t) => {
  const current = await page();
  current.state().remember_last_active_conversation("room", "conversation");
  const before = current.state();
  let changes = 0;
  const unsubscribe = current.useRoomNavigationStore.subscribe(() => { changes += 1; });
  t.after(unsubscribe);
  current.state().remember_last_active_conversation("room", "conversation");
  current.state().save_room_conversation_tabs("room", ["conversation"], "conversation");
  current.synchronizeRoomNavigationStorage({ key: navigationKey });
  assert.equal(current.state(), before);
  assert.equal(changes, 0);
});

test("a stale owner can neither overwrite nor hydrate the new owner's navigation", async () => {
  const old = await page();
  old.state().toggle_pinned_conversation(pin("old"));
  items.set(ownerKey, "user-id:b");
  const current = await page("user-id:b", true);
  current.state().toggle_pinned_conversation(pin("current"));
  const before = items.get(navigationKey);
  old.state().remember_last_active_conversation("old-room", "late-selection");
  old.synchronizeRoomNavigationStorage({ key: navigationKey });
  assert.equal(items.get(navigationKey), before);
  assert.deepEqual(old.state().pinned_conversations, [pin("old")]);
  assert.deepEqual(current.state().pinned_conversations, [pin("current")]);
});

test("auth binding adopts legacy pins once and refuses a foreign tagged snapshot on reload", async () => {
  items.set(navigationKey, JSON.stringify({ version: 5, state: {
    conversation_tabs_by_room: {}, pinned_conversations: [pin("legacy")],
  } }));
  const migrated = await page();
  assert.deepEqual(migrated.state().pinned_conversations, [pin("legacy")]);
  assert.equal(saved().owner_scope, "user-id:a");
  items.set(ownerKey, "user-id:b");
  const newOwner = await page("user-id:b");
  assert.deepEqual(newOwner.state().pinned_conversations, []);
  assert.equal(saved().owner_scope, "user-id:b");
});

test("late owner invalidation preserves the new snapshot and restores it only after auth binds", async () => {
  const old = await page();
  old.state().toggle_pinned_conversation(pin("old"));
  items.set(ownerKey, "user-id:b");
  const current = await page("user-id:b", true);
  current.state().toggle_pinned_conversation(pin("current"));
  const before = items.get(navigationKey);

  old.resetRoomNavigationOwnerScope();
  assert.deepEqual(old.state().pinned_conversations, []);
  assert.equal(items.get(navigationKey), before, "local invalidation must not erase another page's snapshot");
  old.state().toggle_pinned_conversation(pin("late"));
  assert.equal(items.get(navigationKey), before, "commands remain fenced until auth binds again");
  old.setRoomNavigationOwnerScope("user-id:b", () => items.get(ownerKey) === "user-id:b");
  assert.deepEqual(old.state().pinned_conversations, [pin("current")]);

  old.resetRoomNavigationOwnerScope();
  old.setRoomNavigationOwnerScope(null, () => false);
  assert.deepEqual(saved(), { conversation_tabs_by_room: {}, pinned_conversations: [] }, "confirmed logout clears persistence");
});
