import assert from "node:assert/strict";
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

test("deferred input ACK keeps queued user text out of the timeline", async () => {
  const { replaceOptimisticUserMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts",
  );
  const optimistic = userMessage({
    content: "这条还没有被智能体消费",
    messageId: "local-message",
    roundId: "local-message",
    timestamp: 1,
  });

  assert.deepEqual(
    replaceOptimisticUserMessage(
      [optimistic],
      "local-message",
      "user-message",
      "round-message",
      false,
    ),
    [],
    "a queued ACK must remove the optimistic timeline message",
  );
  assert.deepEqual(
    replaceOptimisticUserMessage(
      [optimistic],
      "local-message",
      "user-message",
      "round-message",
      true,
    ).map(({ message_id, round_id }) => ({ message_id, round_id })),
    [{ message_id: "user-message", round_id: "round-message" }],
    "a committed ACK still canonicalizes normal user messages",
  );
});

test("deferred ACK cannot remove an already applied canonical user message", async () => {
  const { replaceOptimisticUserMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts",
  );
  const optimistic = userMessage({
    content: "这条正在等待 ACK",
    messageId: "local-message",
    roundId: "local-message",
    timestamp: 1,
  });
  const canonical = userMessage({
    content: "这条已经被智能体消费",
    messageId: "user-message",
    roundId: "round-message",
    timestamp: 2,
  });

  assert.deepEqual(
    replaceOptimisticUserMessage(
      [optimistic, canonical],
      "local-message",
      "user-message",
      "round-message",
      false,
    ).map(({ message_id, round_id }) => ({ message_id, round_id })),
    [{ message_id: "user-message", round_id: "round-message" }],
    "a late deferred ACK must remove only the optimistic copy",
  );
});

test("Room pending slot keeps the backend display index", async () => {
  const { mergeChatAckPendingSlots } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts",
  );
  const slots = mergeChatAckPendingSlots([], {
    pending: [{
      agent_id: "agent-1",
      agent_round_id: "agent-round-1",
      index: 7,
      msg_id: "slot-1",
      status: "streaming",
      timestamp: 10,
    }],
    pending_snapshot: true,
    round_id: "round-root",
  });

  assert.equal(slots[0]?.index, 7);
});

test("Room pending queue shows only user-authored guidance", async () => {
  const { projectRoomPendingInputQueueItems } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/panel/controller/group-chat-panel-projection.ts",
  );
  const items = [
    { id: "user", source: "user" },
    { id: "public-mention", source: "agent_public_mention" },
    { id: "directed-message", source: "agent_room_directed_message" },
  ];

  assert.deepEqual(
    projectRoomPendingInputQueueItems(items).map((item) => item.id),
    ["user"],
  );
});

test("blocked goals stay inline instead of opening a resume confirmation", async () => {
  const { buildGoalControllerProjection } = await server.ssrLoadModule(
    "/src/features/conversation/shared/goal/goal-model.ts",
  );
  const goal = {
    continuation_count: 1,
    created_at: "2026-07-14T00:00:00Z",
    empty_progress_count: 3,
    id: "goal-1",
    objective: "Replace this objective directly",
    session_key: "room:group:conversation-1",
    status: "blocked",
    updated_at: "2026-07-14T00:01:00Z",
    version: 2,
  };
  const projection = buildGoalControllerProjection({
    dialog: { goal, kind: "resume" },
    draft: null,
    goal,
    phase: null,
  });

  assert.equal(projection.canResume, true);
  assert.deepEqual(projection.dialog, { kind: "none" });
});

test("Room no-reply control markers never become visible assistant blocks", async () => {
  const { buildVisibleOrderedAssistantEntries } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-ordering.ts",
  );
  const entries = buildVisibleOrderedAssistantEntries({
    hiddenToolNames: new Set(),
    hiddenToolUseIds: new Set(),
    isLoading: false,
    mergedContent: [{ type: "text", text: "<nexus_room_no_reply/>" }],
    mergedContentSourceMessageIds: ["assistant-no-reply"],
    sourceMessageOrderById: new Map([["assistant-no-reply", 0]]),
    systemEventBlocks: [],
  });

  assert.deepEqual(entries, []);
});

