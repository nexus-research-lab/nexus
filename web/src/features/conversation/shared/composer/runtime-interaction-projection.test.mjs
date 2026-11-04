// @vitest-environment node

import { assistantMessage } from "@/features/conversation/testing/message-fixtures.mjs";

import assert from "node:assert/strict";
import { test } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

test("pending interactions keep first position and latest request snapshot", async () => {
  const { coalescePendingPermissions } = await import("@/lib/conversation/pending-permission-match.ts");
  const { resolveMessageItemPermissions } = await import("@/features/conversation/shared/message/item/controller/projection/message-item-permissions.ts");
  const { resolvePendingInteractionOwner } = await import("@/features/conversation/shared/message/item/message-item-projection.ts");
  const first = {
    message_id: "assistant-permission-owner",
    request_id: "request-stable",
    summary: "旧快照",
    tool_input: { command: "echo stable" },
    tool_name: "Bash",
    tool_use_id: "tool-stable",
  };
  const other = {
    request_id: "request-other",
    tool_input: { path: "/tmp/report" },
    tool_name: "Read",
  };
  const latest = {
    ...first,
    summary: "最新快照",
  };
  assert.deepEqual(
    coalescePendingPermissions([first, other, latest]),
    [latest, other],
    "a repeated request updates in place instead of creating a second surface",
  );

  const assistant = {
    agent_id: "agent-1",
    content: [{
      id: "tool-stable",
      input: { command: "echo stable" },
      name: "Bash",
      type: "tool_use",
    }],
    message_id: "assistant-permission-owner",
    role: "assistant",
    round_id: "round-permission-owner",
    session_key: "room:group:conversation-1",
    stream_status: "streaming",
    timestamp: 1,
  };
  const projection = resolveMessageItemPermissions(
    [assistant],
    [first, other, latest],
  );
  assert.deepEqual(projection.pendingInteractionPermissions, [latest, other]);
  assert.equal(
    projection.matchedPendingPermissionsByToolUseId.get("tool-stable"),
    latest,
  );
  assert.deepEqual(projection.unmatchedPendingPermissions, [other]);
  assert.equal(resolvePendingInteractionOwner("room_result"), "composer");
  assert.equal(resolvePendingInteractionOwner("room_thread"), "composer");
  assert.equal(
    resolvePendingInteractionOwner("room_thread_process"),
    "composer",
  );
  assert.equal(resolvePendingInteractionOwner("dm_live"), "composer");
  assert.equal(resolvePendingInteractionOwner("dm_archived"), "composer");
});

