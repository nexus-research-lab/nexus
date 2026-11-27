// @vitest-environment node

import { userMessage, assistantMessage } from "@/features/conversation/testing/message-fixtures.mjs";

import assert from "node:assert/strict";
import { test } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

test("Room streams and completes inside one stable Agent execution shell", async () => {
  const { GroupAgentReply } = await import("@/features/conversation/room/group/thread/round-card/group-agent-reply.tsx");
  const { I18nProvider } = await import("@/shared/i18n/i18n-provider.tsx");
  const tailMarker = "STREAM_TAIL_VISIBLE_AFTER_EIGHTY_CHARS";
  const text = `${"逐步输出的正文。".repeat(18)}${tailMarker}`;
  const message = assistantMessage({
    agentId: "agent-stream",
    agentRoundId: "agent-round-stream",
    messageId: "assistant-stream",
    status: "streaming",
    text,
    timestamp: 2,
  });
  const entry = {
    agent_id: "agent-stream",
    agent_round_id: "agent-round-stream",
    assistant_messages: [message],
    display_order: 0,
    entry_id: "agent-stream:agent-round:agent-round-stream",
    guidedUserMessages: [],
    pendingPermissions: [],
    pending_slot: {
      agent_id: "agent-stream",
      agent_round_id: "agent-round-stream",
      index: 0,
      msg_id: "slot-stream",
      round_id: "round-root",
      status: "streaming",
      timestamp: 1,
    },
    status: "streaming",
    stopAgentRoundId: "agent-round-stream",
    timestamp: 1,
  };
  const renderReply = (nextEntry) => renderToStaticMarkup(
    React.createElement(
      I18nProvider,
      null,
      React.createElement(GroupAgentReply, {
        agentMentionDirectory: { avatars: {}, names: { "agent-stream": "Stream Agent" } },
        entry: nextEntry,
        isThreadActive: false,
        onClickThread: () => {},
        onPermissionResponse: () => true,
        onStopAgentRound: () => {},
        roundId: "round-root",
      }),
    ),
  );
  const activeHtml = renderReply(entry);
  const resultSummary = {
    duration_api_ms: 10,
    duration_ms: 20,
    is_error: false,
    message_id: "result-stream",
    num_turns: 1,
    result: text,
    subtype: "success",
    timestamp: 3,
  };
  const terminalHtml = renderReply({
    ...entry,
    assistant_messages: [{
      ...message,
      is_complete: true,
      result_summary: resultSummary,
      stop_reason: "end_turn",
      stream_status: "done",
      timestamp: 3,
    }],
    pending_slot: {
      ...entry.pending_slot,
      status: "done",
    },
    result_summary: resultSummary,
    status: "done",
    timestamp: 3,
  });

  assert.match(
    activeHtml,
    new RegExp(tailMarker),
    "the public Room reply must grow in place while the Agent is streaming",
  );
  assert.match(
    activeHtml,
    /data-message-activity-elapsed/,
    "the shared activity indicator must remain visible in the public reply",
  );
  assert.doesNotMatch(
    activeHtml,
    /line-clamp-1/,
    "the live reply must not collapse into a one-line status preview",
  );
  assert.match(
    terminalHtml,
    new RegExp(tailMarker),
    "the public terminal result must be complete as soon as the backend snapshot arrives",
  );
  assert.doesNotMatch(
    terminalHtml,
    /正在回复/,
    "terminal Room replies must remove the transient activity indicator",
  );
  assert.equal(
    activeHtml.match(/data-room-agent-execution-shell/g)?.length,
    1,
  );
  assert.equal(
    terminalHtml.match(/data-room-agent-execution-shell/g)?.length,
    1,
  );
  assert.match(
    activeHtml,
    /data-room-agent-execution-shell="round-root:agent-stream:agent-round:agent-round-stream"/,
  );
  assert.match(
    terminalHtml,
    /data-room-agent-execution-shell="round-root:agent-stream:agent-round:agent-round-stream"/,
    "pending and terminal snapshots must retain the same outer execution identity",
  );
  const statusBeforeResultHtml = renderReply({
    ...entry,
    pending_slot: {
      ...entry.pending_slot,
      status: "done",
    },
    status: "done",
  });
  assert.match(
    statusBeforeResultHtml,
    new RegExp(tailMarker),
    "a terminal lifecycle event must not replace or hide the already visible stream",
  );
});