test("Room no-reply control markers stay out of previews and result summaries", async () => {
  const { extractAgentPreviewText } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const { buildGroupAgentStatusModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const marker = "<nexus_room_no_reply/>";

  assert.equal(
    extractAgentPreviewText([assistantMessage({ text: marker, timestamp: 1 })]),
    "",
  );

  const status = buildGroupAgentStatusModel({
    labels: {
      failed: "Failed",
      stopped: "Stopped",
      waitingPermission: "Waiting",
    },
    messages: [],
    pendingPermissions: [],
    resultSummary: {
      duration_api_ms: 0,
      duration_ms: 0,
      is_error: false,
      num_turns: 1,
      result: marker,
      subtype: "interrupted",
      timestamp: 1,
    },
    status: "cancelled",
  });
  assert.equal(status.summaryText, "Stopped");
});

test("consumed Room guide update moves beside its running assistant", async () => {
  const { parseConversationMessage } = await server.ssrLoadModule(
    "/src/lib/conversation/message-protocol.ts",
  );
  const { mergeLoadedMessages, upsertMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/message/message-collection-model.ts",
  );
  const {
    filterSupersededRoundIndexItems,
    groupMessagesByRound,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/timeline-model.ts",
  );
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );

  const rootUser = userMessage({
    content: "先分析",
    messageId: "user-root",
    roundId: "round-root",
    timestamp: 1,
  });
  const guideBeforeConsumption = userMessage({
    content: "然后给点建议",
    messageId: "user-guide",
    roundId: "round-guide",
    timestamp: 3,
  });
  const assistant = {
    agent_id: "agent-1",
    content: [{ type: "text", text: "最终建议" }],
    is_complete: false,
    message_id: "assistant-root",
    role: "assistant",
    round_id: "round-root",
    session_key: "room:group:conversation-1",
    stream_status: "streaming",
    timestamp: 2,
  };
  const consumedGuide = parseConversationMessage({
    ...guideBeforeConsumption,
    agent_id: "",
    delivery_policy: "guide",
    round_id: "round-root",
    source_round_id: "round-guide",
  });

  assert.ok(consumedGuide, "Room user updates allow an empty agent_id");
  const messages = upsertMessage(
    [rootUser, assistant, guideBeforeConsumption],
    consumedGuide,
  );
  const groups = groupMessagesByRound(messages);
  assert.equal(groups.has("round-guide"), false);

  const rootMessages = groups.get("round-root") ?? [];
  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: { "agent-1": "Agent1" },
    messages: rootMessages,
    pendingPermissions: [],
    pendingSlots: [],
  });
  assert.deepEqual(
    model.userMessages.map(({ message }) => message.message_id),
    ["user-root", "user-guide"],
  );
  assert.equal(model.entries.length, 1);
  assert.equal(model.entries[0]?.agent_id, "agent-1");

  const sourceIndex = roundIndexItem("round-guide", {
    hasUserMessage: true,
    timestamp: 3,
  });
  const targetIndex = roundIndexItem("round-root", {
    agentIds: ["agent-1"],
    isLive: true,
    timestamp: 1,
  });
  assert.deepEqual(
    filterSupersededRoundIndexItems([targetIndex, sourceIndex], messages)
      .map((item) => item.roundId),
    ["round-root"],
    "the consumed source round must not remain as an unloaded navigator card",
  );
  assert.deepEqual(
    filterSupersededRoundIndexItems([
      targetIndex,
      { ...sourceIndex, agentIds: ["agent-2"], isLive: true },
    ], messages).map((item) => item.roundId),
    ["round-root", "round-guide"],
    "a source round with another live agent must remain visible",
  );

  const mergedAfterStaleHistory = mergeLoadedMessages(
    [rootUser, assistant, guideBeforeConsumption],
    messages,
  );
  const groupsAfterStaleHistory = groupMessagesByRound(mergedAfterStaleHistory);
  assert.equal(
    groupsAfterStaleHistory.has("round-guide"),
    false,
    "a stale history response must not undo durable guidance reparenting",
  );
  assert.deepEqual(
    (groupsAfterStaleHistory.get("round-root") ?? [])
      .filter((message) => message.role === "user")
      .map((message) => message.message_id),
    ["user-root", "user-guide"],
  );
  assert.equal(
    mergedAfterStaleHistory.find(
      (message) => message.message_id === "user-guide",
    )?.delivery_policy,
    "guide",
    "a stale history response must not undo fields persisted with reparenting",
  );

  const refreshedGuide = {
    ...consumedGuide,
    attachments: [{ id: "attachment-1", name: "detail.txt" }],
    content: "然后给点更完整的建议",
    timestamp: 4,
  };
  const mergedAfterCanonicalHistory = mergeLoadedMessages(
    [rootUser, assistant, refreshedGuide],
    mergedAfterStaleHistory,
  );
  const canonicalGuide = mergedAfterCanonicalHistory.find(
    (message) => message.message_id === "user-guide",
  );
  assert.equal(canonicalGuide?.round_id, "round-root");
  assert.equal(canonicalGuide?.source_round_id, "round-guide");
  assert.equal(canonicalGuide?.content, "然后给点更完整的建议");
  assert.equal(canonicalGuide?.attachments?.[0]?.name, "detail.txt");
  assert.equal(canonicalGuide?.timestamp, 4);
});

