import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const storageItems = new Map();
const sessionStorageItems = new Map();
let failSessionStorageRemovals = false;
const originalWindow = globalThis.window;

globalThis.window = {
  addEventListener: () => {},
  dispatchEvent: () => true,
  localStorage: {
    getItem: (key) => storageItems.get(key) ?? null,
    removeItem: (key) => storageItems.delete(key),
    setItem: (key, value) => storageItems.set(key, value),
  },
  sessionStorage: {
    get length() {
      return sessionStorageItems.size;
    },
    getItem: (key) => sessionStorageItems.get(key) ?? null,
    key: (index) => [...sessionStorageItems.keys()][index] ?? null,
    removeItem: (key) => {
      if (failSessionStorageRemovals) {
        throw new Error("remove denied");
      }
      sessionStorageItems.delete(key);
    },
    setItem: (key, value) => sessionStorageItems.set(key, String(value)),
  },
  location: {
    host: "localhost",
    origin: "http://localhost",
    protocol: "http:",
  },
  removeEventListener: () => {},
};
Object.defineProperty(globalThis, "localStorage", {
  configurable: true,
  value: globalThis.window.localStorage,
});

const server = await createServer({
  configFile: false,
  logLevel: "silent",
  resolve: { alias: { "@": path.join(webRoot, "src") } },
  root: webRoot,
  server: { middlewareMode: true },
});

test.after(async () => {
  await server.close();
  storageItems.clear();
  sessionStorageItems.clear();
  failSessionStorageRemovals = false;
  if (originalWindow === undefined) {
    delete globalThis.window;
  } else {
    globalThis.window = originalWindow;
  }
});

