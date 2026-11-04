// @vitest-environment node

import { userMessage, assistantMessage } from "@/features/conversation/testing/message-fixtures.mjs";

import assert from "node:assert/strict";
import { test } from "vitest";

test("Room keeps separate agent_round entries for the same agent", async () => {
  const { buildRoomAgentRoundEntries } = await import("@/features/conversation/room/group/round/round-agent-model.ts");
  const { projectGroupAgentTimeline } = await import("@/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts");
  const oldResult = assistantMessage({
    agentRoundId: "agent-round-old",
    isComplete: true,
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      is_error: false,
      num_turns: 1,
      result: "旧回复",
      subtype: "success",
      timestamp: 10,
    },
    status: "done",
    stopReason: "end_turn",
    text: "旧回复",
    timestamp: 10,
  });
  const activeSlot = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-new",
    msg_id: "slot-new",
    round_id: "round-root",
    status: "streaming",
    timestamp: 20,
  };

  let entries = buildRoomAgentRoundEntries([oldResult], [activeSlot]);
  assert.equal(entries.length, 2);
  assert.deepEqual(
    entries.map(({ agent_round_id, status }) => ({ agent_round_id, status })),
    [
      { agent_round_id: "agent-round-old", status: "done" },
      { agent_round_id: "agent-round-new", status: "streaming" },
    ],
  );
  assert.deepEqual(entries[1]?.assistant_messages, []);

  const currentStream = assistantMessage({
    agentRoundId: "agent-round-new",
    messageId: "assistant-new",
    status: "streaming",
    text: "正在处理新问题",
    timestamp: 21,
  });
  entries = buildRoomAgentRoundEntries(
    [oldResult, currentStream],
    [activeSlot],
  );
  assert.equal(entries[1]?.status, "streaming");
  assert.deepEqual(
    entries[1]?.assistant_messages.map((message) => message.message_id),
    ["assistant-new"],
  );

  const legacyStream = assistantMessage({
    messageId: "assistant-legacy-new",
    status: "streaming",
    text: "兼容旧协议流",
    timestamp: 22,
  });
  entries = buildRoomAgentRoundEntries(
    [
      { ...oldResult, agent_round_id: undefined },
      legacyStream,
    ],
    [activeSlot],
  );
  assert.equal(entries[1]?.status, "streaming");
  assert.equal(entries[1]?.result_summary, undefined);
  assert.deepEqual(
    entries[1]?.assistant_messages.map((message) => message.message_id),
    ["assistant-legacy-new"],
  );

  const interruptedToolTurn = assistantMessage({
    agentRoundId: "agent-round-interrupted",
    isComplete: true,
    messageId: "assistant-interrupted-tool-turn",
    roundId: "round-root",
    status: "done",
    stopReason: "tool_use",
    text: "旧执行仍在调用工具",
    timestamp: 30,
  });
  const completedRestart = assistantMessage({
    agentRoundId: "agent-round-restarted",
    isComplete: true,
    messageId: "assistant-restarted-result",
    roundId: "round-root",
    status: "done",
    stopReason: "end_turn",
    text: "重启后的执行已完成",
    timestamp: 40,
  });
  entries = buildRoomAgentRoundEntries([
    interruptedToolTurn,
    completedRestart,
  ]);
  assert.deepEqual(
    entries.map(({ agent_round_id, status }) => ({ agent_round_id, status })),
    [
      { agent_round_id: "agent-round-interrupted", status: "cancelled" },
      { agent_round_id: "agent-round-restarted", status: "done" },
    ],
    "a historical tool turn superseded by a later execution is stopped",
  );
  const projection = projectGroupAgentTimeline({
    messageGroups: new Map([["round-root", [
      interruptedToolTurn,
      completedRestart,
    ]]]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map(),
    roundIds: ["round-root"],
  });
  const interruptedNodeId = projection.roundIds.find((nodeId) => (
    projection.messageGroups.get(nodeId)?.some(
      (message) => message.message_id === interruptedToolTurn.message_id,
    )
  ));
  assert.ok(interruptedNodeId);
  const interruptedNodeStates =
    projection.roomAgentExecutionStateGroups.get(interruptedNodeId) ?? [];
  assert.equal(interruptedNodeStates[0]?.status, "cancelled");
});

