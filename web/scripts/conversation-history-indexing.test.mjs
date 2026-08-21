import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
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

test("history pages request deferred indexing by default and preserve its retry state", async () => {
  const {
    buildConversationMessagesQuerySuffix,
    normalizeConversationMessagePage,
  } = await server.ssrLoadModule(
    "/src/lib/api/conversation/message-page-model.ts",
  );

  assert.equal(
    buildConversationMessagesQuerySuffix({limit: 3}),
    "?defer_index=true&limit=3",
  );
  assert.equal(
    buildConversationMessagesQuerySuffix({defer_index: false, limit: 3}),
    "?limit=3",
  );
  assert.deepEqual(
    normalizeConversationMessagePage({
      has_more: false,
      indexing: true,
      items: [],
      retry_after_ms: 250,
    }),
    {
      has_more: false,
      indexing: true,
      items: [],
      next_before_round_id: null,
      next_before_round_timestamp: null,
      retry_after_ms: 250,
    },
  );
});

test("indexing responses stay in the request loop instead of becoming empty history", async () => {
  const [historySource, lifecycleSource, roundIndexApi, roundIndexHook] = await Promise.all([
    readFile(
      path.join(webRoot, "src/hooks/agent/session/conversation-history.ts"),
      "utf8",
    ),
    readFile(
      path.join(webRoot, "src/hooks/agent/session/conversation-lifecycle.ts"),
      "utf8",
    ),
    readFile(
      path.join(webRoot, "src/lib/api/conversation/session-api.ts"),
      "utf8",
    ),
    readFile(
      path.join(webRoot, "src/hooks/conversation/use-session-round-index.ts"),
      "utf8",
    ),
  ]);
  assert.match(historySource, /requestConversationHistoryPageUntilReady/);
  assert.match(lifecycleSource, /requestConversationHistoryPageUntilReady/);
  assert.match(roundIndexApi, /if \(!result\.indexing\)/);
  assert.match(roundIndexApi, /await waitForSessionRoundIndex/);
  assert.match(roundIndexHook, /new AbortController\(\)/);
  assert.match(roundIndexHook, /return \(\) => controller\.abort\(\)/);
});

test("initial history waits for deferred indexing instead of committing an empty page", async () => {
  const { requestConversationHistoryPageUntilReady } = await server.ssrLoadModule(
    "/src/hooks/agent/session/conversation-history-request.ts",
  );
  let requestCount = 0;
  const loadedPage = {
    has_more: false,
    indexing: false,
    items: [{ message_id: "message-1" }],
    next_before_round_id: null,
    next_before_round_timestamp: null,
    retry_after_ms: 0,
  };

  const page = await requestConversationHistoryPageUntilReady({
    loadPage: async () => {
      requestCount += 1;
      return requestCount === 1
        ? { ...loadedPage, indexing: true, items: [], retry_after_ms: 1 }
        : loadedPage;
    },
  });

  assert.equal(requestCount, 2);
  assert.deepEqual(page, loadedPage);
});

test("message history keeps complete root rounds inside a bounded browser window", async () => {
  const { boundLoadedMessages } = await server.ssrLoadModule(
    "/src/hooks/agent/message/message-window-model.ts",
  );
  const messages = Array.from({ length: 20 }, (_, index) => ([
    {
      agent_id: "amy",
      content: `user-${index}`,
      message_id: `user-${index}`,
      role: "user",
      round_id: `round-${index}`,
      session_key: "agent:amy:ws:dm:test",
      timestamp: index * 2,
    },
    {
      agent_id: "amy",
      content: [{ type: "text", text: `assistant-${index}` }],
      message_id: `assistant-${index}`,
      role: "assistant",
      round_id: `round-${index}`,
      session_key: "agent:amy:ws:dm:test",
      timestamp: index * 2 + 1,
    },
  ])).flat();

  const latest = boundLoadedMessages(messages, {
    maxBytes: Number.MAX_SAFE_INTEGER,
    maxRounds: 3,
    preference: "latest",
  });
  assert.deepEqual(
    [...new Set(latest.map((message) => message.round_id))],
    ["round-17", "round-18", "round-19"],
  );
  assert.equal(latest.length, 6, "a root round must never be split");

  const around = boundLoadedMessages(messages, {
    anchorRoundIds: ["round-2"],
    maxBytes: Number.MAX_SAFE_INTEGER,
    maxRounds: 9,
    preference: "anchor",
  });
  const retained = new Set(around.map((message) => message.round_id));
  assert.equal(retained.size, 9);
  assert.equal(retained.has("round-2"), true);
  for (let index = 12; index < 20; index += 1) {
    assert.equal(retained.has(`round-${index}`), true);
  }

  const byteBounded = boundLoadedMessages([
    { ...messages[0], content: "a".repeat(2_000), round_id: "large-old" },
    { ...messages[2], content: "b".repeat(2_000), round_id: "large-new" },
  ], {
    maxBytes: 512,
    maxRounds: 10,
    preference: "latest",
  });
  assert.deepEqual(
    [...new Set(byteBounded.map((message) => message.round_id))],
    ["large-new"],
    "one oversized latest round remains atomic without retaining older rounds",
  );
});

