import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

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

test("scroll-to-latest requires real viewport overflow", async () => {
  const { hasScrollableOverflow } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/follow-scroll-model.ts",
  );
  assert.equal(
    hasScrollableOverflow(
      { clientHeight: 500, scrollHeight: 500, scrollTop: 0 },
    ),
    false,
    "an empty or short conversation must not expose a scroll-to-latest action",
  );
  assert.equal(
    hasScrollableOverflow(
      { clientHeight: 500, scrollHeight: 501, scrollTop: 0 },
    ),
    false,
    "sub-pixel layout rounding must not create a false scroll affordance",
  );
  assert.equal(
    hasScrollableOverflow(
      { clientHeight: 500, scrollHeight: 502, scrollTop: 0 },
    ),
    true,
    "real overflow must preserve the scroll-to-latest affordance",
  );
});

test("scroll events only resume following near the bottom", async () => {
  const { isNearScrollBottom } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/follow-scroll-model.ts",
  );
  assert.equal(
    isNearScrollBottom(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 2_000 },
    ),
    false,
    "an intermediate downward animation frame must not disable following",
  );
  assert.equal(
    isNearScrollBottom(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 1_800 },
    ),
    false,
    "Room layout movement must not be mistaken for user upward scrolling",
  );
  assert.equal(
    isNearScrollBottom(
      { clientHeight: 500, scrollHeight: 5_000, scrollTop: 4_450 },
    ),
    true,
    "scrolling back near the bottom must restore following",
  );
});

test("Room streaming revisions keep the follow key fresh for non-last Agent output", async () => {
  const { buildConversationScrollContentKey } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/follow-scroll-model.ts",
  );
  const streaming = assistantMessage({
    agentId: "agent-streaming",
    agentRoundId: "agent-round-streaming",
    messageId: "assistant-streaming",
    text: "第一段",
    timestamp: 1,
  });
  const later = assistantMessage({
    agentId: "agent-later",
    agentRoundId: "agent-round-later",
    messageId: "assistant-later",
    text: "较晚进入数组的并行回复",
    timestamp: 2,
  });

  const before = buildConversationScrollContentKey(
    "room:group:conversation",
    [streaming, later],
  );
  const after = buildConversationScrollContentKey(
    "room:group:conversation",
    [{
      ...streaming,
      content: [{ type: "text", text: "第一段继续输出" }],
    }, later],
  );

  assert.notEqual(
    before,
    after,
    "任意并行 Agent 的流式正文增长都必须触发主 Room 的贴底事务",
  );
});

test("auto follow settles again after virtual Room measurement", async () => {
  const { BottomScrollAnimator } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/scroll/scroll-animation.ts",
  );
  const frames = [];
  const originalWindow = globalThis.window;
  globalThis.window = {
    cancelAnimationFrame: () => {},
    requestAnimationFrame: (callback) => {
      frames.push(callback);
      return frames.length;
    },
  };
  try {
    const container = {
      clientHeight: 500,
      scrollHeight: 1_000,
      scrollTop: 0,
    };
    const animator = new BottomScrollAnimator(() => container, () => {});
    animator.scroll("auto");
    assert.equal(container.scrollTop, 500);
    assert.equal(
      frames.length,
      1,
      "auto follow needs one post-measurement settlement frame",
    );

    container.scrollHeight = 1_300;
    frames.shift()(performance.now());
    assert.equal(
      container.scrollTop,
      800,
      "virtual list height changes after layout must still finish at the bottom",
    );
  } finally {
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("Room renders permission details and actions without opening Thread", async () => {
  const { GroupAgentStatusCard } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-agent-status-card.tsx",
  );
  const { GroupConversationRound } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-conversation-round.tsx",
  );
  const { I18nProvider } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-provider.tsx",
  );
  const permission = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-1",
    interaction_mode: "permission",
    request_id: "permission-1",
    risk_label: "执行命令",
    risk_level: "medium",
    round_id: "round-root",
    summary: "需要人工确认",
    tool_input: { command: "echo permission-required" },
    tool_name: "Bash",
  };
  const provider = (child) => React.createElement(
    I18nProvider,
    null,
    child,
  );

  const agentCardHtml = renderToStaticMarkup(provider(React.createElement(
    GroupAgentStatusCard,
    {
      agentAvatar: null,
      agentId: "agent-1",
      agentName: "Dev",
      isThreadActive: false,
      messages: [],
      onClickThread: () => {},
      onPermissionResponse: () => true,
      pendingPermissions: [permission],
      status: "pending",
      timestamp: 1,
    },
  )));
  assert.match(
    agentCardHtml,
    /echo permission-required/,
    "Agent 卡片必须直接展示待审批操作的具体内容",
  );
  assert.match(agentCardHtml, />允许</);
  assert.match(agentCardHtml, />拒绝</);

  const permissionOnlyRoundHtml = renderToStaticMarkup(provider(
    React.createElement(GroupConversationRound, {
      renderer: {
        agentAvatarMap: {},
        agentNameMap: {},
        currentAgentAvatar: null,
        currentAgentName: "Dev",
        currentUserAvatar: null,
        isLastRoundPendingPermissions: [permission],
        onPermissionResponse: () => true,
        onStopAgentRound: () => {},
        runtimePhase: null,
      },
      state: {
        index: 0,
        isLast: true,
        isLive: true,
        isLoaded: true,
        messages: [],
        pendingPermissions: [permission],
        pendingSlots: [],
        rootRoundId: "round-root",
        roundId: "round-root",
      },
    }),
  ));
  assert.match(
    permissionOnlyRoundHtml,
    /echo permission-required/,
    "权限先于 Agent 消息到达时，主 Room 也不能丢失审批入口",
  );
});