test("auth owner scope clears unbound legacy data, preserves one owner, and fences the next", async () => {
  const [
    ownerScope,
    agentModule,
    conversationModule,
    composerDraftModule,
    roomNavigationModule,
    sidebarModule,
    composerHistoryModule,
    workspaceFilesModule,
    workspaceLiveModule,
    ownerGenerationModule,
    roomActivityModule,
    volatileConversationModule,
  ] = await Promise.all([
    server.ssrLoadModule("/src/app/auth/auth-owner-scope.ts"),
    server.ssrLoadModule("/src/store/agent/index.ts"),
    server.ssrLoadModule("/src/store/conversation/index.ts"),
    server.ssrLoadModule(
      "/src/features/conversation/shared/composer/composer-draft-store.ts",
    ),
    server.ssrLoadModule("/src/store/room-navigation.ts"),
    server.ssrLoadModule("/src/store/sidebar.ts"),
    server.ssrLoadModule(
      "/src/features/conversation/shared/composer/composer-history-store.ts",
    ),
    server.ssrLoadModule("/src/store/workspace-files.ts"),
    server.ssrLoadModule("/src/store/workspace-live.ts"),
    server.ssrLoadModule("/src/shared/auth/auth-owner-generation.ts"),
    server.ssrLoadModule("/src/features/home/room-activity-resource.ts"),
    server.ssrLoadModule(
      "/src/hooks/agent/runtime/snapshot/conversation-volatile-storage.ts",
    ),
  ]);
  const { useAgentStore } = agentModule;
  const { useConversationStore } = conversationModule;
  const { useComposerDraftStore } = composerDraftModule;
  const { useRoomNavigationStore } = roomNavigationModule;
  const { useSidebarStore } = sidebarModule;
  const { useComposerHistoryStore } = composerHistoryModule;
  const { useWorkspaceFilesStore } = workspaceFilesModule;
  const { useWorkspaceLiveStore } = workspaceLiveModule;
  const {
    captureAuthOwnerScopeGeneration,
    isAuthOwnerScopeGenerationCurrent,
  } = ownerGenerationModule;
  const stores = {
    useAgentStore,
    useComposerDraftStore,
    useComposerHistoryStore,
    useConversationStore,
    useRoomNavigationStore,
    useSidebarStore,
    useWorkspaceFilesStore,
    useWorkspaceLiveStore,
  };

  seedOwnerState({
    ...stores,
    suffix: "legacy",
  });
  assert.equal(ownerScope.applyAuthOwnerScope(authenticatedStatus("owner-a")), true);
  assertOwnerStateCleared(stores);

  seedOwnerState({
    ...stores,
    suffix: "owner-a",
  });
  assert.equal(ownerScope.applyAuthOwnerScope(authenticatedStatus("owner-a")), false);
  assert.equal(useConversationStore.getState().conversations[0].title, "owner-a");
  assert.equal(
    useRoomNavigationStore.getState().pinned_conversations[0].title,
    "owner-a",
  );
  assert.deepEqual(
    useComposerHistoryStore.getState().items_by_scope["room:shared"],
    ["owner-a"],
  );
  assert.equal(useAgentStore.getState().agents[0].name, "owner-a");
  assert.equal(
    useComposerDraftStore.getState().drafts_by_scope["room:shared"].input,
    "owner-a",
  );
  const previousOwnerDraft = useComposerDraftStore
    .getState()
    .drafts_by_scope["room:shared"];
  const ownerAGeneration = captureAuthOwnerScopeGeneration();
  volatileConversationModule.writeVolatileConversationSnapshot(
    "agent:same-session",
    {
      messages: [{ id: "owner-a", role: "assistant", content: "owner-a-secret" }],
      pending_agent_slots: [],
      updated_at: Date.now(),
    },
    ownerAGeneration,
  );
  assert.equal(sessionStorageItems.size, 1);
  roomActivityModule.updateRoomActivity(
    "room:owner-a",
    "session:owner-a",
    "round:owner-a",
    "running",
  );
  assert.equal(roomActivityModule.getRoomActivity().size, 1);

  assert.equal(ownerScope.applyAuthOwnerScope(authenticatedStatus("owner-b")), true);
  assert.equal(
    isAuthOwnerScopeGenerationCurrent(ownerAGeneration),
    false,
    "mounted owner-a event callbacks must become stale before React cleanup",
  );
  assert.equal(
    volatileConversationModule.readVolatileConversationSnapshot(
      "agent:same-session",
      captureAuthOwnerScopeGeneration(),
    ),
    null,
    "owner B must not restore owner A's same-key volatile message",
  );
  assert.equal(sessionStorageItems.size, 0);
  assert.equal(
    roomActivityModule.getRoomActivity().size,
    0,
    "transient Room activity must not survive an owner boundary",
  );
  assertOwnerStateCleared(stores);
  assert.equal(
    useComposerDraftStore.getState().restore_composer_draft_after_failed_submission(
      "room:shared",
      previousOwnerDraft,
    ),
    false,
    "a late failure callback must not restore the previous owner's draft",
  );
  assert.equal(
    storageItems.get(ownerScope.AUTH_OWNER_SCOPE_STORAGE_KEY),
    "user-id:owner-b",
  );

  const ownerBGeneration = captureAuthOwnerScopeGeneration();
  volatileConversationModule.writeVolatileConversationSnapshot(
    "agent:same-session",
    {
      messages: [{ id: "owner-b", role: "assistant", content: "owner-b-secret" }],
      pending_agent_slots: [],
      updated_at: Date.now(),
    },
    ownerBGeneration,
  );
  failSessionStorageRemovals = true;
  assert.equal(ownerScope.applyAuthOwnerScope(authenticatedStatus("owner-c")), true);
  failSessionStorageRemovals = false;
  const ownerCGeneration = captureAuthOwnerScopeGeneration();
  assert.equal(
    volatileConversationModule.readVolatileConversationSnapshot(
      "agent:same-session",
      ownerCGeneration,
    ),
    null,
    "a failed physical purge must still hide B through the owner namespace",
  );
  volatileConversationModule.writeVolatileConversationSnapshot(
    "agent:same-session",
    {
      messages: [{ id: "owner-c", role: "assistant", content: "owner-c-message" }],
      pending_agent_slots: [],
      updated_at: Date.now(),
    },
    ownerCGeneration,
  );
  volatileConversationModule.removeVolatileConversationSnapshot(
    "agent:same-session",
    ownerBGeneration,
  );
  assert.equal(
    volatileConversationModule.readVolatileConversationSnapshot(
      "agent:same-session",
      ownerCGeneration,
    ).messages[0].content,
    "owner-c-message",
    "late B cleanup must not delete C's same-key snapshot",
  );

  seedOwnerState({
    ...stores,
    suffix: "owner-c-external",
  });
  ownerScope.invalidateLocalAuthOwnerScope();
  assertOwnerStateCleared(stores);
  assert.equal(
    JSON.parse(storageItems.get("nexus-room-navigation")).state.pinned_conversations[0].title,
    "owner-c-external",
    "local invalidation must preserve the shared navigation until authoritative auth binds",
  );
  assert.equal(
    storageItems.get(ownerScope.AUTH_OWNER_SCOPE_STORAGE_KEY),
    "user-id:owner-c",
    "an external-tab fence must not erase the authoritative marker written by that tab",
  );
  assert.equal(ownerScope.applyAuthOwnerScope(authenticatedStatus("owner-c")), true);
  assert.equal(useRoomNavigationStore.getState().pinned_conversations[0].title, "owner-c-external");

  assert.equal(ownerScope.applyAuthOwnerScope(unauthenticatedStatus()), true);
  assert.equal(storageItems.has(ownerScope.AUTH_OWNER_SCOPE_STORAGE_KEY), false);
  assertPersistedKeysUseCompatibleEmptyState();
});