test("Room Agent slot order survives live, terminal, and history projections", async () => {
  const { buildRoomAgentRoundEntries } = await import("@/features/conversation/room/group/round/round-agent-model.ts");
  const firstDone = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "agent-round-1",
    displayOrder: 1_000,
    isComplete: true,
    messageId: "assistant-agent-1-done",
    status: "done",
    stopReason: "end_turn",
    text: "Agent1 已完成",
    timestamp: 20,
  });
  const secondStream = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "agent-round-2",
    messageId: "assistant-agent-2-stream",
    text: "Agent2 正在处理",
    timestamp: 21,
  });
  const liveSlots = [
    {
      agent_id: "agent-2",
      agent_round_id: "agent-round-2",
      index: 0,
      msg_id: "slot-agent-2",
      round_id: "round-root",
      status: "streaming",
      timestamp: 2,
    },
    {
      agent_id: "agent-3",
      agent_round_id: "agent-round-3",
      index: 1,
      msg_id: "slot-agent-3",
      round_id: "round-root",
      status: "pending",
      timestamp: 2,
    },
  ];

  const mixed = buildRoomAgentRoundEntries(
    [secondStream, firstDone],
    liveSlots,
  );
  assert.deepEqual(
    mixed.map(({ agent_id, display_order, status }) => ({
      agent_id,
      display_order,
      status,
    })),
    [
      { agent_id: "agent-1", display_order: 1_000, status: "done" },
      { agent_id: "agent-2", display_order: 2_000, status: "streaming" },
      { agent_id: "agent-3", display_order: 2_001, status: "pending" },
    ],
    "a new live member must append after a terminal sibling instead of jumping above it",
  );

  const secondDone = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "agent-round-2",
    displayOrder: 2_000,
    isComplete: true,
    messageId: "assistant-agent-2-done",
    status: "done",
    stopReason: "end_turn",
    text: "Agent2 已完成",
    timestamp: 30,
  });
  const terminal = buildRoomAgentRoundEntries([secondDone, firstDone]);
  assert.deepEqual(
    terminal.map(({ agent_id, display_order }) => ({
      agent_id,
      display_order,
    })),
    [
      { agent_id: "agent-1", display_order: 1_000 },
      { agent_id: "agent-2", display_order: 2_000 },
    ],
    "pending -> terminal must retain the same canonical slot positions",
  );

  const firstFinishedLater = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "history-agent-round-1",
    displayOrder: 1_000,
    isComplete: true,
    messageId: "history-assistant-agent-1",
    status: "done",
    stopReason: "end_turn",
    text: "Agent1 后完成",
    timestamp: 40,
  });
  const secondFinishedEarlier = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "history-agent-round-2",
    displayOrder: 2_001,
    isComplete: true,
    messageId: "history-assistant-agent-2",
    status: "done",
    stopReason: "end_turn",
    text: "Agent2 先完成",
    timestamp: 30,
  });
  const history = buildRoomAgentRoundEntries([
    secondFinishedEarlier,
    firstFinishedLater,
  ]);
  assert.deepEqual(
    history.map(({ agent_id, display_order }) => ({
      agent_id,
      display_order,
    })),
    [
      { agent_id: "agent-1", display_order: 1_000 },
      { agent_id: "agent-2", display_order: 2_001 },
    ],
    "history reload must restore slot order instead of completion order",
  );
});

test("Room canonical assistant replaces its temporary synthetic result", async () => {
  const { buildRoomAgentRoundEntries } = await import("@/features/conversation/room/group/round/round-agent-model.ts");
  const canonical = assistantMessage({
    agentRoundId: "agent-round-1",
    messageId: "assistant-canonical",
    model: "canonical-model",
    status: "streaming",
    text: "已完成过程处理",
    timestamp: 10,
  });
  const synthetic = assistantMessage({
    agentRoundId: "agent-round-1",
    isComplete: true,
    messageId: "assistant_result-1",
    resultSummary: {
      duration_api_ms: 20,
      duration_ms: 30,
      is_error: false,
      message_id: "result-1",
      num_turns: 2,
      subtype: "success",
      timestamp: 30,
    },
    status: "done",
    stopReason: "end_turn",
    text: "最终模型回复",
    timestamp: 30,
  });

  const entries = buildRoomAgentRoundEntries([canonical, synthetic]);
  assert.equal(entries.length, 1);
  assert.equal(entries[0]?.status, "done");
  assert.equal(entries[0]?.timestamp, 30);
  assert.deepEqual(
    entries[0]?.assistant_messages.map((message) => message.message_id),
    ["assistant-canonical"],
  );
  assert.equal(
    entries[0]?.assistant_messages[0]?.result_summary?.result,
    "最终模型回复",
  );
  assert.equal(entries[0]?.assistant_messages[0]?.model, "canonical-model");
});