test("resolved history rounds remain only when visible content was projected", async () => {
  const {
    buildIndexedTimelineRoundIds,
    filterResolvedEmptyRoundIndexItems,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/timeline/timeline-model.ts",
  );
  const visible = roundIndexItem("round-visible");
  const internal = roundIndexItem("goal_continuation_private");

  const unresolvedItems = filterResolvedEmptyRoundIndexItems(
    [visible, internal],
    [visible.roundId],
    [],
  );
  assert.deepEqual(
    buildIndexedTimelineRoundIds(unresolvedItems, [visible.roundId]),
    [visible.roundId, internal.roundId],
    "an unresolved neighbor remains as an invisible history load anchor",
  );

  const resolvedEmptyItems = filterResolvedEmptyRoundIndexItems(
    [visible, internal],
    [visible.roundId],
    [internal.roundId],
  );
  assert.deepEqual(
    resolvedEmptyItems.map((item) => item.roundId),
    [visible.roundId],
    "a resolved round with no visible content must leave no placeholder",
  );

  const resolvedVisibleItems = filterResolvedEmptyRoundIndexItems(
    [visible, internal],
    [visible.roundId, internal.roundId],
    [internal.roundId],
  );
  assert.deepEqual(
    resolvedVisibleItems.map((item) => item.roundId),
    [visible.roundId, internal.roundId],
    "a resolved round with visible content stays for the real message card",
  );
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

test("Room slot terminal state cannot be revived by a late running event", async () => {
  const {
    reconcileAgentRoundPendingSlots,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts",
  );
  const cancelledSlot = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-stopped",
    msg_id: "slot-stopped",
    round_id: "round-root",
    status: "cancelled",
    timestamp: 10,
  };

  assert.deepEqual(
    reconcileAgentRoundPendingSlots(
      [cancelledSlot],
      "agent-round-stopped",
      false,
    ),
    [cancelledSlot],
    "迟到的 non-terminal 事件不能把已停止槽位改回 streaming",
  );
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

test("recoverable malformed tool results stay out of the user timeline", async () => {
  const {
    buildVisibleOrderedAssistantEntries,
    collectHiddenToolUseIds,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/controller/projection/message-item-ordering.ts",
  );
  const content = [
    {
      type: "tool_use",
      id: "tool-malformed",
      name: "WebFetch",
      input: {},
      metadata: {
        _nexus_internal_kind: "malformed_tool_input",
      },
    },
    {
      type: "tool_result",
      tool_use_id: "tool-malformed",
      content: "Tool input was not valid JSON",
      is_error: true,
      metadata: {
        _nexus_internal_kind: "malformed_tool_input",
      },
    },
    { type: "text", text: "模型已自行修正并继续。" },
  ];
  const hiddenToolUseIds = collectHiddenToolUseIds(content, new Set());
  const entries = buildVisibleOrderedAssistantEntries({
    hiddenToolNames: new Set(),
    hiddenToolUseIds,
    isLoading: false,
    mergedContent: content,
    mergedContentSourceMessageIds: ["assistant-1", "assistant-2", "assistant-3"],
    sourceMessageOrderById: new Map([
      ["assistant-1", 0],
      ["assistant-2", 1],
      ["assistant-3", 2],
    ]),
    systemEventBlocks: [],
  });

  assert.deepEqual([...hiddenToolUseIds], ["tool-malformed"]);
  assert.deepEqual(
    entries.map(({ block }) => block),
    [{ type: "text", text: "模型已自行修正并继续。" }],
  );
});

test("recoverable malformed tool results stay out of process error counts", async () => {
  const { buildProcessSummary } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/process/message-process-summary.ts",
  );
  const summary = buildProcessSummary({
    pendingPermissionCount: 0,
    processContent: [
      {
        type: "tool_use",
        id: "tool-malformed",
        name: "WebFetch",
        input: {},
        metadata: {
          _nexus_internal_kind: "malformed_tool_input",
        },
      },
      {
        type: "tool_result",
        tool_use_id: "tool-malformed",
        content: "Tool input was not valid JSON",
        is_error: true,
        metadata: {
          _nexus_internal_kind: "malformed_tool_input",
        },
      },
    ],
  });

  assert.equal(summary, "查看过程");
});

test("recoverable malformed tool use does not keep the activity indicator busy", async () => {
  const { resolveContentActivityState } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/activity/message-content-activity.ts",
  );
  assert.equal(
    resolveContentActivityState({
      consumedBlockIndexes: new Set(),
      content: [{
        type: "tool_use",
        id: "tool-malformed",
        name: "WebFetch",
        input: {},
        metadata: {
          _nexus_internal_kind: "malformed_tool_input",
        },
      }],
      hiddenToolNames: new Set(),
      resolvedToolUseIds: new Set(),
    }),
    "thinking",
  );
});

test("history restores only the latest assistant round error", async () => {
  const {
    DEFAULT_ASSISTANT_ERROR_MESSAGE,
    latestAssistantResultErrorMessage,
    resolveAssistantResultErrorMessage,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/message/assistant-message-model.ts",
  );
  const failed = assistantMessage({
    messageId: "assistant-failed",
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      errors: ["", "provider stream failed"],
      is_error: true,
      num_turns: 1,
      subtype: "error",
      timestamp: 2,
    },
    roundId: "round-failed",
    text: "",
    timestamp: 2,
  });

  assert.equal(
    latestAssistantResultErrorMessage([failed]),
    "provider stream failed",
  );
  assert.equal(
    latestAssistantResultErrorMessage([
      failed,
      assistantMessage({
        messageId: "assistant-retrying",
        roundId: "round-retrying",
        text: "正在重试",
        timestamp: 3,
      }),
    ]),
    null,
    "a newer active round must suppress the previous terminal error",
  );
  assert.equal(
    latestAssistantResultErrorMessage([
      assistantMessage({
        messageId: "assistant-room-failed",
        roundId: "room-round-1",
        resultSummary: {
          duration_api_ms: 0,
          duration_ms: 0,
          errors: ["slot provider failed"],
          is_error: true,
          num_turns: 1,
          subtype: "error",
          timestamp: 4,
        },
        text: "",
        timestamp: 4,
      }),
      assistantMessage({
        messageId: "assistant-room-success",
        roundId: "room-round-1",
        resultSummary: {
          duration_api_ms: 0,
          duration_ms: 0,
          is_error: false,
          num_turns: 1,
          subtype: "success",
          timestamp: 5,
        },
        text: "另一个 Agent 完成",
        timestamp: 5,
      }),
    ]),
    "slot provider failed",
    "same root round must retain a failing Room slot",
  );
  assert.equal(
    resolveAssistantResultErrorMessage({
      duration_api_ms: 0,
      duration_ms: 0,
      is_error: true,
      num_turns: 0,
      subtype: "error",
    }),
    DEFAULT_ASSISTANT_ERROR_MESSAGE,
  );
});

