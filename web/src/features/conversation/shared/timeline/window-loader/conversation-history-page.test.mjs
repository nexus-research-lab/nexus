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