test("Room public activity survives the pause between reply text and tool work", async () => {
  const { GroupAgentExecutionShell } = await import("@/features/conversation/room/group/thread/round-card/group-agent-execution-shell.tsx");
  const { I18nProvider } = await import("@/shared/i18n/i18n-provider.tsx");
  const renderShell = (props) => renderToStaticMarkup(
    React.createElement(
      I18nProvider,
      null,
      React.createElement(GroupAgentExecutionShell, {
        agentAvatar: null,
        agentId: "agent-public-activity",
        agentName: "Researcher",
        isThreadActive: false,
        onClickThread: () => {},
        onPermissionResponse: () => true,
        pendingPermissions: [],
        roundId: "round-public-activity:agent-public-activity",
        timestamp: 1,
        ...props,
      }),
    ),
  );

  const pendingHtml = renderShell({
    messages: [],
    status: "pending",
  });
  assert.match(pendingHtml, /正在思考/);
  assert.doesNotMatch(
    pendingHtml,
    /room-agent-execution-enter/,
    "a pending shell must not add translated geometry during parallel growth",
  );
  assert.equal(
    pendingHtml.match(/data-loading-orb="active"/g)?.length,
    1,
    "a pending slot uses the shared activity surface exactly once",
  );
  assert.match(pendingHtml, /data-message-activity-indicator/);

  const completedPublicTurn = assistantMessage({
    agentId: "agent-public-activity",
    agentRoundId: "agent-round-public-activity",
    isComplete: true,
    messageId: "assistant-public-turn",
    roundId: "round-public-activity",
    status: "done",
    stopReason: "end_turn",
    text: "我先说明计划，随后继续在 Thread 中执行。",
    timestamp: 2,
  });
  const continuedHtml = renderShell({
    messages: [completedPublicTurn],
    status: "streaming",
  });
  assert.match(continuedHtml, /我先说明计划，随后继续在/);
  assert.match(continuedHtml, /中执行。/);
  assert.match(
    continuedHtml,
    /data-message-activity-elapsed/,
    "an active Agent Thread keeps a public activity row after an intermediate text turn completes",
  );
  assert.equal(
    continuedHtml.match(/data-loading-orb="active"/g)?.length,
    1,
    "the continued Thread activity stays inside the existing Agent card",
  );

  const toolContinuation = {
    ...assistantMessage({
      agentId: "agent-public-activity",
      agentRoundId: "agent-round-public-activity",
      isComplete: true,
      messageId: "assistant-public-tool-turn",
      roundId: "round-public-activity",
      status: "done",
      stopReason: "tool_use",
      text: "我先搜索产品线信息。",
      timestamp: 2,
    }),
    content: [
      { type: "text", text: "我先搜索产品线信息。" },
      {
        id: "tool-public-search",
        input: { query: "M3 product line" },
        name: "WebSearch",
        type: "tool_use",
      },
      {
        preceding_tool_use_ids: ["tool-public-search"],
        subtype: "tool_use_summary",
        text: "搜索产品线资料",
        type: "progress_update",
      },
    ],
  };
  const workingHtml = renderShell({
    messages: [toolContinuation],
    status: "streaming",
  });
  assert.match(workingHtml, /我先搜索产品线信息。/);
  assert.match(
    workingHtml,
    /网络搜索 M3 product line/,
    "Room main feed mirrors the running tool group header",
  );
  assert.doesNotMatch(workingHtml, /搜索产品线资料/);
  assert.match(workingHtml, /data-room-tool-activity/);

  assert.match(workingHtml, /data-process-activity-icon="search"/);
  assert.match(
    workingHtml,
    /text-\(--icon-muted\)[^>]*data-process-activity-icon="search"/,
  );
  assert.match(workingHtml, /text-primary/);
  assert.doesNotMatch(workingHtml, /data-tool-run-list|data-tool-run-id/);
  assert.equal(
    workingHtml.match(/data-loading-orb="active"/g)?.length,
    undefined,
    "the running tool header does not add a second loading spinner",
  );

  const toolOnlyContinuation = {
    ...assistantMessage({
      agentId: "agent-public-activity",
      agentRoundId: "agent-round-public-activity",
      isComplete: true,
      messageId: "assistant-public-tool-only-turn",
      roundId: "round-public-activity",
      status: "done",
      stopReason: "tool_use",
      text: "",
      timestamp: 3,
    }),
    content: [{
      id: "tool-public-write",
      input: { file_path: "research.md" },
      name: "Write",
      type: "tool_use",
    }],
  };
  const persistentReplyHtml = renderShell({
    messages: [completedPublicTurn, toolOnlyContinuation],
    status: "streaming",
  });
  assert.match(persistentReplyHtml, /我先说明计划，随后继续在/);
  assert.match(persistentReplyHtml, /写入内容 research\.md/);
  assert.ok(
    persistentReplyHtml.indexOf("我先说明计划")
      < persistentReplyHtml.indexOf("写入内容 research.md"),
    "a tool-only turn keeps the latest public reply in its fixed slot",
  );

  const workingAfterReplyHtml = renderShell({
    messages: [
      toolContinuation,
      assistantMessage({
        agentId: "agent-public-activity",
        agentRoundId: "agent-round-public-activity",
        messageId: "assistant-public-reply-after-tool",
        roundId: "round-public-activity",
        status: "streaming",
        text: "先同步当前进度",
        timestamp: 3,
      }),
    ],
    status: "streaming",
  });
  assert.match(workingAfterReplyHtml, /data-room-tool-activity/);
  assert.match(workingAfterReplyHtml, /网络搜索 M3 product line/);
  assert.doesNotMatch(workingAfterReplyHtml, /正在回复/);
  assert.doesNotMatch(workingAfterReplyHtml, /我先搜索产品线信息。/);
  assert.ok(
    workingAfterReplyHtml.indexOf("先同步当前进度")
      < workingAfterReplyHtml.indexOf("网络搜索 M3 product line"),
    "the current Room tool follows the newest visible reply",
  );

  const terminalHtml = renderShell({
    messages: [{
      ...toolContinuation,
      result_summary: {
        duration_api_ms: 10,
        duration_ms: 20,
        is_error: false,
        message_id: "result-public-activity",
        num_turns: 1,
        result: "研究完成。",
        subtype: "success",
        timestamp: 3,
      },
      stop_reason: "end_turn",
      timestamp: 3,
    }],
    status: "done",
    timestamp: 3,
  });
  assert.doesNotMatch(terminalHtml, /正在浏览/);
  assert.equal(
    terminalHtml.match(/data-room-agent-execution-shell/g)?.length,
    1,
  );
});

