// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

test("history pages request deferred indexing by default and preserve its retry state", async () => {
  const {
    buildConversationMessagesQuerySuffix,
    normalizeConversationMessagePage,
  } = await import("@/lib/api/conversation/message-page-model.ts");

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

test("initial history waits for deferred indexing instead of committing an empty page", async () => {
  const { requestConversationHistoryPageUntilReady } = await import("@/hooks/agent/session/conversation-history-request.ts");
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
  const { boundLoadedMessages } = await import("@/hooks/agent/message/message-window-model.ts");
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
  });
  assert.deepEqual(
    [...new Set(latest.map((message) => message.round_id))],
    ["round-17", "round-18", "round-19"],
  );
  assert.equal(latest.length, 6, "a root round must never be split");

  const byteBounded = boundLoadedMessages([
    { ...messages[0], content: "a".repeat(2_000), round_id: "large-old" },
    { ...messages[2], content: "b".repeat(2_000), round_id: "large-new" },
  ], {
    maxBytes: 512,
    maxRounds: 10,
  });
  assert.deepEqual(
    [...new Set(byteBounded.map((message) => message.round_id))],
    ["large-new"],
    "one oversized latest round remains atomic without retaining older rounds",
  );
});
