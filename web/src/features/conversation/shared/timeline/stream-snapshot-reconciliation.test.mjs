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

test("message protocol preserves CC rich blocks and contains unknown provider blocks", async () => {
  const {
    parseConversationMessage,
    parseStreamMessage,
  } = await import("@/lib/conversation/message-protocol.ts");

  const message = parseConversationMessage({
    agent_id: "agent-1",
    content: [
      { type: "redacted_thinking", data: "encrypted" },
      { type: "future_provider_block", value: 42 },
    ],
    message_id: "assistant-rich",
    role: "assistant",
    round_id: "round-rich",
    session_key: "agent:agent-1:ws:dm:test",
    timestamp: 1,
  });
  assert.equal(message?.content[0]?.type, "redacted_thinking");
  assert.deepEqual(message?.content[1], {
    type: "unsupported",
    original_type: "future_provider_block",
    payload: { type: "future_provider_block", value: 42 },
  });

  const stream = parseStreamMessage({
    agent_id: "agent-1",
    content_block: {
      type: "tool_use",
      id: "tool-1",
      input: { command: "pwd" },
      name: "Bash",
    },
    index: 0,
    message_id: "assistant-rich",
    parent_tool_use_id: "agent-call-1",
    round_id: "round-rich",
    session_key: "agent:agent-1:ws:dm:test",
    timestamp: 2,
    type: "content_block_start",
  });
  assert.equal(stream?.content_block?.type, "tool_use");
  assert.equal(stream?.parent_tool_use_id, "agent-call-1");

  const blockStop = parseStreamMessage({
    ...stream,
    content_block: undefined,
    index: 0,
    type: "content_block_stop",
  });
  assert.equal(blockStop?.type, "content_block_stop");
  assert.equal(blockStop?.index, 0);

  const roomNotice = parseConversationMessage({
    agent_id: "",
    content: [{ type: "text", text: "请使用 @AgentName 指定要对话的成员" }],
    conversation_id: "conversation-1",
    message_id: "assistant-room-notice",
    role: "assistant",
    room_id: "room-1",
    round_id: "round-room-notice",
    session_key: "room:group:conversation-1",
    timestamp: 3,
  });
  assert.equal(roomNotice?.message_id, "assistant-room-notice");
  assert.equal(parseConversationMessage({
    ...roomNotice,
    room_id: undefined,
    session_key: "agent:agent-1:ws:dm:test",
  }), null, "agentless assistant remains invalid outside a Room");
  assert.equal(parseStreamMessage({
    ...stream,
    agent_id: "",
  }), null, "Room stream events still require an agent identity");
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

test("queued stream patches cannot shorten a newer terminal snapshot", async () => {
  const { applyStreamMessage } = await import("@/hooks/agent/message/stream-message-reducer.ts");
  const terminal = assistantMessage({
    isComplete: true,
    messageId: "assistant-terminal-race",
    status: "done",
    stopReason: "end_turn",
    text: "完整正文abcdef",
    timestamp: 20,
  });
  const terminalMessages = [terminal];

  const afterStalePatch = applyStreamMessage(terminalMessages, {
    agent_id: "agent-1",
    content_block: { type: "text", text: "完整正文abc" },
    index: 0,
    message_id: "assistant-terminal-race",
    round_id: "round-root",
    session_key: "room:group:conversation-1",
    timestamp: 10,
    type: "content_block_delta",
  });

  assert.equal(afterStalePatch[0]?.content[0]?.text, "完整正文abcdef");
  assert.equal(afterStalePatch[0]?.stream_status, "done");
  assert.equal(
    afterStalePatch,
    terminalMessages,
    "the delayed RAF patch must leave the terminal snapshot unchanged",
  );

  for (const status of ["cancelled", "error"]) {
    const stopped = assistantMessage({
      messageId: `assistant-${status}-race`,
      status,
      text: `${status} 完整正文`,
      timestamp: 20,
    });
    const stoppedMessages = [stopped];
    const afterStoppedPatch = applyStreamMessage(stoppedMessages, {
      agent_id: "agent-1",
      content_block: { type: "text", text: `${status} 旧正文` },
      index: 0,
      message_id: `assistant-${status}-race`,
      round_id: "round-root",
      session_key: "room:group:conversation-1",
      timestamp: 10,
      type: "content_block_delta",
    });

    assert.equal(afterStoppedPatch, stoppedMessages);
    assert.equal(afterStoppedPatch[0]?.stream_status, status);
    assert.equal(afterStoppedPatch[0]?.content[0]?.text, `${status} 完整正文`);
  }
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