test("terminal round status keeps its displayable error message", async () => {
  const { parseRoundStatusEventPayload } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-data.ts",
  );

  assert.deepEqual(
    parseRoundStatusEventPayload({
      is_terminal: true,
      message: "query: provider request failed",
      result_subtype: "error",
      round_id: "round-error",
      status: "error",
    }),
    {
      error_message: "query: provider request failed",
      is_terminal: true,
      result_subtype: "error",
      round_id: "round-error",
      status: "error",
    },
  );
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
    ["user-root"],
    "Room 主时间线不渲染已重挂的引导消息",
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

test("Room Composer hides the global stop action when no stop capability is supplied", async () => {
  const {
    projectComposerActions,
    projectComposerInput,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/controller/composer-view-projections.ts",
  );
  const base = {
    canCreateGoal: true,
    compact: false,
    goalCreateBlockedReason: null,
    input: "",
    inputState: projectComposerInput("", 0),
    isGoalCreating: false,
    isGoalMode: false,
    isPreparingAttachments: false,
    runtimeState: {
      activity: "replying",
      canStopGeneration: true,
      isAwaitingPermission: false,
      sessionBusy: true,
    },
  };

  assert.equal(
    projectComposerActions({ ...base, hasStopAction: false }).shouldShowStopButton,
    false,
  );
  assert.equal(
    projectComposerActions({ ...base, hasStopAction: true }).shouldShowStopButton,
    true,
  );
});

test("message protocol preserves CC rich blocks and contains unknown provider blocks", async () => {
  const {
    parseConversationMessage,
    parseStreamMessage,
  } = await server.ssrLoadModule(
    "/src/lib/conversation/message-protocol.ts",
  );

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
});

test("stream reducer exposes tool calls and removes terminal empty assistants", async () => {
  const { applyStreamMessage } = await server.ssrLoadModule(
    "/src/hooks/agent/message/stream-message-reducer.ts",
  );
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

test("Room interruption projection follows the slot identity without a ghost card", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const slot = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-stopped",
    msg_id: "slot-stopped",
    round_id: "round-root",
    status: "streaming",
    timestamp: 20,
  };
  const stream = assistantMessage({
    agentRoundId: "agent-round-stopped",
    messageId: "assistant-stopped-stream",
    status: "streaming",
    text: "",
    timestamp: 21,
  });
  const interrupted = {
    ...assistantMessage({
      agentId: "agent-1",
      isComplete: true,
      messageId: "assistant_result_round-root",
      resultSummary: {
        duration_api_ms: 0,
        duration_ms: 0,
        is_error: false,
        num_turns: 0,
        subtype: "interrupted",
        timestamp: 22,
      },
      status: "cancelled",
      text: "",
      timestamp: 22,
    }),
    // 兼容旧事件：结果没有 agent_round_id，但 parent_id 仍指向 slot。
    agent_round_id: undefined,
    parent_id: "slot-stopped",
  };

  const entries = buildRoomAgentRoundEntries([stream, interrupted], [slot]);
  assert.equal(entries.length, 1);
  assert.equal(entries[0]?.agent_round_id, "agent-round-stopped");
  assert.equal(entries[0]?.status, "cancelled");
  assert.deepEqual(
    entries[0]?.assistant_messages.map((message) => message.message_id),
    ["assistant-stopped-stream"],
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

test("Room places pending Agent cards before streaming output", async () => {
  const { buildGroupRoundCardModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const {
    buildGroupAgentTimelineNodeId,
    projectGroupAgentTimeline,
  } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts",
  );
  const stream = assistantMessage({
    agentId: "agent-streaming",
    agentRoundId: "round-streaming",
    messageId: "assistant-streaming",
    text: "正在输出正文",
    timestamp: 5,
  });
  const slots = [
    {
      agent_id: "agent-streaming",
      agent_round_id: "round-streaming",
      index: 0,
      msg_id: "slot-streaming",
      round_id: "round-root",
      status: "streaming",
      timestamp: 1,
    },
    {
      agent_id: "agent-pending",
      agent_round_id: "round-pending",
      index: 1,
      msg_id: "slot-pending",
      round_id: "round-root",
      status: "pending",
      timestamp: 2,
    },
  ];

  const model = buildGroupRoundCardModel({
    agentAvatarMap: {},
    agentNameMap: {},
    messages: [stream],
    pendingPermissions: [],
    pendingSlots: slots,
  });
  assert.deepEqual(
    model.entries.map(({ agent_id, status }) => ({ agent_id, status })),
    [
      { agent_id: "agent-pending", status: "pending" },
      { agent_id: "agent-streaming", status: "streaming" },
    ],
  );

  const projection = projectGroupAgentTimeline({
    messageGroups: new Map([["round-root", [stream]]]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map([["round-root", slots]]),
    roundIds: ["round-root"],
  });
  assert.deepEqual(projection.roundIds, [
    buildGroupAgentTimelineNodeId(
      "round-root",
      "agent-pending:agent-round:round-pending",
    ),
    buildGroupAgentTimelineNodeId(
      "round-root",
      "agent-streaming:agent-round:round-streaming",
    ),
  ]);
});

test("Room Agent timestamp stays on start while active and switches to finish at terminal", async () => {
  const { buildRoomAgentRoundEntries } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/round/round-agent-model.ts",
  );
  const { buildGroupAgentStatusModel } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/thread/round-card/group-round-card-model.ts",
  );
  const slot = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-stable-time",
    msg_id: "slot-stable-time",
    round_id: "round-root",
    status: "streaming",
    timestamp: 2,
  };
  const stream = assistantMessage({
    agentRoundId: "agent-round-stable-time",
    messageId: "assistant-stable-time",
    text: "流式快照更新时间不能改 header",
    timestamp: 20,
  });
  const active = buildRoomAgentRoundEntries([stream], [slot])[0];
  assert.equal(active?.timestamp, 2);
  assert.equal(buildGroupAgentStatusModel({
    labels: {
      failed: "Failed",
      stopped: "Stopped",
      waitingPermission: "Waiting",
    },
    messages: active.assistant_messages,
    pendingPermissions: [],
    status: active.status,
    timestamp: active.timestamp,
  }).timestamp, 2);

  const result = assistantMessage({
    agentRoundId: "agent-round-stable-time",
    isComplete: true,
    messageId: "assistant-stable-time",
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      is_error: false,
      num_turns: 1,
      result: "最终回复",
      subtype: "success",
      timestamp: 30,
    },
    status: "done",
    stopReason: "end_turn",
    text: "最终回复",
    timestamp: 25,
  });
  const terminal = buildRoomAgentRoundEntries([result])[0];
  assert.equal(terminal?.timestamp, 30);
});

test("Room projects every agent_round as a stable chronological feed node", async () => {
  const {
    buildGroupAgentTimelineNodeId,
    projectGroupAgentTimeline,
  } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts",
  );
  const rootUser = userMessage({
    content: "一起分析",
    messageId: "user-agent-node-root",
    roundId: "round-root",
    timestamp: 1,
  });
  const completed = assistantMessage({
    agentId: "agent-2",
    agentRoundId: "agent-2-node",
    isComplete: true,
    messageId: "assistant-agent-2-node",
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
  const activeStream = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "agent-1-node",
    messageId: "assistant-agent-1-node",
    text: "Agent1 仍在继续",
    timestamp: 7,
  });
  const consumedGuide = userMessage({
    agentRoundId: "agent-1-node",
    content: "Agent1 再补充一个维度",
    deliveryPolicy: "guide",
    messageId: "user-guide-agent-node",
    roundId: "round-root",
    sourceRoundId: "round-guide-agent-node",
    targetAgentIds: ["agent-1"],
    timestamp: 6,
  });
  const laterUser = userMessage({
    content: "另一个后续问题",
    messageId: "user-later-root",
    roundId: "round-later",
    timestamp: 5,
  });
  const activeSlot = {
    agent_id: "agent-1",
    agent_round_id: "agent-1-node",
    index: 0,
    msg_id: "slot-agent-1-node",
    round_id: "round-root",
    status: "streaming",
    timestamp: 2,
  };
  const activeProjection = projectGroupAgentTimeline({
    messageGroups: new Map([
      ["round-root", [rootUser, completed, activeStream, consumedGuide]],
      ["round-later", [laterUser]],
    ]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map([["round-root", [activeSlot]]]),
    roundIds: ["round-root", "round-later"],
  });
  const agent1NodeId = buildGroupAgentTimelineNodeId(
    "round-root",
    "agent-1:agent-round:agent-1-node",
  );
  const agent2NodeId = buildGroupAgentTimelineNodeId(
    "round-root",
    "agent-2:agent-round:agent-2-node",
  );
  assert.deepEqual(activeProjection.roundIds, [
    "round-root",
    agent2NodeId,
    "round-later",
    agent1NodeId,
  ]);
  assert.deepEqual(
    activeProjection.messageGroups.get(agent1NodeId)?.map(
      (message) => message.message_id,
    ),
    ["user-guide-agent-node", "assistant-agent-1-node"],
  );
  assert.equal(activeProjection.rootRoundIds.get(agent1NodeId), "round-root");

  const terminal = assistantMessage({
    agentId: "agent-1",
    agentRoundId: "agent-1-node",
    isComplete: true,
    messageId: "assistant-agent-1-node",
    resultSummary: {
      duration_api_ms: 20,
      duration_ms: 30,
      is_error: false,
      num_turns: 2,
      result: "Agent1 完成",
      subtype: "success",
      timestamp: 8,
    },
    status: "done",
    stopReason: "end_turn",
    text: "Agent1 完成",
    timestamp: 8,
  });
  const terminalProjection = projectGroupAgentTimeline({
    messageGroups: new Map([
      ["round-root", [rootUser, completed, terminal, consumedGuide]],
      ["round-later", [laterUser]],
    ]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map(),
    roundIds: ["round-root", "round-later"],
  });
  assert.equal(terminalProjection.roundIds.at(-1), agent1NodeId);
  assert.equal(
    terminalProjection.roundIds.includes(agent1NodeId),
    true,
    "pending -> terminal must not change the visual node identity",
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
  roundId = "round-root",
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
    round_id: roundId,
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