test("Room terminal agent status keeps its slot until a message or root takes over", async () => {
  const {
    cancelRunningAgentSlots,
    filterRoundPendingAgentSlots,
    reconcileAgentRoundPendingSlots,
    reconcilePendingSlotsWithAssistantMessage,
  } = await import("@/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts");
  const runningSlot = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-stopped",
    msg_id: "slot-stopped",
    round_id: "round-root",
    status: "streaming",
    timestamp: 10,
  };
  const terminalCases = [
    ["finished", "done"],
    ["interrupted", "cancelled"],
    ["error", "error"],
  ];
  for (const [eventStatus, slotStatus] of terminalCases) {
    assert.deepEqual(
      reconcileAgentRoundPendingSlots(
        [runningSlot],
        "agent-round-stopped",
        eventStatus,
      ),
      [{ ...runningSlot, status: slotStatus }],
      `${eventStatus} must keep the same visible slot as ${slotStatus}`,
    );
  }

  const cancelledSlot = {
    ...runningSlot,
    status: "cancelled",
  };

  assert.deepEqual(
    reconcileAgentRoundPendingSlots(
      [cancelledSlot],
      "agent-round-stopped",
      "running",
    ),
    [cancelledSlot],
    "迟到的 non-terminal 事件不能把已停止槽位改回 streaming",
  );
  const doneSlot = {
    ...runningSlot,
    status: "done",
  };
  assert.deepEqual(
    cancelRunningAgentSlots([doneSlot]),
    [doneSlot],
    "session status settlement must not downgrade a finished slot to cancelled",
  );

  const terminalMessage = assistantMessage({
    agentRoundId: "agent-round-stopped",
    isComplete: true,
    messageId: "assistant-terminal",
    roundId: "round-root",
    status: "done",
    stopReason: "end_turn",
    text: "终态正文",
    timestamp: 11,
  });
  assert.deepEqual(
    reconcilePendingSlotsWithAssistantMessage([cancelledSlot], terminalMessage),
    [],
    "terminal message/result must atomically replace the retained slot",
  );
  assert.deepEqual(
    reconcilePendingSlotsWithAssistantMessage(
      [runningSlot],
      assistantMessage({
        agentRoundId: "agent-round-stopped",
        messageId: "assistant-streaming",
        roundId: "round-root",
        status: "streaming",
        text: "仍在流式输出",
        timestamp: 11,
      }),
    ),
    [runningSlot],
    "streaming assistant still needs the slot's stable index and start time",
  );
  assert.deepEqual(
    filterRoundPendingAgentSlots([cancelledSlot], "round-root"),
    [],
    "root round terminal status remains the final cleanup boundary",
  );
});