test("late history cannot roll an assistant snapshot backward", async () => {
  const { mergeLoadedMessages, upsertMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/message/message-collection-model.ts",
  );

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

test("Room keeps separate agent_round entries for the same agent", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
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
});

test("Room canonical assistant replaces its temporary synthetic result", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
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

test("Room final replies stay in completion order around a guide", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
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
    agentAvatarMap: {},
    agentNameMap: { "agent-1": "Agent1", "agent-2": "Agent2" },
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
      { agent_id: "agent-2", agent_round_id: "agent-2-round" },
      { agent_id: "agent-1", agent_round_id: "agent-1-round" },
    ],
  );
  assert.deepEqual(
    flattenGroupRoundRenderOrder(model),
    [
      "user:user-root-display-order",
      "agent:agent-2",
      "user:user-guide-display-order",
      "agent:agent-1",
    ],
  );
});

test("late Room guidance does not reorder completed Agent cards", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: { "agent-1": "Agent1", "agent-2": "Agent2" },
    messages: [
      userMessage({
        content: "一起分析",
        messageId: "user-root-stable-completed",
        roundId: "round-root",
        timestamp: 1,
      }),
      assistantMessage({
        agentId: "agent-1",
        agentRoundId: "agent-1-completed",
        isComplete: true,
        messageId: "assistant-agent-1-completed",
        status: "done",
        stopReason: "end_turn",
        text: "Agent1 先完成",
        timestamp: 2,
      }),
      assistantMessage({
        agentId: "agent-2",
        agentRoundId: "agent-2-completed",
        isComplete: true,
        messageId: "assistant-agent-2-completed",
        status: "done",
        stopReason: "end_turn",
        text: "Agent2 后完成",
        timestamp: 4,
      }),
      userMessage({
        agentRoundId: "agent-1-completed",
        content: "这是 Agent1 实际消费的补充",
        deliveryPolicy: "guide",
        messageId: "user-guide-stable-completed",
        roundId: "round-root",
        sourceRoundId: "round-guide-stable-completed",
        targetAgentIds: ["agent-1"],
        timestamp: 5,
      }),
    ],
    pendingPermissions: [],
    pendingSlots: [],
  });

  assert.deepEqual(
    model.entries.map(({ agent_id }) => agent_id),
    ["agent-1", "agent-2"],
  );
  assert.deepEqual(
    flattenGroupRoundRenderOrder(model),
    [
      "user:user-root-stable-completed",
      "user:user-guide-stable-completed",
      "agent:agent-1",
      "agent:agent-2",
    ],
  );
});