test("indexed history reloads evicted rounds and lets an explicit top pull retry", async () => {
  const {
    buildExcludedRoundIds,
    createWindowLoaderRuntime,
    createWindowLoadRequest,
    recordWindowLoadResult,
    refreshWindowLoaderContent,
    shouldRefreshWindowLoaderFromPull,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/window-loader/window-loader-runtime.ts",
  );
  const runtime = createWindowLoaderRuntime();
  const request = createWindowLoadRequest(runtime, 1, "round-evicted");

  recordWindowLoadResult(runtime, request, { status: "loaded" }, 1_000);
  assert.equal(buildExcludedRoundIds(runtime, 1_000).has("round-evicted"), true);

  refreshWindowLoaderContent(runtime);
  assert.equal(
    buildExcludedRoundIds(runtime, 1_000).has("round-evicted"),
    false,
    "a round evicted from the bounded message window must become loadable again",
  );

  for (let attempt = 0; attempt < 3; attempt += 1) {
    recordWindowLoadResult(
      runtime,
      request,
      { status: "missing" },
      2_000 + attempt * 4_000,
    );
  }
  assert.equal(buildExcludedRoundIds(runtime, 20_000).has("round-evicted"), true);
  assert.equal(shouldRefreshWindowLoaderFromPull(0, 36), true);
  assert.equal(shouldRefreshWindowLoaderFromPull(24, 36), false);
  assert.equal(shouldRefreshWindowLoaderFromPull(0, 12), false);

  refreshWindowLoaderContent(runtime);
  assert.equal(
    buildExcludedRoundIds(runtime, 20_000).has("round-evicted"),
    false,
    "a fresh user pull must reopen an exhausted automatic retry",
  );
});

test("indexed history detects equal-size window replacement and keeps pullable geometry", async () => {
  const { buildVisibleRoundRevision } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/window-loader/visible-window-revision.ts",
  );
  const { resolveConversationVirtualPlaceholderHeight } = await server.ssrLoadModule(
    "/src/features/conversation/shared/feed/use-conversation-virtual-scroll-policy.ts",
  );
  const base = {
    feedRoundCount: 80,
    liveRoundCount: 0,
    messageCount: 24,
    pendingAgentSlotCount: 0,
    pendingPermissionCount: 0,
    roomAgentExecutionStateCount: 0,
  };

  assert.notEqual(
    buildVisibleRoundRevision({ ...base, loadedRoundIds: ["round-20", "round-21"] }),
    buildVisibleRoundRevision({ ...base, loadedRoundIds: ["round-18", "round-19"] }),
    "bounded windows with equal counts still carry different resident identities",
  );
  assert.equal(resolveConversationVirtualPlaceholderHeight(true, 180), undefined);
  assert.equal(resolveConversationVirtualPlaceholderHeight(false, 180), 180);
  assert.equal(resolveConversationVirtualPlaceholderHeight(false, undefined), 80);

  const { shouldAdjustConversationVirtualScrollPosition } = await server.ssrLoadModule(
    "/src/features/conversation/shared/feed/use-conversation-virtual-scroll-policy.ts",
  );
  assert.equal(
    shouldAdjustConversationVirtualScrollPosition(
      { end: 100 },
      20,
      { scrollOffset: 200, scrollRect: { height: 600 } },
      {
        bottomScrollActive: false,
        followingLatest: false,
        navigationActive: true,
        userScrollActive: false,
      },
    ),
    false,
    "virtual measurement must not overwrite an explicit round navigation",
  );
});

test("large message details load only on demand and remain abortable", async () => {
  const [toolDetailSource, toolControllerSource, imageBlockSource, sessionApiSource] = await Promise.all([
    readFile(
      path.join(webRoot, "src/features/conversation/shared/message/blocks/tool/tool-block-detail.tsx"),
      "utf8",
    ),
    readFile(
      path.join(webRoot, "src/features/conversation/shared/message/blocks/tool/use-tool-block-controller.ts"),
      "utf8",
    ),
    readFile(
      path.join(webRoot, "src/features/conversation/shared/message/blocks/artifact/image/image-block.tsx"),
      "utf8",
    ),
    readFile(
      path.join(webRoot, "src/lib/api/conversation/session-api.ts"),
      "utf8",
    ),
  ]);
  assert.match(toolDetailSource, /new AbortController\(\)/);
  assert.match(toolDetailSource, /getSessionMessageDetailApi/);
  assert.match(toolDetailSource, /return \(\) => controller\.abort\(\)/);
  assert.match(toolControllerSource, /resolveCompleteToolResult/);
  assert.match(toolControllerSource, /getSessionMessageDetailApi/);
  assert.match(imageBlockSource, /getSessionMessageImageDetailApi/);
  assert.match(imageBlockSource, /URL\.createObjectURL/);
  assert.match(imageBlockSource, /return \(\) => \{/);
  assert.match(sessionApiSource, /applyDesktopRequestHeaders/);
  assert.match(sessionApiSource, /\/sessions\/message-detail/);
});
