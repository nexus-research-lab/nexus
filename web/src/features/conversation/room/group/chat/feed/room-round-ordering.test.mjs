// @vitest-environment node

import { userMessage, assistantMessage } from "@/features/conversation/testing/message-fixtures.mjs";

import assert from "node:assert/strict";
import { test } from "vitest";

test("Room permission-first children append after an existing reply and never move", async () => {
  const { buildRoomAgentRoundEntries } = await import("@/features/conversation/room/group/round/round-agent-model.ts");
  const {
    acknowledgeRoomAgentExecutionPermission,
    syncRoomAgentExecutionFromStream,
    syncRoomAgentExecutionsFromMessages,
    syncRoomAgentExecutionsFromPermissions,
    syncRoomAgentExecutionsFromSlots,
  } = await import("@/hooks/agent/runtime/model/room-agent-execution-state.ts");
  const roundId = "round-parent-before-permissions";
  const parent = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "agent-round-1",
    displayOrder: 10_000,
    isComplete: true,
    messageId: "assistant-agent-1",
    roundId,
    status: "done",
    stopReason: "end_turn",
    text: "@Agent2 调研 M1/M2，@Agent3 调研 M3/M4",
    timestamp: 10,
  });
  const permissions = [
    {
      agent_id: "agent-2",
      agent_round_id: "agent-round-2",
      request_id: "permission-agent-2",
      round_id: roundId,
      tool_input: { query: "M1 M2" },
      tool_name: "WebSearch",
    },
    {
      agent_id: "agent-3",
      agent_round_id: "agent-round-3",
      request_id: "permission-agent-3",
      round_id: roundId,
      tool_input: { query: "M3 M4" },
      tool_name: "WebSearch",
    },
  ];
  const entryOrder = (messages, slots, pendingPermissions, states) => (
    buildRoomAgentRoundEntries(
      messages,
      slots,
      pendingPermissions,
      states,
    ).map((entry) => entry.agent_round_id)
  );

  // Agent Session permission events may beat the shared pending-slot snapshot.
  // The already visible parent reply still owns the first canonical position.
  const permissionFirst = syncRoomAgentExecutionsFromPermissions(
    [],
    permissions,
    20,
  );
  const expectedOrder = [
    "agent-round-1",
    "agent-round-2",
    "agent-round-3",
  ];
  assert.deepEqual(
    entryOrder([parent], [], permissions, permissionFirst),
    expectedOrder,
    "permission-only children must append after the existing parent reply",
  );

  const acknowledged = permissions.reduce(
    (states, permission) => acknowledgeRoomAgentExecutionPermission(
      states,
      permission,
      21,
    ),
    permissionFirst,
  );
  const afterPermissionRemoval = syncRoomAgentExecutionsFromPermissions(
    acknowledged,
    [],
    22,
  );
  assert.deepEqual(
    entryOrder([parent], [], [], afterPermissionRemoval),
    expectedOrder,
    "acknowledging the last permission must retain the same execution shells",
  );

  const reverseSlots = [
    {
      agent_id: "agent-3",
      agent_round_id: "agent-round-3",
      index: 1,
      msg_id: "slot-agent-3",
      round_id: roundId,
      status: "streaming",
      timestamp: 30,
    },
    {
      agent_id: "agent-2",
      agent_round_id: "agent-round-2",
      index: 0,
      msg_id: "slot-agent-2",
      round_id: roundId,
      status: "streaming",
      timestamp: 30,
    },
  ];
  const active = syncRoomAgentExecutionsFromSlots(
    afterPermissionRemoval,
    reverseSlots,
  );
  assert.deepEqual(
    entryOrder([parent], reverseSlots, [], active),
    expectedOrder,
    "reverse slot arrival must enrich rather than reorder permission-first nodes",
  );

  const agent2Stream = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "agent-round-2",
    messageId: "assistant-agent-2",
    roundId,
    status: "streaming",
    text: "Agent2 正在回复",
    timestamp: 31,
  });
  const agent3Done = assistantMessage({
    agentId: "agent-3",
    agentRoundId: "agent-round-3",
    isComplete: true,
    messageId: "assistant-agent-3",
    roundId,
    status: "done",
    stopReason: "end_turn",
    text: "Agent3 已回复",
    timestamp: 32,
  });
  const afterStream = syncRoomAgentExecutionFromStream(active, {
    agent_id: "agent-2",
    agent_round_id: "agent-round-2",
    message_id: "assistant-agent-2",
    round_id: roundId,
    session_key: "room:group:conversation-1",
    timestamp: 31,
    type: "message_start",
  });
  const withMessages = syncRoomAgentExecutionsFromMessages(
    afterStream,
    [agent3Done, agent2Stream],
  );
  assert.deepEqual(
    entryOrder(
      [parent, agent3Done, agent2Stream],
      reverseSlots,
      [],
      withMessages,
    ),
    expectedOrder,
    "stream and terminal message evidence must keep the first visible order",
  );
});