test("Room Agent replies keep their first display order through completion", async () => {
  const { buildGroupRoundCardModel } = await import("@/features/conversation/room/group/thread/round-card/group-round-card-model.ts");
  const rootUser = userMessage({
    content: "一起分析",
    messageId: "user-root-display-order",
    roundId: "round-root",
    timestamp: 1,
  });
  const agent1Partial = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "agent-1-round",
    messageId: "assistant-agent-1-partial",
    text: "Agent1 正在处理",
    timestamp: 2,
  });
  const agent2Done = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "agent-2-round",
    isComplete: true,
    messageId: "assistant-agent-2-done",
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      is_error: false,
      num_turns: 1,
      result: "Agent2 完成",
      subtype: "success",
      timestamp: 4,
    },
    status: "done",
    stopReason: "end_turn",
    text: "Agent2 完成",
    timestamp: 4,
  });
  const guide = userMessage({
    content: "Agent1 再补充结论",
    deliveryPolicy: "guide",
    messageId: "user-guide-display-order",
    roundId: "round-root",
    sourceRoundId: "round-guide-display-order",
    targetAgentIds: ["agent-1"],
    timestamp: 5,
  });
  const agent1Done = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "agent-1-round",
    isComplete: true,
    messageId: "assistant-agent-1-done",
    resultSummary: {
      duration_api_ms: 20,
      duration_ms: 30,
      is_error: false,
      num_turns: 2,
      result: "Agent1 补充完成",
      subtype: "success",
      timestamp: 6,
    },
    status: "done",
    stopReason: "end_turn",
    text: "Agent1 补充完成",
    timestamp: 6,
  });
  const model = buildGroupRoundCardModel({
    messages: [rootUser, agent1Partial, agent2Done, guide, agent1Done],
    pendingPermissions: [],
    pendingSlots: [],
  });

  assert.deepEqual(
    model.entries.map(({ agent_id, agent_round_id }) => ({
      agent_id,
      agent_round_id,
    })),
    [
      { agent_id: "agent-1", agent_round_id: "agent-1-round" },
      { agent_id: "agent-2", agent_round_id: "agent-2-round" },
    ],
  );
  assert.deepEqual(
    flattenGroupRoundRenderOrder(model),
    [
      "user:user-root-display-order",
      "user:user-guide-display-order",
      "agent:agent-1",
      "agent:agent-2",
    ],
  );
});

test("Room keeps a permission-first execution on its agent_round node", async () => {
  const {
    buildGroupAgentTimelineNodeId,
    projectGroupAgentTimeline,
  } = await import("@/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts");
  const rootUser = userMessage({
    content: "需要 Agent 执行",
    messageId: "user-permission-first",
    roundId: "round-permission-first",
    timestamp: 1,
  });
  const permission = {
    agent_id: "agent-permission-first",
    agent_round_id: "agent-round-permission-first",
    interaction_mode: "question",
    request_id: "permission-first",
    round_id: "round-permission-first",
    tool_input: {
      questions: [{
        options: [{ label: "继续" }],
        question: "是否继续？",
      }],
    },
    tool_name: "AskUserQuestion",
  };
  const slot = {
    agent_id: permission.agent_id,
    agent_round_id: permission.agent_round_id,
    index: 0,
    msg_id: "slot-permission-first",
    round_id: permission.round_id,
    status: "streaming",
    timestamp: 2,
  };
  const message = assistantMessage({
    agentId: permission.agent_id,
    agentRoundId: permission.agent_round_id,
    messageId: "assistant-permission-first",
    roundId: permission.round_id,
    status: "streaming",
    text: "继续执行中",
    timestamp: 3,
  });
  const agentNodeId = buildGroupAgentTimelineNodeId(
    permission.round_id,
    `${permission.agent_id}:agent-round:${permission.agent_round_id}`,
  );
  const project = ({ messages, permissions, slots }) => (
    projectGroupAgentTimeline({
      messageGroups: new Map([[permission.round_id, messages]]),
      pendingPermissionGroups: new Map([
        [permission.round_id, permissions],
      ]),
      pendingSlotGroups: new Map([[permission.round_id, slots]]),
      roundIds: [permission.round_id],
    })
  );

  const permissionFirst = project({
    messages: [rootUser],
    permissions: [permission],
    slots: [],
  });
  const withSlot = project({
    messages: [rootUser],
    permissions: [permission],
    slots: [slot],
  });
  const withMessage = project({
    messages: [rootUser, message],
    permissions: [permission],
    slots: [slot],
  });

  assert.deepEqual(permissionFirst.roundIds, [
    "round-permission-first",
    agentNodeId,
  ]);
  assert.deepEqual(withSlot.roundIds, permissionFirst.roundIds);
  assert.deepEqual(withMessage.roundIds, permissionFirst.roundIds);
  assert.equal(
    permissionFirst.pendingPermissionGroups.get(agentNodeId)?.[0]?.request_id,
    permission.request_id,
  );
  assert.equal(
    permissionFirst.pendingPermissionGroups.get("round-permission-first")?.length,
    0,
    "the permission must never render on the generic root before its slot arrives",
  );
});

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