test("Room keeps active Agent cards at the stable tail", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: {
      "agent-1": "Agent1",
      "agent-2": "Agent2",
      "agent-3": "Agent3",
    },
    messages: [
      assistantMessage({
        agentId: "agent-1",
        agentRoundId: "agent-1-active",
        messageId: "assistant-agent-1-latest",
        text: "Agent1 流式内容更新得更晚",
        timestamp: 20,
      }),
      assistantMessage({
        agentId: "agent-2",
        agentRoundId: "agent-2-active",
        messageId: "assistant-agent-2-earlier",
        text: "Agent2 仍在运行",
        timestamp: 10,
      }),
      assistantMessage({
        agentId: "agent-3",
        agentRoundId: "agent-3-completed",
        isComplete: true,
        messageId: "assistant-agent-3-completed",
        status: "done",
        stopReason: "end_turn",
        text: "Agent3 已完成",
        timestamp: 30,
      }),
      userMessage({
        agentRoundId: "agent-1-active",
        content: "Agent1 继续补充",
        deliveryPolicy: "guide",
        messageId: "user-guide-active-stable",
        roundId: "round-root",
        sourceRoundId: "round-guide-active-stable",
        targetAgentIds: ["agent-1"],
        timestamp: 40,
      }),
    ],
    pendingPermissions: [],
    pendingSlots: [
      {
        agent_id: "agent-1",
        agent_round_id: "agent-1-active",
        index: 0,
        msg_id: "slot-agent-1",
        round_id: "round-root",
        status: "streaming",
        timestamp: 2,
      },
      {
        agent_id: "agent-2",
        agent_round_id: "agent-2-active",
        index: 1,
        msg_id: "slot-agent-2",
        round_id: "round-root",
        status: "streaming",
        timestamp: 3,
      },
    ],
  });

  assert.deepEqual(
    model.entries.map(({ agent_id, status }) => ({ agent_id, status })),
    [
      { agent_id: "agent-3", status: "done" },
      { agent_id: "agent-1", status: "streaming" },
      { agent_id: "agent-2", status: "streaming" },
    ],
  );
  assert.deepEqual(
    flattenGroupRoundRenderOrder(model),
    [
      "agent:agent-3",
      "user:user-guide-active-stable",
      "agent:agent-1",
      "agent:agent-2",
    ],
  );
});

test("single-target Room guidance attaches only to its consuming agent", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const rootUser = userMessage({
    content: "先分别分析",
    messageId: "user-root-target-order",
    roundId: "round-root",
    timestamp: 1,
  });
  const legacyGuide = userMessage({
    content: "旧协议插话",
    deliveryPolicy: "guide",
    messageId: "user-guide-legacy",
    roundId: "round-root",
    sourceRoundId: "round-guide-legacy",
    timestamp: 2,
  });
  const multiTargetGuide = userMessage({
    content: "两位都补充",
    deliveryPolicy: "guide",
    messageId: "user-guide-multi",
    roundId: "round-root",
    sourceRoundId: "round-guide-multi",
    targetAgentIds: ["agent-1", "agent-2"],
    timestamp: 3,
  });
  const agent2Result = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "agent-2-old-round",
    isComplete: true,
    messageId: "assistant-agent-2",
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      is_error: false,
      num_turns: 1,
      result: "Agent2 已完成",
      subtype: "success",
      timestamp: 4,
    },
    status: "done",
    stopReason: "end_turn",
    text: "Agent2 已完成",
    timestamp: 4,
  });
  const agent1Stream = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "agent-1-live-round",
    messageId: "assistant-agent-1",
    text: "Agent1 原输出",
    timestamp: 5,
  });
  const targetedGuide = userMessage({
    content: "Agent1 改成比较 M4 和 M5",
    deliveryPolicy: "guide",
    messageId: "user-guide-agent-1",
    roundId: "round-root",
    sourceRoundId: "round-guide-agent-1",
    targetAgentIds: ["agent-1"],
    timestamp: 6,
  });
  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: { "agent-1": "Agent1", "agent-2": "Agent2" },
    messages: [
      rootUser,
      legacyGuide,
      multiTargetGuide,
      agent2Result,
      agent1Stream,
      targetedGuide,
    ],
    pendingPermissions: [],
    pendingSlots: [{
      agent_id: "agent-1",
      agent_round_id: "agent-1-live-round",
      msg_id: "slot-agent-1",
      round_id: "round-root",
      status: "streaming",
      timestamp: 5,
    }],
  });

  assert.deepEqual(
    model.userMessages.map(({ message }) => message.message_id),
    ["user-root-target-order", "user-guide-legacy", "user-guide-multi"],
  );
  assert.deepEqual(
    model.entries
      .filter((entry) => entry.status === "done")
      .map((entry) => entry.agent_id),
    ["agent-2"],
  );
  assert.deepEqual(model.entries[0]?.guidedUserMessages, []);
  assert.deepEqual(
    model.entries
      .filter((entry) => entry.status !== "done")
      .map((entry) => entry.agent_id),
    ["agent-1"],
  );
  assert.deepEqual(
    model.entries[1]?.guidedUserMessages.map(
      ({ message }) => message.message_id,
    ),
    ["user-guide-agent-1"],
  );
  assert.deepEqual(
    flattenGroupRoundRenderOrder(model),
    [
      "user:user-root-target-order",
      "user:user-guide-legacy",
      "user:user-guide-multi",
      "agent:agent-2",
      "user:user-guide-agent-1",
      "agent:agent-1",
    ],
  );
});