test("Room durable snapshot backfills an earlier Lead after a live child", async () => {
  const { buildRoomAgentRoundEntries } = await import("@/features/conversation/room/group/round/round-agent-model.ts");
  const {
    syncRoomAgentExecutionFromStream,
    syncRoomAgentExecutionsFromMessages,
  } = await import("@/hooks/agent/runtime/model/room-agent-execution-state.ts");
  const roundId = "round-live-child-before-history";
  const lead = assistantMessage({
    agentId: "agent-lead",
    agentRoundId: "agent-round-lead",
    displayOrder: 10_000,
    isComplete: true,
    messageId: "assistant-history-lead",
    roundId,
    status: "done",
    stopReason: "end_turn",
    text: "我先完成分工，Researcher 继续执行。",
    timestamp: 30,
  });
  const researcher = assistantMessage({
    agentId: "agent-researcher",
    agentRoundId: "agent-round-researcher",
    displayOrder: 20_000,
    messageId: "assistant-live-researcher",
    roundId,
    status: "streaming",
    text: "Researcher 正在调研",
    timestamp: 31,
  });
  const streamFirst = syncRoomAgentExecutionFromStream([], {
    agent_id: "agent-researcher",
    agent_round_id: "agent-round-researcher",
    message_id: "assistant-live-researcher",
    round_id: roundId,
    session_key: "room:group:conversation-live-before-history",
    timestamp: 21,
    type: "message_start",
  });
  const reconciled = syncRoomAgentExecutionsFromMessages(
    streamFirst,
    [lead, researcher],
  );
  const statesByAgent = new Map(
    reconciled.map((state) => [state.agent_id, state]),
  );

  assert.deepEqual(
    buildRoomAgentRoundEntries(
      [lead, researcher],
      [],
      [],
      reconciled,
    ).map((entry) => entry.agent_id),
    ["agent-lead", "agent-researcher"],
    "a live child observed during history loading must not stay above its earlier durable Lead",
  );
  assert.equal(statesByAgent.get("agent-lead")?.display_order, 10_000);
  assert.equal(statesByAgent.get("agent-researcher")?.display_order, 20_000);
  assert.equal(
    statesByAgent.get("agent-researcher")?.first_seen_at,
    21,
    "canonical order reconciliation must preserve the original live first-seen timestamp",
  );
});

test("Room timeline conserves user messages while optimistic roots become canonical", async () => {
  const { projectGroupAgentTimeline } = await import("@/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts");
  const clientMessageId = "client-room-conservation";
  const optimistic = {
    ...userMessage({
      content: "先分析这件事",
      messageId: clientMessageId,
      roundId: "optimistic-room-round",
      timestamp: 1,
    }),
    client_message_id: clientMessageId,
  };
  const canonical = {
    ...optimistic,
    content: "先分析这件事（已确认）",
    message_id: "canonical-room-message",
    round_id: "canonical-room-round",
    timestamp: 2,
  };
  const followUp = userMessage({
    content: "再补充可靠性维度",
    messageId: "room-follow-up",
    roundId: "canonical-room-round",
    timestamp: 3,
  });
  const projection = projectGroupAgentTimeline({
    messageGroups: new Map([
      ["optimistic-room-round", [optimistic]],
      ["canonical-room-round", [canonical, followUp]],
    ]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map(),
    roundIds: ["optimistic-room-round", "canonical-room-round"],
  });

  assert.deepEqual(projection.roundIds, [clientMessageId]);
  assert.equal(
    projection.rootRoundIds.get(clientMessageId),
    "canonical-room-round",
  );
  assert.deepEqual(
    projection.messageGroups.get(clientMessageId)?.map(
      (message) => message.message_id,
    ),
    ["canonical-room-message", "room-follow-up"],
    "canonical ACK replacement must not overwrite another visible user message",
  );
});

test("Room guidance stays on its exact consumed agent round", async () => {
  const { buildGroupRoundCardModel } = await import("@/features/conversation/room/group/thread/round-card/group-round-card-model.ts");
  const guide = userMessage({
    agentRoundId: "agent-1-old-round",
    content: "这是旧执行轮实际消费的插话",
    deliveryPolicy: "guide",
    messageId: "user-guide-exact-round",
    roundId: "round-root",
    sourceRoundId: "round-guide-exact",
    targetAgentIds: ["agent-1"],
    timestamp: 11,
  });
  const oldResult = assistantMessage({
    agentRoundId: "agent-1-old-round",
    isComplete: true,
    messageId: "assistant-agent-1-old",
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      is_error: false,
      num_turns: 1,
      result: "旧轮按插话完成",
      subtype: "success",
      timestamp: 12,
    },
    status: "done",
    stopReason: "end_turn",
    text: "旧轮按插话完成",
    timestamp: 12,
  });
  const newStream = assistantMessage({
    agentRoundId: "agent-1-new-round",
    messageId: "assistant-agent-1-new",
    text: "新轮正在处理",
    timestamp: 13,
  });
  const model = buildGroupRoundCardModel({
    messages: [guide, oldResult, newStream],
    pendingPermissions: [],
    pendingSlots: [{
      agent_id: "agent-1",
      agent_round_id: "agent-1-new-round",
      msg_id: "slot-agent-1-new",
      round_id: "round-root",
      status: "streaming",
      timestamp: 13,
    }],
  });

  assert.deepEqual(
    model.entries.map((entry) => ({
      agentRoundId: entry.agent_round_id,
      guides: entry.guidedUserMessages.map(({ message }) => message.message_id),
    })),
    [
      {
        agentRoundId: "agent-1-old-round",
        guides: ["user-guide-exact-round"],
      },
      { agentRoundId: "agent-1-new-round", guides: [] },
    ],
  );
});