test("Room late permission enriches an observed slot without moving its Agent", async () => {
  const { buildRoomAgentRoundEntries } = await import("@/features/conversation/room/group/round/round-agent-model.ts");
  const {
    syncRoomAgentExecutionsFromPermissions,
    syncRoomAgentExecutionsFromSlots,
  } = await import("@/hooks/agent/runtime/model/room-agent-execution-state.ts");
  const slots = [
    {
      agent_id: "agent-a",
      agent_round_id: "agent-round-a",
      index: 0,
      msg_id: "slot-a",
      round_id: "round-late-permission",
      status: "streaming",
      timestamp: 1,
    },
    {
      agent_id: "agent-b",
      agent_round_id: "agent-round-b",
      index: 1,
      msg_id: "slot-b",
      round_id: "round-late-permission",
      status: "streaming",
      timestamp: 2,
    },
  ];
  const permissionForSecond = {
    agent_id: "agent-b",
    agent_round_id: "agent-round-b",
    request_id: "permission-b-late",
    round_id: "round-late-permission",
    tool_input: { command: "echo b" },
    tool_name: "Bash",
  };
  const observed = syncRoomAgentExecutionsFromSlots([], slots);
  const enriched = syncRoomAgentExecutionsFromPermissions(
    observed,
    [permissionForSecond],
    3,
  );

  assert.deepEqual(
    buildRoomAgentRoundEntries(
      [],
      slots,
      [permissionForSecond],
      enriched,
    ).map((entry) => entry.agent_round_id),
    ["agent-round-a", "agent-round-b"],
    "a question or permission arriving for B must not move B above the visible A card",
  );
  assert.deepEqual(
    enriched.map((state) => state.display_order),
    observed.map((state) => state.display_order),
  );
});

test("Room execution anchors seed canonical history and slot order before live appends", async () => {
  const { buildRoomAgentRoundEntries } = await import("@/features/conversation/room/group/round/round-agent-model.ts");
  const {
    syncRoomAgentExecutionsFromMessages,
    syncRoomAgentExecutionsFromSlots,
  } = await import("@/hooks/agent/runtime/model/room-agent-execution-state.ts");
  const firstFinishedLater = assistantMessage({
    agentId: "agent-a",
    agentRoundId: "agent-round-a-history",
    displayOrder: 1_000,
    isComplete: true,
    messageId: "assistant-a-history",
    roundId: "round-canonical-seed",
    status: "done",
    stopReason: "end_turn",
    text: "A 后完成",
    timestamp: 40,
  });
  const secondFinishedEarlier = assistantMessage({
    agentId: "agent-b",
    agentRoundId: "agent-round-b-history",
    displayOrder: 2_000,
    isComplete: true,
    messageId: "assistant-b-history",
    roundId: "round-canonical-seed",
    status: "done",
    stopReason: "end_turn",
    text: "B 先完成",
    timestamp: 30,
  });
  const completionOrderedMessages = [
    secondFinishedEarlier,
    firstFinishedLater,
  ];
  const historyStates = syncRoomAgentExecutionsFromMessages(
    [],
    completionOrderedMessages,
  );
  assert.deepEqual(
    buildRoomAgentRoundEntries(
      completionOrderedMessages,
      [],
      [],
      historyStates,
    ).map((entry) => entry.agent_id),
    ["agent-a", "agent-b"],
    "completion timestamps must not replace the backend Agent start order",
  );
  assert.deepEqual(
    historyStates.map((state) => state.display_order),
    [1_000, 2_000],
  );

  const reverseSlots = [
    {
      agent_id: "agent-b",
      agent_round_id: "agent-round-b-slot",
      index: 1,
      msg_id: "slot-b-canonical",
      round_id: "round-slot-seed",
      status: "streaming",
      timestamp: 2,
    },
    {
      agent_id: "agent-a",
      agent_round_id: "agent-round-a-slot",
      index: 0,
      msg_id: "slot-a-canonical",
      round_id: "round-slot-seed",
      status: "streaming",
      timestamp: 2,
    },
  ];
  const slotStates = syncRoomAgentExecutionsFromSlots([], reverseSlots);
  assert.deepEqual(
    buildRoomAgentRoundEntries([], reverseSlots, [], slotStates).map(
      (entry) => entry.agent_id,
    ),
    ["agent-a", "agent-b"],
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

function flattenGroupRoundRenderOrder(model) {
  const order = model.userMessages.map(
    ({ message }) => `user:${message.message_id}`,
  );
  for (const entry of model.entries) {
    order.push(...entry.guidedUserMessages.map(
      ({ message }) => `user:${message.message_id}`,
    ));
    order.push(`agent:${entry.agent_id}`);
  }
  return order;
}