test("single-target Room guidance also attaches to a completed agent", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const completedGuide = userMessage({
    content: "完成前补充的约束",
    deliveryPolicy: "guide",
    messageId: "user-guide-completed",
    roundId: "round-root",
    sourceRoundId: "round-guide-completed",
    targetAgentIds: ["agent-2"],
    timestamp: 2,
  });
  const completedResult = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "agent-2-completed-round",
    isComplete: true,
    messageId: "assistant-agent-2-completed",
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      is_error: false,
      num_turns: 1,
      result: "已按补充约束完成",
      subtype: "success",
      timestamp: 3,
    },
    status: "done",
    stopReason: "end_turn",
    text: "已按补充约束完成",
    timestamp: 3,
  });
  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: { "agent-2": "Agent2" },
    messages: [
      userMessage({
        content: "初始问题",
        messageId: "user-root-completed",
        roundId: "round-root",
        timestamp: 1,
      }),
      completedGuide,
      completedResult,
    ],
    pendingPermissions: [],
    pendingSlots: [],
  });

  assert.deepEqual(
    flattenGroupRoundRenderOrder(model),
    [
      "user:user-root-completed",
      "user:user-guide-completed",
      "agent:agent-2",
    ],
  );
});

test("Room guidance stays on its exact consumed agent round", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
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
    agentAvatarMap: {},
    agentNameMap: { "agent-1": "Agent1" },
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

function userMessage({
  agentRoundId,
  content,
  deliveryPolicy,
  messageId,
  roundId,
  sourceRoundId,
  targetAgentIds,
  timestamp,
}) {
  return {
    agent_id: "",
    ...(agentRoundId ? { agent_round_id: agentRoundId } : {}),
    content,
    ...(deliveryPolicy ? { delivery_policy: deliveryPolicy } : {}),
    message_id: messageId,
    role: "user",
    round_id: roundId,
    session_key: "room:group:conversation-1",
    ...(sourceRoundId ? { source_round_id: sourceRoundId } : {}),
    ...(targetAgentIds ? { target_agent_ids: targetAgentIds } : {}),
    timestamp,
  };
}

function assistantMessage({
  agentId = "agent-1",
  agentRoundId,
  isComplete = false,
  messageId = "assistant-root",
  model,
  resultSummary,
  status = "streaming",
  stopReason,
  text,
  timestamp,
}) {
  return {
    agent_id: agentId,
    ...(agentRoundId ? { agent_round_id: agentRoundId } : {}),
    content: [{ type: "text", text }],
    is_complete: isComplete,
    message_id: messageId,
    ...(model ? { model } : {}),
    ...(resultSummary ? { result_summary: resultSummary } : {}),
    role: "assistant",
    round_id: "round-root",
    session_key: "room:group:conversation-1",
    ...(stopReason ? { stop_reason: stopReason } : {}),
    stream_status: status,
    timestamp,
  };
}

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

function roundIndexItem(roundId, overrides = {}) {
  return {
    agentIds: [],
    durationMs: null,
    hasUserMessage: false,
    isLive: false,
    roundId,
    status: null,
    timestamp: null,
    title: "",
    ...overrides,
  };
}