test("Room no-reply terminal status closes its published thinking snapshot", async () => {
  const {
    applyTerminalAgentRoundMessageStatus,
  } = await import("@/hooks/agent/runtime/model/conversation-runtime-reconciliation.ts");
  const {
    buildRoomThreadPanelModel,
  } = await import("@/features/conversation/room/group/thread/live/room-thread-panel-model.ts");
  const thinkingSnapshot = {
    agent_id: "agent-lucy",
    agent_round_id: "agent-round-no-reply",
    content: [{ type: "thinking", thinking: "判断是否需要公开回复" }],
    is_complete: false,
    message_id: "assistant-no-reply",
    role: "assistant",
    round_id: "round-root",
    session_key: "room:group:conversation",
    stream_status: "streaming",
    timestamp: 10,
  };
  const unrelatedSnapshot = {
    ...thinkingSnapshot,
    agent_id: "agent-amy",
    agent_round_id: "agent-round-active",
    message_id: "assistant-active",
  };
  const progressSnapshot = {
    ...thinkingSnapshot,
    content: [{ type: "progress_update", text: "正在核对最后一项" }],
    delivery_mode: "ephemeral",
    message_id: "assistant-progress",
  };

  const reconciled = applyTerminalAgentRoundMessageStatus(
    [thinkingSnapshot, progressSnapshot, unrelatedSnapshot],
    "agent-round-no-reply",
    "finished",
  );

  assert.equal(
    reconciled[0]?.stream_status,
    "done",
    "no-reply 没有最终消息时也必须结束已经发布的 thinking 快照",
  );
  assert.equal(
    reconciled[1],
    unrelatedSnapshot,
    "slot 终态只能收口精确匹配的 agent_round_id",
  );
  assert.equal(
    reconciled.some((message) => message.message_id === "assistant-progress"),
    false,
    "slot 终态必须清除同 agent_round 的临时自然语言进度",
  );
  const thread = buildRoomThreadPanelModel({
    agentAvatarMap: {},
    agentNameMap: { "agent-lucy": "Lucy" },
    currentUserAvatar: null,
    messageGroups: new Map([["round-root", reconciled]]),
    onPermissionResponse: () => true,
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map(),
    roomAgentExecutionStateGroups: new Map(),
  }, {
    agentId: "agent-lucy",
    agentRoundId: "agent-round-no-reply",
    roundId: "round-root",
  }, (key) => key);
  assert.equal(
    thread?.isLoading,
    false,
    "Lucy Thread 不应在 no-reply 终态后继续显示正在思考",
  );
});

test("Room queue separates user guidance from private collaboration activity", async () => {
  const {
    projectRoomCollaborationActivity,
    projectRoomPendingInputQueueItems,
  } = await import("@/features/conversation/room/group/chat/panel/controller/group-chat-panel-projection.ts");
  const items = [
    { id: "user", source: "user" },
    { id: "public-mention", source: "agent_public_mention" },
    { id: "directed-message", source: "agent_room_directed_message" },
  ];

  assert.deepEqual(
    projectRoomPendingInputQueueItems(items).map((item) => item.id),
    ["user"],
  );
  assert.equal(projectRoomCollaborationActivity(items, []), "queued");
  assert.equal(projectRoomCollaborationActivity([], [{
    source: "agent_room_directed_message",
    status: "streaming",
  }]), "active");
  assert.equal(projectRoomCollaborationActivity([], [{
    source: "agent_room_directed_message",
    status: "done",
  }]), null);
});

test("Room conversation identity stays stable across physical member sessions", async () => {
  const { getAgentConversationIdentityKey } = await import("@/lib/conversation/agent-conversation-identity.ts");
  const firstMemberSession = {
    chat_type: "group",
    conversation_id: "conversation-shared",
    room_session_id: "room-session-agent-1",
    session_key: "room:group:conversation-shared",
  };
  const secondMemberSession = {
    ...firstMemberSession,
    room_session_id: "room-session-agent-2",
  };

  assert.equal(
    getAgentConversationIdentityKey(firstMemberSession),
    "room-conversation:conversation-shared",
  );
  assert.equal(
    getAgentConversationIdentityKey(secondMemberSession),
    "room-conversation:conversation-shared",
    "a member runtime/session reorder must not reset the shared Room timeline",
  );
  assert.equal(
    getAgentConversationIdentityKey({
      chat_type: "dm",
      conversation_id: "conversation-dm",
      room_session_id: "room-session-dm",
      session_key: "agent:agent-1:workspace:dm:conversation-dm",
    }),
    "room-session:room-session-dm",
    "non-Room conversations keep their physical session boundary",
  );
});