test("auth provider advances the scope before publishing every authoritative status", async () => {
  const source = await readFile(
    path.join(webRoot, "src/app/auth/auth-provider.tsx"),
    "utf8",
  );
  assert.equal(source.match(/applyAuthOwnerScope\(nextStatus\)/g)?.length, 3);
  assert.equal(source.match(/hidePreviousOwnerSurface\(scopeChanged\)/g)?.length, 3);
  assert.match(
    source,
    /applyAuthOwnerScope\(nextStatus\);\s*hidePreviousOwnerSurface\(scopeChanged\);\s*const runtimeOptionsError = await/,
  );
  assert.match(source, /applyAuthOwnerScope\(DEFAULT_UNAUTHORIZED_STATUS\)/);
  assert.match(source, /invalidateLocalAuthOwnerScope\(\)/);
  assert.match(source, /addEventListener\("storage", handleOwnerScopeStorageChange\)/);
});

test("auth status singleflight never reuses or publishes an older owner request", async () => {
  const [bootstrapModule, ownerGenerationModule] = await Promise.all([
    server.ssrLoadModule("/src/app/auth/auth-status-bootstrap.ts"),
    server.ssrLoadModule("/src/shared/auth/auth-owner-generation.ts"),
  ]);
  const first = deferred();
  const second = deferred();
  let firstAccepted = false;
  let secondAcceptCount = 0;

  const firstRun = bootstrapModule.runAuthStatusBootstrap(
    () => first.promise,
    async (status) => {
      firstAccepted = true;
      return status;
    },
  );
  ownerGenerationModule.advanceAuthOwnerScopeGeneration();
  ownerGenerationModule.publishAuthOwnerScopeGeneration();
  const secondRun = bootstrapModule.runAuthStatusBootstrap(
    () => second.promise,
    async (status) => {
      secondAcceptCount += 1;
      return status;
    },
  );
  const coalescedSecondRun = bootstrapModule.runAuthStatusBootstrap(
    () => Promise.reject(new Error("same generation must stay single-flight")),
    async (status) => status,
  );
  assert.strictEqual(coalescedSecondRun, secondRun);

  first.resolve(authenticatedStatus("owner-old"));
  await assert.rejects(firstRun, { name: "AuthOwnerScopeSupersededError" });
  assert.equal(firstAccepted, false);

  second.resolve(authenticatedStatus("owner-current"));
  assert.equal((await secondRun).user_id, "owner-current");
  assert.equal(secondAcceptCount, 1);

  const staleFailure = deferred();
  const staleFailureRun = bootstrapModule.runAuthStatusBootstrap(
    () => staleFailure.promise,
    async (status) => status,
  );
  ownerGenerationModule.advanceAuthOwnerScopeGeneration();
  ownerGenerationModule.publishAuthOwnerScopeGeneration();
  const currentAfterFailure = deferred();
  const currentAfterFailureRun = bootstrapModule.runAuthStatusBootstrap(
    () => currentAfterFailure.promise,
    async (status) => status,
  );
  staleFailure.reject(new Error("old owner request failed"));
  await assert.rejects(staleFailureRun, { name: "AuthOwnerScopeSupersededError" });
  currentAfterFailure.resolve(authenticatedStatus("owner-after-failure"));
  assert.equal(
    (await currentAfterFailureRun).user_id,
    "owner-after-failure",
  );
});