test("Room keeps every pending runtime human interaction in the Composer", async () => {
  const { ComposerInteractionSurface } = await import("@/features/conversation/shared/composer/components/interaction/composer-interaction-surface.tsx");
  const { GroupAgentExecutionShell } = await import("@/features/conversation/room/group/thread/round-card/group-agent-execution-shell.tsx");
  const { GroupConversationRound } = await import("@/features/conversation/room/group/chat/feed/group-conversation-round.tsx");
  const { resolveGroupConversationRound } = await import("@/features/conversation/room/group/chat/feed/group-conversation-feed-model.ts");
  const { projectGroupAgentTimeline } = await import("@/features/conversation/room/group/chat/feed/group-agent-timeline-model.ts");
  const { ThreadControlContext } = await import("@/features/conversation/room/group/thread/group-thread-state.ts");
  const { I18nProvider } = await import("@/shared/i18n/i18n-provider.tsx");
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
  const questionPermission = {
    agent_id: "agent-1",
    agent_round_id: "agent-round-1",
    interaction_mode: "question",
    request_id: "question-1",
    round_id: "round-root",
    summary: "请选择研究口径",
    tool_input: {
      questions: [{
        header: "研究口径",
        multiSelect: false,
        options: [
          { label: "保守", description: "优先采用可验证数据" },
          { label: "积极", description: "纳入前瞻性假设" },
        ],
        question: "这次分析采用哪种研究口径？",
      }],
    },
    tool_name: "AskUserQuestion",
    tool_use_id: "tool-question",
  };
  const planConfirmation = {
    ...permission,
    request_id: "plan-confirmation-1",
    summary: "确认按这份计划继续执行",
    tool_input: {
      plan: "先验证数据源，再生成最终报告。",
    },
    tool_name: "ExitPlanMode",
    tool_use_id: "tool-plan-confirmation",
  };
  const futureApproval = {
    ...permission,
    interaction_mode: "future_review",
    request_id: "future-approval-1",
    summary: "确认发布研究结果",
    tool_input: {
      description: "将报告发布到共享工作区。",
    },
    tool_name: "RequestHumanReview",
    tool_use_id: "tool-future-approval",
  };
  const provider = (child) => React.createElement(
    I18nProvider,
    null,
    React.createElement(
      ThreadControlContext.Provider,
      {
        value: {
          activeThread: null,
          closeThread: () => {},
          openThread: () => {},
        },
      },
      child,
    ),
  );

  const composerHtml = renderToStaticMarkup(provider(React.createElement(
    ComposerInteractionSurface,
    {
      agentAvatarMap: { "agent-1": null },
      agentNameMap: { "agent-1": "Dev" },
      onResponse: () => true,
      permissions: [
        permission,
        questionPermission,
        planConfirmation,
        futureApproval,
      ],
    },
  )));
  assert.match(composerHtml, /data-composer-interaction-surface="true"/);
  assert.match(composerHtml, /Dev/);
  assert.match(composerHtml, /echo permission-required/);
  assert.match(composerHtml, /1 \/ 4/);
  assert.match(composerHtml, />允许本次</);
  assert.match(composerHtml, />拒绝</);
  assert.doesNotMatch(
    composerHtml,
    /这次分析采用哪种研究口径？/,
    "Composer must show only the first request in the stable queue",
  );
  const nextComposerHtml = renderToStaticMarkup(provider(React.createElement(
    ComposerInteractionSurface,
    {
      onResponse: () => true,
      permissions: [
        questionPermission,
        planConfirmation,
        futureApproval,
      ],
    },
  )));
  assert.match(nextComposerHtml, /这次分析采用哪种研究口径？/);
  assert.match(nextComposerHtml, /1 \/ 3/);
  assert.match(nextComposerHtml, /继续协作/);

  const agentCardHtml = renderToStaticMarkup(provider(React.createElement(
    GroupAgentExecutionShell,
    {
      agentAvatar: null,
      agentId: "agent-1",
      agentName: "Dev",
      isThreadActive: false,
      messages: [],
      onClickThread: () => {},
      onPermissionResponse: () => true,
      pendingPermissions: [
        permission,
        questionPermission,
        planConfirmation,
        futureApproval,
      ],
      roundId: "round-root:agent-1:agent-round-1",
      status: "pending",
      timestamp: 1,
    },
  )));
  assert.doesNotMatch(agentCardHtml, /data-human-interaction-surface/);
  assert.doesNotMatch(agentCardHtml, />允许</);
  assert.doesNotMatch(agentCardHtml, />拒绝</);
  assert.doesNotMatch(agentCardHtml, /继续协作/);
  const adjacentAgentHtml = renderToStaticMarkup(provider(React.createElement(
    GroupAgentExecutionShell,
    {
      agentAvatar: null,
      agentId: "agent-2",
      agentName: "Review",
      isThreadActive: false,
      messages: [],
      onClickThread: () => {},
      onPermissionResponse: () => true,
      pendingPermissions: [],
      roundId: "round-root:agent-2:agent-round-2",
      showAgentBoundary: true,
      status: "pending",
      timestamp: 2,
    },
  )));
  assert.match(
    adjacentAgentHtml,
    /data-conversation-agent-boundary/,
    "相邻 Agent 只用局部身份提示建立边界",
  );
  assert.doesNotMatch(
    adjacentAgentHtml,
    /conversation-round-divider/,
    "Room 主 Feed 的 Agent 边界不能再伪装成 Markdown 全宽分隔线",
  );

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
        stoppingAgentRoundIds: [],
        runtimePhase: null,
      },
      state: {
        index: 0,
        isLast: true,
        isLive: true,
        isLoaded: true,
        messages: [],
        pendingPermissions: [
          permission,
          questionPermission,
          planConfirmation,
          futureApproval,
        ],
        pendingSlots: [],
        roomAgentExecutionStates: [],
        rootRoundId: "round-root",
        roundId: "round-root",
      },
    }),
  ));
  assert.doesNotMatch(permissionOnlyRoundHtml, /data-human-interaction-surface/);
  assert.doesNotMatch(permissionOnlyRoundHtml, />允许</);
  assert.doesNotMatch(permissionOnlyRoundHtml, />拒绝</);
  assert.doesNotMatch(permissionOnlyRoundHtml, /继续协作/);

  const completedToolMessage = {
    ...assistantMessage({
      agentId: "agent-1",
      agentRoundId: "agent-round-1",
      isComplete: true,
      messageId: "assistant-tool-call",
      model: "glm-5.2",
      roundId: "round-root",
      status: "done",
      stopReason: "tool_use",
      text: "Goal 已设定，现在开始调研。",
      timestamp: 2,
    }),
    content: [
      { type: "text", text: "Goal 已设定，现在开始调研。" },
      {
        type: "tool_use",
        id: "tool-search",
        input: { query: "Apple M3 vs M4 vs M5 chip comparison specifications" },
        name: "WebSearch",
      },
      {
        type: "tool_use",
        id: "tool-question",
        input: questionPermission.tool_input,
        name: "AskUserQuestion",
      },
    ],
  };
  const completedPermission = {
    ...permission,
    message_id: "assistant-tool-call",
    request_id: "permission-search",
    summary: "Apple M3 vs M4 vs M5 chip comparison specifications",
    tool_input: {
      query: "Apple M3 vs M4 vs M5 chip comparison specifications",
    },
    tool_name: "WebSearch",
    tool_use_id: "tool-search",
  };
  const completedProjection = projectGroupAgentTimeline({
    messageGroups: new Map([["round-root", [completedToolMessage]]]),
    pendingPermissionGroups: new Map([
      ["round-root", [completedPermission, questionPermission]],
    ]),
    pendingSlotGroups: new Map(),
    roundIds: ["round-root"],
  });
  const completedState = resolveGroupConversationRound({
    liveRoundIds: ["round-root"],
    messageGroups: completedProjection.messageGroups,
    pendingPermissionGroups: completedProjection.pendingPermissionGroups,
    pendingSlotGroups: completedProjection.pendingSlotGroups,
    roomAgentExecutionStateGroups:
      completedProjection.roomAgentExecutionStateGroups,
    rootRoundIds: completedProjection.rootRoundIds,
    roundIds: completedProjection.roundIds,
  }, 0);
  const completedRoundHtml = renderToStaticMarkup(provider(
    React.createElement(GroupConversationRound, {
      renderer: {
        agentAvatarMap: {},
        agentNameMap: { "agent-1": "Kevin" },
        currentAgentAvatar: null,
        currentAgentName: "Kevin",
        currentUserAvatar: null,
        isLastRoundPendingPermissions: [
          completedPermission,
          questionPermission,
        ],
        onPermissionResponse: () => true,
        onStopAgentRound: () => {},
        stoppingAgentRoundIds: [],
        runtimePhase: null,
      },
      state: completedState,
    }),
  ));
  assert.match(
    completedRoundHtml,
    /Goal[\s\S]*已设定，现在开始调研/,
    "the Room timeline keeps its public reply while Composer owns approval",
  );
  assert.doesNotMatch(completedRoundHtml, />允许</);
  assert.doesNotMatch(completedRoundHtml, />拒绝</);
  assert.doesNotMatch(completedRoundHtml, /继续协作/);
  assert.doesNotMatch(completedRoundHtml, /data-human-interaction-surface/);

  const questionOnlyMessage = {
    ...completedToolMessage,
    message_id: "assistant-question-only",
    content: [{
      type: "tool_use",
      id: "tool-question",
      input: questionPermission.tool_input,
      name: "AskUserQuestion",
    }],
  };
  const questionOnlyPermission = {
    ...questionPermission,
    message_id: "assistant-question-only",
  };
  const questionOnlyProjection = projectGroupAgentTimeline({
    messageGroups: new Map([["round-root", [questionOnlyMessage]]]),
    pendingPermissionGroups: new Map([
      ["round-root", [questionOnlyPermission]],
    ]),
    pendingSlotGroups: new Map(),
    roundIds: ["round-root"],
  });
  const questionOnlyState = resolveGroupConversationRound({
    liveRoundIds: ["round-root"],
    messageGroups: questionOnlyProjection.messageGroups,
    pendingPermissionGroups: questionOnlyProjection.pendingPermissionGroups,
    pendingSlotGroups: questionOnlyProjection.pendingSlotGroups,
    roomAgentExecutionStateGroups:
      questionOnlyProjection.roomAgentExecutionStateGroups,
    rootRoundIds: questionOnlyProjection.rootRoundIds,
    roundIds: questionOnlyProjection.roundIds,
  }, 0);
  const questionOnlyHtml = renderToStaticMarkup(provider(
    React.createElement(GroupConversationRound, {
      renderer: {
        agentAvatarMap: {},
        agentNameMap: { "agent-1": "Kevin" },
        currentAgentAvatar: null,
        currentAgentName: "Kevin",
        currentUserAvatar: null,
        isLastRoundPendingPermissions: [questionOnlyPermission],
        onPermissionResponse: () => true,
        onStopAgentRound: () => {},
        stoppingAgentRoundIds: [],
        runtimePhase: null,
      },
      state: questionOnlyState,
    }),
  ));
  assert.doesNotMatch(questionOnlyHtml, /继续协作/);
  assert.doesNotMatch(questionOnlyHtml, />允许</);
  assert.doesNotMatch(questionOnlyHtml, />拒绝</);
  assert.doesNotMatch(questionOnlyHtml, /data-human-interaction-surface/);
});