test("Room terminal result keeps public structure, hides thinking, and preserves monotonic text", async () => {
  const { resolveRoomResultFinalAssistantContent } = await import("@/features/conversation/shared/message/item/controller/projection/message-item-final-projection.ts");
  const fallback = [{ type: "text", text: "逐字输出" }];
  const extended = resolveRoomResultFinalAssistantContent({
    fallbackFinalAssistantContent: fallback,
    resultText: "逐字输出完成",
  });

  assert.ok(Array.isArray(extended));
  assert.equal(extended[0]?.type, "text");
  assert.equal(extended[0]?.text, "逐字输出完成");
  assert.equal(
    resolveRoomResultFinalAssistantContent({
      fallbackFinalAssistantContent: fallback,
      resultText: "逐字",
    }),
    fallback,
    "a shorter terminal summary must not shrink already visible Room text",
  );
  assert.deepEqual(
    resolveRoomResultFinalAssistantContent({
      fallbackFinalAssistantContent: null,
      resultText: "仅有终态结果",
    }),
    [{ type: "text", text: "仅有终态结果" }],
  );

  const thinking = { type: "thinking", thinking: "内部过程" };
  const artifact = {
    type: "workspace_file_artifact",
    path: "report.md",
  };
  const corrected = resolveRoomResultFinalAssistantContent({
    fallbackFinalAssistantContent: [
      thinking,
      { type: "text", text: "旧正文" },
      artifact,
    ],
    resultText: "修订后的正文",
  });
  assert.deepEqual(corrected[0], { type: "text", text: "修订后的正文" });
  assert.equal(corrected[1], artifact);
  assert.deepEqual(
    resolveRoomResultFinalAssistantContent({
      fallbackFinalAssistantContent: [thinking, artifact],
      resultText: "附件后的正文",
    }),
    [artifact, { type: "text", text: "附件后的正文" }],
  );
  assert.deepEqual(
    resolveRoomResultFinalAssistantContent({
      fallbackFinalAssistantContent: [{
        type: "text",
        text: "逐字输出 \n",
      }],
      resultText: "逐字输出\n完成",
    }),
    [{ type: "text", text: "逐字输出\n完成" }],
    "terminal suffixes must use the visible text boundary without duplicating whitespace",
  );
});