test("Agent and workspace requests fence late results by owner revision", async () => {
  const [agentStore, workspaceFiles] = await Promise.all([
    readFile(path.join(webRoot, "src/store/agent/index.ts"), "utf8"),
    readFile(path.join(webRoot, "src/store/workspace-files.ts"), "utf8"),
  ]);
  assert.match(agentStore, /const ownerRevision = agentOwnerScopeRevision/g);
  assert.match(agentStore, /ownerScopeIsCurrent\(ownerRevision\)/g);
  assert.match(agentStore, /agentOwnerScopeRevision \+= 1/);
  assert.match(workspaceFiles, /workspaceFilesOwnerRevision === ownerRevision/);
  assert.match(workspaceFiles, /workspaceFilesOwnerRevision \+= 1/);
});

test("realtime and authoritative reconciliation callbacks capture the auth owner generation", async () => {
  const [sharedSocket, roomSnapshot, homeDirectory] =
    await Promise.all([
      readFile(
        path.join(
          webRoot,
          "src/lib/websocket/use-socket.ts",
        ),
        "utf8",
      ),
      readFile(
        path.join(
          webRoot,
          "src/pages/room/controller/model/use-room-conversation-snapshot.ts",
        ),
        "utf8",
      ),
      readFile(
        path.join(webRoot, "src/features/home/home-directory-resource.ts"),
        "utf8",
      ),
    ]);

  assert.match(sharedSocket, /const ownerScopeGenerationRef = useRef\(ownerScopeGeneration\)/);
  assert.match(sharedSocket, /isAuthOwnerScopeGenerationCurrent/);
  assert.match(roomSnapshot, /useSyncExternalStore\(\s*subscribeAuthOwnerScopeGeneration/);
  assert.match(roomSnapshot, /isAuthOwnerScopeGenerationCurrent/);
  assert.match(sharedSocket, /if \(!ownsCurrentScope\(\)\) \{\s*return \{ disposition: "dropped" \}/);
  assert.match(sharedSocket, /const ownsChannelScope = \(\) => \(\s*isAuthOwnerScopeGenerationCurrent\(ownerScopeGeneration\)/);
  assert.match(sharedSocket, /onAccepted: \(\) => \{\s*if \(ownsLeaseScope\(\)\)/);
  assert.match(sharedSocket, /useSyncExternalStore\(\s*subscribeAuthOwnerScopeGeneration/);
  assert.match(sharedSocket, /ownerScopeGenerationRef\.current = ownerScopeGeneration/);
  assert.match(homeDirectory, /const ownerScopeGeneration = captureAuthOwnerScopeGeneration\(\)/);
  assert.match(homeDirectory, /isAuthOwnerScopeGenerationCurrent\(ownerScopeGeneration\)/);
});

function authenticatedStatus(userId) {
  return {
    auth_required: true,
    authenticated: true,
    password_login_enabled: true,
    user_id: userId,
    username: userId,
  };
}

function unauthenticatedStatus() {
  return {
    auth_required: true,
    authenticated: false,
    password_login_enabled: true,
    user_id: null,
    username: null,
  };
}

function deferred() {
  let reject;
  let resolve;
  const promise = new Promise((promiseResolve, promiseReject) => {
    reject = promiseReject;
    resolve = promiseResolve;
  });
  return { promise, reject, resolve };
}

function seedOwnerState({
  suffix,
  useAgentStore,
  useComposerDraftStore,
  useComposerHistoryStore,
  useConversationStore,
  useRoomNavigationStore,
  useSidebarStore,
  useWorkspaceFilesStore,
  useWorkspaceLiveStore,
}) {
  useAgentStore.setState({
    agents: [{ agent_id: `agent:${suffix}`, name: suffix }],
    current_agent_id: `agent:${suffix}`,
  });
  useConversationStore.setState({
    conversations: [{
      created_at: 1,
      last_activity_at: 1,
      options: {},
      session_id: null,
      session_key: `session:${suffix}`,
      title: suffix,
    }],
  });
  useRoomNavigationStore.setState({
    conversation_tabs_by_room: {
      "room-shared": {
        active_conversation_id: `conversation:${suffix}`,
        open_conversation_ids: [`conversation:${suffix}`],
      },
    },
    pinned_conversations: [{
      conversation_id: `conversation:${suffix}`,
      room_id: "room-shared",
      session_key: `session:${suffix}`,
      title: suffix,
    }],
  });
  useComposerHistoryStore.setState({
    items_by_scope: { "room:shared": [suffix] },
  });
  useComposerDraftStore.setState({
    drafts_by_scope: {
      "room:shared": {
        attachments: [],
        goalLeadAgentId: null,
        input: suffix,
        inputMode: "message",
        revision: 1,
        selectedTargetIDs: [],
      },
    },
    goal_error_by_scope: { "room:shared": suffix },
  });
  useWorkspaceFilesStore.setState({
    files_by_agent: { [`agent:${suffix}`]: [{ name: suffix, path: suffix }] },
    requested_open_agent_id: `agent:${suffix}`,
  });
  useWorkspaceLiveStore.setState({
    file_states: {
      [`agent:${suffix}:${suffix}`]: {
        agent_id: `agent:${suffix}`,
        path: suffix,
        status: "updated",
        updated_at: 1,
        version: 1,
      },
    },
    recent_events: [{ agent_id: `agent:${suffix}`, id: suffix, path: suffix }],
  });
  useSidebarStore.setState({
    active_panel_item_id: `room:${suffix}`,
    chat_badge_count: 1,
    chat_tab_unseen_counts: { [`room:${suffix}`]: 1 },
    chat_unread_counts: { [`room:${suffix}`]: 1 },
    chat_unread_targets: {
      [`room:${suffix}`]: { key: `room:${suffix}`, room_id: `room:${suffix}` },
    },
    notified_chat_message_ids: [`room:${suffix}\u001fmessage:${suffix}`],
  });
}

function assertOwnerStateCleared({
  useAgentStore,
  useComposerDraftStore,
  useComposerHistoryStore,
  useConversationStore,
  useRoomNavigationStore,
  useSidebarStore,
  useWorkspaceFilesStore,
  useWorkspaceLiveStore,
}) {
  assert.deepEqual(useAgentStore.getState().agents, []);
  assert.equal(useAgentStore.getState().current_agent_id, null);
  assert.deepEqual(useConversationStore.getState().conversations, []);
  assert.deepEqual(useRoomNavigationStore.getState().conversation_tabs_by_room, {});
  assert.deepEqual(useRoomNavigationStore.getState().pinned_conversations, []);
  assert.deepEqual(useComposerHistoryStore.getState().items_by_scope, {});
  assert.deepEqual(useComposerDraftStore.getState().drafts_by_scope, {});
  assert.deepEqual(useComposerDraftStore.getState().goal_error_by_scope, {});
  assert.deepEqual(useWorkspaceFilesStore.getState().files_by_agent, {});
  assert.equal(useWorkspaceFilesStore.getState().requested_open_agent_id, null);
  assert.deepEqual(useWorkspaceLiveStore.getState().file_states, {});
  assert.deepEqual(useWorkspaceLiveStore.getState().recent_events, []);
  assert.equal(useSidebarStore.getState().active_panel_item_id, null);
  assert.deepEqual(useSidebarStore.getState().chat_unread_counts, {});
  assert.deepEqual(useSidebarStore.getState().notified_chat_message_ids, []);
}

function assertPersistedKeysUseCompatibleEmptyState() {
  assert.deepEqual(
    JSON.parse(storageItems.get("agent-ui-conversations")).state.conversations,
    [],
  );
  assert.deepEqual(
    JSON.parse(storageItems.get("nexus-room-navigation")).state,
    { conversation_tabs_by_room: {}, pinned_conversations: [] },
  );
  assert.deepEqual(
    JSON.parse(storageItems.get("nexus-composer-history")).state.items_by_scope,
    {},
  );
  assert.equal(
    JSON.parse(storageItems.get("agent-ui-agents")).state.current_agent_id,
    null,
  );
}
