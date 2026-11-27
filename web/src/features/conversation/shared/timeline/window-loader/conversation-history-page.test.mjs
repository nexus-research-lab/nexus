// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

test("history page requests carry only cursor fields and normalize to the complete page contract", async () => {
  const {
    buildConversationMessagesQuerySuffix,
    normalizeConversationMessagePage,
  } = await import("@/lib/api/conversation/message-page-model.ts");

  assert.equal(buildConversationMessagesQuerySuffix({limit: 3}), "?limit=3");
  assert.equal(
    buildConversationMessagesQuerySuffix({
      before_round_id: "round-2",
      before_round_timestamp: 200,
      limit: 3,
    }),
    "?limit=3&before_round_id=round-2&before_round_timestamp=200",
  );
  assert.deepEqual(
    normalizeConversationMessagePage({
      has_more: false,
      items: [],
    }),
    {
      has_more: false,
      items: [],
      next_before_round_id: null,
      next_before_round_timestamp: null,
    },
  );
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