test("history restores only the latest assistant round failure identity", async () => {
  const {
    DEFAULT_ASSISTANT_ERROR_MESSAGE,
    latestAssistantResultFailure,
    normalizeAssistantMessages,
    resolveAssistantResultErrorMessage,
  } = await import("@/hooks/agent/message/assistant-message-model.ts");
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

  const echoedFailure = assistantMessage({
    isComplete: true,
    messageId: "assistant-echoed-failure",
    resultSummary: {
      duration_api_ms: 10,
      duration_ms: 20,
      is_error: true,
      num_turns: 1,
      result: "provider failed",
      subtype: "error",
      timestamp: 2,
    },
    roundId: "round-echoed-failure",
    status: "done",
    stopReason: "end_turn",
    text: "provider failed",
    timestamp: 2,
  });
  const normalizedEcho = normalizeAssistantMessages([echoedFailure]);
  assert.deepEqual(
    normalizedEcho[0].content,
    [],
    "content-only normalization must not be discarded when stream status is already terminal",
  );

  assert.deepEqual(
    latestAssistantResultFailure([failed]),
    { agent_round_id: null, code: "round_failed", round_id: "round-failed" },
  );
  const runtimeExitMessage =
    "Agent runtime 的响应流意外结束，本轮未完成。会话会在下一条消息自动恢复，请重试。";
  assert.deepEqual(
    latestAssistantResultFailure([assistantMessage({
      messageId: "assistant-runtime-exit",
      resultSummary: {
        duration_api_ms: 0,
        duration_ms: 0,
        is_error: true,
        num_turns: 0,
        result: runtimeExitMessage,
        subtype: "error",
        timestamp: 2,
      },
      roundId: "round-runtime-exit",
      text: "",
      timestamp: 2,
    })]),
    { agent_round_id: null, code: "round_failed", round_id: "round-runtime-exit" },
    "result-only runtime failure must restore identity without exposing provider text",
  );
  assert.deepEqual(
    latestAssistantResultFailure([assistantMessage({
      messageId: "assistant-partial-runtime-exit",
      resultSummary: {
        duration_api_ms: 0,
        duration_ms: 0,
        is_error: true,
        num_turns: 0,
        result: runtimeExitMessage,
        subtype: "error",
        timestamp: 2,
      },
      roundId: "round-partial-runtime-exit",
      text: "已完成一部分输出",
      timestamp: 2,
    })]),
    { agent_round_id: null, code: "round_failed", round_id: "round-partial-runtime-exit" },
    "partial assistant output still needs a structured terminal failure",
  );
  assert.equal(
    latestAssistantResultFailure([
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
  assert.deepEqual(
    latestAssistantResultFailure([
      assistantMessage({
        agentRoundId: "agent-round-failed",
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
    { agent_round_id: "agent-round-failed", code: "round_failed", round_id: "room-round-1" },
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

test("round status updates lifecycle without duplicating durable error copy", async () => {
  const { AGENT_SESSION_EVENT_HANDLERS } = await import("@/hooks/agent/transport/handlers/session-event-handlers.ts");
  const applied = [];
  let failureWrites = 0;
  const context = {
    runtime: {
      applyAgentRoundStatus: (payload) => {
        applied.push(["agent", payload.status]);
      },
      applyRoundStatus: (_roundId, status) => {
        applied.push(["round", status]);
      },
    },
    scope: {
      chatType: "group",
      isCurrentSessionEvent: () => true,
      sessionKey: "room:group:conversation-1",
    },
    state: {
      reliability: {
        observeRecovery: () => {},
        reportFailure: () => {
          failureWrites += 1;
        },
      },
    },
  };

  AGENT_SESSION_EVENT_HANDLERS.agent_round_status({
    data: {
      agent_id: "agent-1",
      agent_round_id: "agent-round-1",
      is_terminal: true,
      round_id: "round-1",
      status: "error",
    },
    event_type: "agent_round_status",
    protocol_version: 2,
    session_key: "room:group:conversation-1",
    timestamp: 1,
  }, context);
  AGENT_SESSION_EVENT_HANDLERS.round_status({
    data: {
      is_terminal: true,
      message: "already projected by durable result",
      result_subtype: "error",
      round_id: "round-1",
      status: "error",
    },
    event_type: "round_status",
    protocol_version: 2,
    session_key: "room:group:conversation-1",
    timestamp: 2,
  }, context);

  assert.deepEqual(applied, [["agent", "error"], ["round", "error"]]);
  assert.equal(failureWrites, 0);
});

test("Room no-reply stays out of the public feed", async () => {
  const { projectGroupAgentTimeline } = await import("@/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts");
  const roundId = "round-root";
  const rootUser = userMessage({
    content: "继续协作",
    messageId: "user-root",
    roundId,
    timestamp: 1,
  });
  const resultSummary = {
    duration_api_ms: 10,
    duration_ms: 100,
    is_error: false,
    num_turns: 1,
    result: "<nexus_room_no_reply/>",
    subtype: "success",
    timestamp: 2,
  };
  const assistant = {
    ...assistantMessage({
      agentId: "agent-lucy",
      agentRoundId: "agent-round-lucy",
      messageId: "assistant-lucy",
      model: "glm-4.7",
      resultSummary,
      roundId,
      status: "done",
      timestamp: 2,
    }),
    content: [{
      thinking: "仅供 Thread 查看",
      type: "thinking",
    }],
  };
  const projection = projectGroupAgentTimeline({
    messageGroups: new Map([[roundId, [rootUser, assistant]]]),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map(),
    roundIds: [roundId],
  });

  assert.deepEqual(projection.roundIds, [roundId]);
  assert.deepEqual(
    projection.messageGroups.get(roundId)?.map((message) => message.message_id),
    [rootUser.message_id],
    "no-reply 是内部控制结果，不应生成公区 Agent 节点或空壳",
  );
});

test("Room interruption projection follows the slot identity without a ghost card", async () => {
  const { buildRoomAgentRoundEntries } = await import("@/features/conversation/room/group/round/round-agent-model.ts");
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

test("canonical timeline hides private Room execution evidence", async () => {
  const {
    buildRoomAgentRoundEntries,
    hasRoomAgentRoundEntries,
  } = await import("@/features/conversation/room/group/round/round-agent-model.ts");
  const { syncRoomAgentExecutionsFromSlots } = await import("@/hooks/agent/runtime/model/room-agent-execution-state.ts");
  const {
    filterVisibleRoomLiveRoundIds,
    filterVisibleTimelineMessages,
    groupMessagesByRound,
    groupPendingPermissionsByRound,
    groupPendingSlotsByRound,
    groupRoomAgentExecutionStatesByRound,
    projectVisibleRoomTimelineEvidence,
  } = await import("@/features/conversation/shared/timeline/timeline-model.ts");
  const privateRoundId = "root-round";
  const privateSlot = {
    agent_id: "agent-witch",
    agent_round_id: "agent-round-witch",
    hidden_from_user: true,
    index: 0,
    msg_id: "slot-witch",
    round_id: privateRoundId,
    status: "streaming",
    timestamp: 1,
  };
  const [privateExecution] = syncRoomAgentExecutionsFromSlots([], [privateSlot]);
  assert.equal(privateExecution.hidden_from_user, true);
  const privatePermission = {
    agent_id: privateSlot.agent_id,
    agent_round_id: privateSlot.agent_round_id,
    request_id: "permission-witch",
    round_id: privateRoundId,
    tool_input: {},
    tool_name: "Read",
  };
  const privateEvidence = projectVisibleRoomTimelineEvidence(
    [privateSlot],
    [privatePermission],
    [privateExecution],
  );

  assert.deepEqual(privateEvidence.pendingSlots, []);
  assert.deepEqual(privateEvidence.pendingPermissions, []);
  assert.deepEqual(privateEvidence.executionStates, []);
  assert.deepEqual(
    filterVisibleRoomLiveRoundIds(
      [privateRoundId],
      groupMessagesByRound([]),
      groupPendingPermissionsByRound(privateEvidence.pendingPermissions),
      groupPendingSlotsByRound(privateEvidence.pendingSlots),
      groupRoomAgentExecutionStatesByRound(privateEvidence.executionStates),
    ),
    [],
    "只有私域证据的物理 round 不应进入公区时间线",
  );
  assert.equal(hasRoomAgentRoundEntries([], [], [], []), false);
  assert.deepEqual(
    buildRoomAgentRoundEntries(
      [],
      privateEvidence.pendingSlots,
      privateEvidence.pendingPermissions,
      privateEvidence.executionStates,
    ),
    [],
  );
  assert.deepEqual(
    filterVisibleTimelineMessages([
      { message_id: "private", hidden_from_user: true },
      { message_id: "public" },
    ]).map((message) => message.message_id),
    ["public"],
  );

  const publicSlot = {
    ...privateSlot,
    hidden_from_user: false,
  };
  const publicEvidence = projectVisibleRoomTimelineEvidence(
    [publicSlot],
    [],
    syncRoomAgentExecutionsFromSlots([], [publicSlot]),
  );
  assert.equal(
    buildRoomAgentRoundEntries(
      [],
      publicEvidence.pendingSlots,
      publicEvidence.pendingPermissions,
      publicEvidence.executionStates,
    ).length,
    1,
  );
});

test("Room terminal execution rejects stale active evidence and late interaction", async () => {
  const { buildGroupRoundCardModel } = await import("@/features/conversation/room/group/thread/round-card/group-round-card-model.ts");
  const { projectGroupAgentTimeline } = await import("@/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts");
  const {
    filterPendingPermissionsForTerminalRoomExecutions,
  } = await import("@/lib/conversation/pending-permission-match.ts");
  const { AgentConversationRuntimeMachine } = await import("@/hooks/agent/runtime/model/agent-conversation-runtime-machine.ts");
  const roundId = "round-terminal-monotonic";
  const agentRoundId = "agent-round-terminal-monotonic";
  const staleMessage = assistantMessage({
    agentId: "agent-terminal",
    agentRoundId,
    messageId: "assistant-terminal-stale-stream",
    roundId,
    status: "streaming",
    text: "迟到的流式快照",
    timestamp: 4,
  });
  const staleSlot = {
    agent_id: "agent-terminal",
    agent_round_id: agentRoundId,
    index: 0,
    msg_id: "slot-terminal-stale",
    round_id: roundId,
    status: "streaming",
    timestamp: 2,
  };
  const terminalState = {
    agent_id: "agent-terminal",
    agent_round_id: agentRoundId,
    display_order: 0,
    first_seen_at: 1,
    phase: "terminal",
    round_id: roundId,
    status: "error",
  };
  const lateQuestion = {
    agent_id: "agent-terminal",
    agent_round_id: agentRoundId,
    interaction_mode: "question",
    request_id: "question-after-terminal",
    round_id: roundId,
    tool_input: {
      questions: [{
        options: [{ label: "重试" }],
        question: "是否继续？",
      }],
    },
    tool_name: "AskUserQuestion",
  };
  const model = buildGroupRoundCardModel({
    executionStates: [terminalState],
    messages: [staleMessage],
    pendingPermissions: [lateQuestion],
    pendingSlots: [staleSlot],
  });

  assert.equal(
    model.entries[0]?.status,
    "error",
    "an authoritative terminal error must bound stale slot/message activity",
  );
  assert.deepEqual(
    model.entries[0]?.pendingPermissions,
    [],
    "a late exact permission cannot revive an interactive terminal shell",
  );

  const projection = projectGroupAgentTimeline({
    messageGroups: new Map([[roundId, [staleMessage]]]),
    pendingPermissionGroups: new Map([[roundId, [lateQuestion]]]),
    pendingSlotGroups: new Map([[roundId, [staleSlot]]]),
    roomAgentExecutionStateGroups: new Map([[roundId, [terminalState]]]),
    roundIds: [roundId],
  });
  assert.deepEqual(
    Array.from(projection.pendingPermissionGroups.values()).flat(),
    [],
    "the filtered permission must not fall back to a generic root card",
  );

  const rawPermissions =
    filterPendingPermissionsForTerminalRoomExecutions(
      [lateQuestion],
      [terminalState],
    );
  assert.deepEqual(
    rawPermissions,
    [],
    "the volatile source must reject the request before it contributes to runtime count",
  );
  const runtime = new AgentConversationRuntimeMachine("group");
  runtime.trackRoundStatus(roundId, "running");
  runtime.setPendingPermissionCount(rawPermissions.length);
  assert.notEqual(
    runtime.snapshot().phase,
    "awaiting_permission",
    "an invisible late request must not lock the composer",
  );

  const legacyQuestion = {
    ...lateQuestion,
    agent_round_id: null,
    request_id: "legacy-question-after-terminal",
  };
  assert.deepEqual(
    filterPendingPermissionsForTerminalRoomExecutions(
      [legacyQuestion],
      [terminalState],
    ),
    [legacyQuestion],
    "legacy interaction without an exact execution identity cannot be guessed away",
  );
});

test("Room exact stop survives slot cleanup and settles ACK/terminal races per Agent", async () => {
  const { buildGroupRoundCardModel } = await import("@/features/conversation/room/group/thread/round-card/group-round-card-model.ts");
  const { confirmRoomAgentExecutionStop } = await import("@/hooks/agent/runtime/model/room-agent-execution-state.ts");
  const {
    addStoppingAgentRoundId,
    removeStoppingAgentRoundId,
  } = await import("@/hooks/agent/runtime/state/use-conversation-volatile-state.ts");
  const { parseInterruptAckData } = await import("@/hooks/agent/transport/handlers/session-event-data.ts");
  const roundId = "round-stop-race";
  const stateA = {
    agent_id: "agent-a",
    agent_round_id: "agent-round-a",
    display_order: 1,
    first_seen_at: 1,
    phase: "active",
    round_id: roundId,
    status: "streaming",
  };
  const stateB = {
    ...stateA,
    agent_id: "agent-b",
    agent_round_id: "agent-round-b",
    display_order: 2,
  };
  const completedTurn = assistantMessage({
    agentId: stateA.agent_id,
    agentRoundId: stateA.agent_round_id,
    isComplete: true,
    messageId: "assistant-a-tool-boundary",
    roundId,
    status: "done",
    stopReason: "tool_use",
    text: "先完成一段输出",
    timestamp: 2,
  });
  const model = buildGroupRoundCardModel({
    executionStates: [stateA, stateB],
    messages: [completedTurn],
    pendingPermissions: [],
    pendingSlots: [],
  });
  assert.equal(
    model.entries.find((entry) => entry.agent_id === stateA.agent_id)
      ?.stopAgentRoundId,
    stateA.agent_round_id,
    "the exact stop target must come from execution identity after pending slot cleanup",
  );

  let stopping = addStoppingAgentRoundId([], stateA.agent_round_id);
  assert.strictEqual(
    addStoppingAgentRoundId(stopping, stateA.agent_round_id),
    stopping,
    "a double click must not register the same exact target twice",
  );
  stopping = addStoppingAgentRoundId(stopping, stateB.agent_round_id);
  stopping = removeStoppingAgentRoundId(stopping, stateA.agent_round_id);
  const terminalBeforeAck = removeStoppingAgentRoundId(
    stopping,
    stateA.agent_round_id,
  );
  assert.deepEqual(
    terminalBeforeAck,
    [stateB.agent_round_id],
    "terminal-before-ACK settlement must be idempotent and preserve Agent B",
  );

  const stoppedStates = confirmRoomAgentExecutionStop(
    [stateA, stateB],
    stateA.agent_round_id,
  );
  assert.equal(stoppedStates[0].phase, "terminal");
  assert.equal(stoppedStates[0].status, "cancelled");
  assert.strictEqual(stoppedStates[1], stateB);
  assert.strictEqual(
    confirmRoomAgentExecutionStop(stoppedStates, stateA.agent_round_id),
    stoppedStates,
    "ACK-before-terminal and terminal-before-ACK must converge idempotently",
  );
  assert.deepEqual(
    parseInterruptAckData({
      accepted: true,
      ack_timeout_ms: 10_000,
      agent_round_id: stateA.agent_round_id,
      client_request_id: "request-stop-a",
      round_id: roundId,
    }),
    {
      accepted: true,
      ack_timeout_ms: 10_000,
      agent_round_id: stateA.agent_round_id,
      client_request_id: "request-stop-a",
      round_id: roundId,
    },
  );
});