test("Room Composer stop-all freezes exact active multi-Agent targets at click time", async () => {
  const {
    collectActiveRoomAgentRoundIds,
    stopRoomAgentOutputs,
  } = await import("@/features/conversation/room/group/chat/panel/controller/use-group-chat-composer-model.ts");
  const conversation = {
    room_agent_execution_states: [
      { agent_round_id: "round-agent-a", phase: "active" },
      { agent_round_id: "round-agent-b", phase: "pending_permission" },
      { agent_round_id: "round-agent-finished", phase: "terminal" },
    ],
    pending_agent_slots: [
      { agent_round_id: "round-agent-a", status: "streaming" },
      { agent_round_id: "round-agent-c", status: "pending" },
      { agent_round_id: "round-agent-finished", status: "completed" },
    ],
    stopping_agent_round_ids: ["round-agent-b"],
  };
  const targets = collectActiveRoomAgentRoundIds(conversation);

  assert.deepEqual(
    targets,
    ["round-agent-a", "round-agent-c"],
    "terminal, duplicate, and already-stopping rounds must not enter the batch",
  );

  const stopped = [];
  stopRoomAgentOutputs(targets, (agentRoundId) => {
    stopped.push(agentRoundId);
    if (agentRoundId === "round-agent-a") {
      targets.push("round-agent-late");
    }
  });
  assert.deepEqual(
    stopped,
    ["round-agent-a", "round-agent-c"],
    "the first synchronous stop response must not mutate the click-time batch",
  );
});
