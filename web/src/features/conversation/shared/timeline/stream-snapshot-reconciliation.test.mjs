// @vitest-environment node

import { assistantMessage } from "@/features/conversation/testing/message-fixtures.mjs";

import assert from "node:assert/strict";
import { test } from "vitest";

test("迟到历史用 Goal 完成收据推进同一 assistant 快照", async () => {
  const { mergeLoadedMessages } = await import("@/hooks/agent/message/message-collection-model.ts");
  const base = {
    agent_id: "agent-1",
    content: [{ type: "text", text: "最终交付" }],
    is_complete: true,
    message_id: "assistant-receipt-history",
    role: "assistant",
    round_id: "round-receipt-history",
    session_key: "agent:agent-1:ws:dm:receipt-history",
    stop_reason: "end_turn",
    timestamp: 1000,
  };
  const merged = mergeLoadedMessages(
    [{
      ...base,
      goal_completion_receipt: {
        actual_tokens: 42,
        goal_id: "goal-hidden",
        round_id: base.round_id,
      },
    }],
    [base],
  );

  assert.equal(merged.length, 1);
  assert.equal(merged[0].goal_completion_receipt.actual_tokens, 42);
  assert.equal(merged[0].content[0].text, "最终交付");
});

test("stream reducer exposes tool calls and removes terminal empty assistants", async () => {
  const { applyStreamMessage } = await import("@/hooks/agent/message/stream-message-reducer.ts");
  const base = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-room",
    message_id: "assistant-tool-stream",
    parent_tool_use_id: "agent-call-1",
    room_id: "room-1",
    round_id: "round-tool-stream",
    session_key: "agent:agent-1:ws:dm:test",
    timestamp: 1,
  };
  let messages = applyStreamMessage([], {
    ...base,
    message: { model: "glm-5.2" },
    type: "message_start",
  });
  assert.equal(messages[0]?.parent_id, "agent-call-1");
  assert.equal(
    messages[0]?.agent_round_id,
    "agent-round-room",
    "Room stream placeholder must keep the slot execution identity",
  );
  assert.equal(messages[0]?.room_id, "room-1");
  messages = applyStreamMessage(messages, {
    ...base,
    content_block: {
      type: "tool_use",
      id: "tool-1",
      input: { command: "pwd" },
      name: "Bash",
    },
    index: 0,
    type: "content_block_start",
  });
  assert.equal(messages[0]?.content[0]?.type, "tool_use");
  messages = applyStreamMessage(messages, {
    ...base,
    index: 0,
    type: "content_block_stop",
  });
  assert.equal(messages[0]?.content[0]?.type, "tool_use");

  let emptyMessages = applyStreamMessage([], {
    ...base,
    message_id: "assistant-empty",
    type: "message_start",
  });
  emptyMessages = applyStreamMessage(emptyMessages, {
    ...base,
    message_id: "assistant-empty",
    type: "message_stop",
  });
  assert.deepEqual(emptyMessages, []);
});

test("RAF stream batches stay isolated to the latest exact session", async () => {
  const { applyStreamPayloadBatchForActiveSession } =
    await import("@/hooks/agent/transport/use-conversation-stream-buffer.ts");
  const payload = (sessionKey, messageId, timestamp) => ({
    agent_id: "agent-1",
    message_id: messageId,
    round_id: "round-stream-buffer",
    session_key: sessionKey,
    timestamp,
    type: "message_start",
  });
  const currentSession = "room:group:conversation-current";
  const messages = applyStreamPayloadBatchForActiveSession(
    [],
    [
      payload(currentSession, "assistant-current-1", 1),
      payload("room:group:conversation-old", "assistant-old", 2),
      payload(currentSession, "assistant-current-2", 3),
    ],
    currentSession,
    currentSession,
  );
  assert.deepEqual(
    messages.map((message) => message.message_id),
    ["assistant-current-1", "assistant-current-2"],
    "the current session keeps batch arrival order while old payloads are discarded",
  );
  assert.equal(
    applyStreamPayloadBatchForActiveSession(
      messages,
      [payload(currentSession, "assistant-stale-after-switch", 4)],
      currentSession,
      "room:group:conversation-newer",
    ),
    messages,
    "a second session switch before React commits must reject the whole captured batch",
  );
  assert.equal(
    applyStreamPayloadBatchForActiveSession(
      messages,
      [payload("agent:default:workspace:group:c1", "assistant-alias", 5)],
      "room:group:c1",
      "room:group:c1",
    ),
    messages,
    "stream isolation uses exact protocol session keys instead of cross-scope aliases",
  );
});

test("late history cannot roll an assistant snapshot backward", async () => {
  const { mergeLoadedMessages, upsertMessage } = await import("@/hooks/agent/message/message-collection-model.ts");

  const liveDone = upsertMessage(
    [assistantMessage({ text: "完整的模型", timestamp: 10 })],
    assistantMessage({
      isComplete: true,
      status: "done",
      stopReason: "end_turn",
      text: "完整的模型回复",
      timestamp: 20,
    }),
  );
  const afterStaleHistory = mergeLoadedMessages(
    [assistantMessage({
      isComplete: true,
      status: "done",
      stopReason: "end_turn",
      text: "完整的模型",
      timestamp: 99,
    })],
    liveDone,
  );
  assert.equal(afterStaleHistory[0]?.stream_status, "done");
  assert.equal(afterStaleHistory[0]?.content[0]?.text, "完整的模型回复");
  assert.equal(afterStaleHistory[0]?.timestamp, 20);

  const canonicalResult = {
    duration_api_ms: 20,
    duration_ms: 30,
    is_error: false,
    message_id: "assistant-root",
    num_turns: 2,
    result: "完整的模型回复，附上最终依据",
    subtype: "success",
    timestamp: 30,
  };
  const afterCanonicalHistory = mergeLoadedMessages(
    [assistantMessage({
      isComplete: true,
      resultSummary: canonicalResult,
      status: "done",
      stopReason: "end_turn",
      text: "完整的模型回复，附上最终依据",
      timestamp: 30,
    })],
    afterStaleHistory,
  );
  assert.equal(
    afterCanonicalHistory[0]?.content[0]?.text,
    "完整的模型回复，附上最终依据",
  );
  assert.equal(afterCanonicalHistory[0]?.result_summary?.timestamp, 30);
  assert.equal(afterCanonicalHistory[0]?.timestamp, 30);
});
