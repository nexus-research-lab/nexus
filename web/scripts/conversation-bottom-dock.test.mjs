import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
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

async function renderWithI18n(element) {
  const { I18N_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-context.ts",
  );
  return renderToStaticMarkup(
    React.createElement(
      I18N_CONTEXT.Provider,
      {
        value: {
          locale: "zh",
          setLocale: () => {},
          t: (key) => key,
        },
      },
      element,
    ),
  );
}

test("回到底部入口隐藏时零标记，显示时只有局部热区且没有原生 tooltip", async () => {
  const { ScrollToLatestButton } = await server.ssrLoadModule(
    "/src/features/conversation/shared/scroll-to-latest-button.tsx",
  );
  const hidden = renderToStaticMarkup(
    React.createElement(ScrollToLatestButton, {
      isLoading: false,
      onClick: () => {},
      visible: false,
    }),
  );
  const visible = renderToStaticMarkup(
    React.createElement(ScrollToLatestButton, {
      isLoading: false,
      onClick: () => {},
      visible: true,
    }),
  );

  assert.equal(hidden, "");
  assert.match(visible, /data-scroll-to-latest="true"/);
  assert.match(visible, /\bh-11\b/);
  assert.match(visible, /\bw-11\b/);
  assert.doesNotMatch(visible, /\stitle=/);
  assert.doesNotMatch(visible, /\bh-10 shrink-0\b/);
});

test("消息尾部只为真实可见的浮动 Dock 保留避让", async () => {
  const { ConversationPanelViewport } = await server.ssrLoadModule(
    "/src/features/conversation/shared/conversation-panel-layout.tsx",
  );
  const viewport = {
    error: null,
    isHistoryLoading: false,
    onPointerDown: () => {},
    onScroll: () => {},
    onTouchEnd: () => {},
    onTouchMove: () => {},
    onTouchStart: () => {},
    onWheel: () => {},
    scrollRef: { current: null },
  };
  const hidden = renderToStaticMarkup(
    React.createElement(
      ConversationPanelViewport,
      { floatingDockOccupied: false, isMobileLayout: false, viewport },
      React.createElement("div", null, "message"),
    ),
  );
  const occupied = renderToStaticMarkup(
    React.createElement(
      ConversationPanelViewport,
      { floatingDockOccupied: true, isMobileLayout: false, viewport },
      React.createElement("div", null, "message"),
    ),
  );

  assert.doesNotMatch(hidden, /data-conversation-dock-clearance/);
  assert.match(occupied, /data-conversation-dock-clearance/);
  assert.match(occupied, /\bh-14\b/);
});

test("Composer Footer keeps Powered by Nexus in its physical center column", async () => {
  const { ComposerFooter } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/footer/composer-footer.tsx",
  );
  const html = await renderWithI18n(
    React.createElement(ComposerFooter, {
      actionButtonRef: { current: null },
      activeError: null,
      canCreateGoal: true,
      canUseLoop: false,
      charCount: 0,
      goalModeExtra: null,
      goalScopeLabel: "会话 Goal",
      historyIndex: -1,
      inputHistoryLength: 0,
      isActionMenuOpen: false,
      isGoalCreating: false,
      isGoalMode: false,
      isNearLimit: false,
      isOverLimit: false,
      isPreparingAttachments: false,
      maxLength: 10_000,
      onActionMenuClose: () => {},
      onActionMenuToggle: () => {},
      onAttachmentSelect: () => {},
      onCancelGoal: () => {},
      onGoalToggle: () => {},
      onLoopSelect: () => {},
      runtimeActivity: null,
      showPoweredByNexus: true,
      submit: {
        enterLabel: "发送",
        isDisabled: true,
        isGoalCreating: false,
        isGoalMode: false,
        isPreparingAttachments: false,
        onSend: () => {},
        sendLabel: "发送",
        shouldStop: false,
        stopLabel: "停止",
      },
    }),
  );

  assert.match(html, /\bnexus-chat-composer-footer\b/);
  assert.match(html, /data-composer-powered-by="true"/);
  assert.match(html, /Powered by\s*<\/span>Nexus/);
  const recipeSource = await readFile(
    path.join(webRoot, "src/app/styles/theme-recipes.css"),
    "utf8",
  );
  assert.match(
    recipeSource,
    /grid-template-columns: minmax\(0, 1fr\) auto minmax\(0, 1fr\)/,
  );
  assert.match(
    recipeSource,
    /@container nexus-chat-composer \(max-width: 420px\)/,
  );
  assert.match(
    recipeSource,
    /var\(--text-soft\) 52%[\s\S]*var\(--input-shell-background\)/,
    "the centered brand stays below normal secondary-text contrast",
  );
});

test("Workspace Task uses a centered step-summary capsule and an absolute upward detail", async () => {
  const { WorkspaceTaskPanel } = await server.ssrLoadModule(
    "/src/shared/ui/workspace/surface/workspace-task-strip.tsx",
  );
  const { resolveWorkspaceTaskSummary } = await server.ssrLoadModule(
    "/src/shared/ui/workspace/surface/workspace-task-strip-model.ts",
  );
  const todos = [
    {
      content: "读取现有布局",
      status: "completed",
    },
    {
      active_form: "正在核对布局",
      content: "核对布局",
      status: "in_progress",
    },
    {
      content: "验证多尺寸",
      status: "pending",
    },
  ];
  assert.deepEqual(resolveWorkspaceTaskSummary(todos), {
    completedCount: 1,
    currentStep: 2,
    hasRunningTask: true,
    summary: "正在核对布局",
    totalCount: 3,
  });
  const html = await renderWithI18n(
    React.createElement(WorkspaceTaskPanel, {
      todos,
    }),
  );

  assert.match(html, /data-workspace-task-panel="true"/);
  assert.match(html, /data-workspace-task-trigger="true"/);
  assert.match(html, /data-workspace-task-summary="正在核对布局"/);
  assert.match(html, /\bh-11\b/);
  assert.match(html, /\brounded-full\b/);
  assert.match(html, /tasks\.step_progress/);
  assert.match(html, /正在核对布局/);
  assert.match(html, /aria-controls="[^"]+"/);
  assert.match(html, /aria-expanded="false"/);

  const taskSource = await readFile(
    path.join(
      webRoot,
      "src/shared/ui/workspace/surface/workspace-task-strip.tsx",
    ),
    "utf8",
  );
  assert.match(taskSource, /bottom-\[calc\(100%\+0\.5rem\)\]/);
  assert.match(taskSource, /left-1\/2 -translate-x-1\/2/);
  assert.match(taskSource, /data-placement="top"/);
  assert.match(taskSource, /<span className="sr-only">\{taskStatusLabel\(todo\.status\)\}<\/span>/);
  assert.ok(
    taskSource.indexOf("data-workspace-task-trigger")
      < taskSource.indexOf("data-placement=\"top\""),
    "the trigger must precede its detail panel in DOM and tab order",
  );
});

test("Room and DM compose Task inside the shared bottom area instead of the Surface top", async () => {
  const sources = await Promise.all([
    "src/features/conversation/room/surface/layout/room-surface-content.tsx",
    "src/features/conversation/room/surface/mobile/room-mobile-surface.tsx",
    "src/features/conversation/room/dm/panel/view/dm-chat-panel-view.tsx",
    "src/features/conversation/room/group/chat/panel/view/group-chat-panel-view.tsx",
  ].map((file) => readFile(path.join(webRoot, file), "utf8")));
  const [desktopSurface, mobileSurface, dmView, groupView] = sources;

  assert.doesNotMatch(desktopSurface, /<WorkspaceTaskPanel/);
  assert.doesNotMatch(mobileSurface, /<WorkspaceTaskPanel/);
  assert.match(dmView, /<ConversationPanelBottomArea/);
  assert.match(groupView, /<ConversationPanelBottomArea/);
  assert.match(dmView, /<ConversationPanelViewportArea/);
  assert.match(groupView, /<ConversationPanelViewportArea/);
  assert.doesNotMatch(dmView, /bottom-\[156px\]/);
  assert.doesNotMatch(groupView, /bottom-\[156px\]/);
  assert.match(dmView, /model\.todos\.length > 0[\s\S]*<WorkspaceTaskPanel todos=\{model\.todos\} \/>/);
  assert.match(groupView, /model\.todos\.length > 0[\s\S]*<WorkspaceTaskPanel todos=\{model\.todos\} \/>/);

  const layoutSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/shared/conversation-panel-layout.tsx",
    ),
    "utf8",
  );
  assert.ok(
    layoutSource.indexOf("data-conversation-pre-composer")
      < layoutSource.indexOf("data-conversation-composer-anchor"),
    "Goal/provider content must stay above the Composer anchor",
  );
  const composerAnchorStart = layoutSource.indexOf(
    "data-conversation-composer-anchor",
  );
  assert.ok(
    layoutSource.indexOf("<ConversationPanelFloatingControls", composerAnchorStart)
      > composerAnchorStart,
    "Task and scroll controls must be anchored by the Composer itself",
  );
});

test("Task and scroll controls share a centered dock while retaining local pointer hit areas", async () => {
  const { ConversationPanelFloatingControls } = await server.ssrLoadModule(
    "/src/features/conversation/shared/conversation-panel-layout.tsx",
  );
  const html = renderToStaticMarkup(
    React.createElement(ConversationPanelFloatingControls, {
      activity: React.createElement(
        "button",
        { "data-test-task-control": true },
        "task",
      ),
      isMobileLayout: false,
      scrollToLatest: {
        isLoading: false,
        onClick: () => {},
        visible: true,
      },
    }),
  );

  assert.match(html, /data-conversation-activity-dock="true"/);
  assert.match(html, /data-conversation-dock-activity="true"/);
  assert.match(html, /data-conversation-dock-scroll="true"/);
  assert.match(html, /\bjustify-center\b/);
  assert.match(html, /\bgap-1\b/);
  assert.match(html, /data-test-task-control="true"/);
  assert.match(html, /data-scroll-to-latest="true"/);

  const layoutSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/shared/conversation-panel-layout.tsx",
    ),
    "utf8",
  );
  assert.match(
    layoutSource,
    /data-conversation-activity-dock[\s\S]*data-conversation-dock-activity[\s\S]*data-conversation-dock-scroll/,
  );
  assert.doesNotMatch(
    layoutSource,
    /data-conversation-activity-dock[\s\S]*grid-cols-\[minmax\(0,1fr\)_auto_minmax\(0,1fr\)\]/,
  );
});

test("DM pending interactions replace the Composer input in one stable queue", async () => {
  const { buildComposerInteractionQueue } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/interaction/composer-interaction-model.ts",
  );
  const { ComposerInteractionSurface } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/interaction/composer-interaction-surface.tsx",
  );
  const { ComposerPanel } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-panel.tsx",
  );
  const {
    removePendingPermission,
    upsertPendingPermission,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/permission/pending-permission-state.ts",
  );
  const first = {
    request_id: "request-first",
    summary: "旧快照",
    tool_input: { command: "echo old" },
    tool_name: "Bash",
  };
  const second = {
    request_id: "request-second",
    summary: "第二项",
    tool_input: { file_path: "/tmp/second" },
    tool_name: "Read",
  };
  const latest = {
    ...first,
    summary: "最新快照",
    tool_input: { command: "echo updated" },
  };

  let pending = upsertPendingPermission([], first);
  pending = upsertPendingPermission(pending, second);
  pending = upsertPendingPermission(pending, latest);
  assert.deepEqual(pending, [latest, second]);
  assert.deepEqual(removePendingPermission(pending, first.request_id), [second]);

  const queue = buildComposerInteractionQueue(pending);
  assert.equal(queue.current?.request_id, first.request_id);
  assert.equal(queue.kind, "permission");
  assert.equal(queue.total, 2);

  const interaction = React.createElement(ComposerInteractionSurface, {
    onResponse: () => true,
    permissions: pending,
    workspaceAgentId: "agent-1",
  });
  const interactionHtml = await renderWithI18n(interaction);
  assert.match(interactionHtml, /data-composer-interaction-surface="true"/);
  assert.match(
    interactionHtml,
    /data-pending-interaction-request-id="request-first"/,
  );
  assert.match(interactionHtml, /1 \/ 2/);
  assert.match(interactionHtml, /echo updated/);
  assert.doesNotMatch(interactionHtml, /\/tmp\/second/);
  assert.match(interactionHtml, />允许</);
  assert.match(interactionHtml, />拒绝</);

  const composerProps = {
    compact: false,
    defaultDeliveryPolicy: "queue",
    draftScopeKey: "dm:agent-1:session-1",
    goalScopeLabel: "会话 Goal",
    historyScopeKey: "dm:agent-1",
    inputQueueItems: [],
    interactionIdentity: first.request_id,
    interactionSurface: interaction,
    isLoading: true,
    onDeleteQueuedMessage: () => {},
    onEnqueueMessage: () => {},
    onGuideQueuedMessage: () => {},
    onPrepareAttachments: async () => [],
    onReorderQueueMessages: () => {},
    onSendMessage: () => {},
    runtimeKind: "nxs",
    runtimePhase: "awaiting_permission",
    tourAnchor: "composer",
  };
  const replacedHtml = await renderWithI18n(
    React.createElement(ComposerPanel, composerProps),
  );
  assert.match(replacedHtml, /data-composer-surface="interaction"/);
  assert.match(replacedHtml, /data-composer-interaction-surface="true"/);
  assert.doesNotMatch(replacedHtml, /<textarea/);
  assert.doesNotMatch(replacedHtml, /data-composer-powered-by/);

  const inputHtml = await renderWithI18n(
    React.createElement(ComposerPanel, {
      ...composerProps,
      interactionIdentity: null,
      interactionSurface: undefined,
      isLoading: false,
      runtimePhase: "idle",
    }),
  );
  assert.match(inputHtml, /data-composer-surface="input"/);
  assert.match(inputHtml, /<textarea/);
  assert.match(inputHtml, /data-composer-powered-by="true"/);
});

test("DM questions and plan confirmations use the same Composer replacement owner", async () => {
  const { buildComposerInteractionQueue } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/interaction/composer-interaction-model.ts",
  );
  const { ComposerInteractionSurface } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/interaction/composer-interaction-surface.tsx",
  );
  const question = {
    interaction_mode: "question",
    request_id: "request-question",
    tool_input: {
      questions: [{
        header: "研究口径",
        multi_select: false,
        options: [{
          description: "先保证稳健性",
          label: "保守",
        }],
        question: "这次分析采用哪种研究口径？",
      }],
    },
    tool_name: "AskUserQuestion",
  };
  const plan = {
    request_id: "request-plan",
    summary: "先验证数据源，再生成最终报告",
    tool_input: { plan: "先验证数据源，再生成最终报告" },
    tool_name: "ExitPlanMode",
  };
  assert.equal(buildComposerInteractionQueue([question]).kind, "question");
  assert.equal(buildComposerInteractionQueue([plan]).kind, "plan");

  const questionHtml = await renderWithI18n(
    React.createElement(ComposerInteractionSurface, {
      onResponse: () => true,
      permissions: [question],
    }),
  );
  assert.match(questionHtml, /data-composer-interaction-kind="question"/);
  assert.match(questionHtml, /这次分析采用哪种研究口径？/);
  assert.match(questionHtml, /继续协作/);

  const planHtml = await renderWithI18n(
    React.createElement(ComposerInteractionSurface, {
      onResponse: () => true,
      permissions: [plan],
    }),
  );
  assert.match(planHtml, /data-composer-interaction-kind="plan"/);
  assert.match(planHtml, /先验证数据源，再生成最终报告/);
  assert.match(planHtml, />允许</);
  assert.match(planHtml, />拒绝</);
});

test("DM messages keep pending interactions as read-only evidence without a second action surface", async () => {
  const { resolvePendingInteractionOwner } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/message-item-projection.ts",
  );
  const { ContentRenderer } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/content/content-renderer.tsx",
  );
  const { AssistantMessageContent } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/assistant/assistant-message-content.tsx",
  );
  const permission = {
    request_id: "request-read-only",
    tool_input: { command: "echo protected" },
    tool_name: "Bash",
    tool_use_id: "tool-read-only",
  };
  assert.equal(resolvePendingInteractionOwner("dm_live"), "composer");
  assert.equal(resolvePendingInteractionOwner("dm_archived"), "composer");
  assert.equal(resolvePendingInteractionOwner("room_result"), "list");

  const toolHtml = await renderWithI18n(
    React.createElement(ContentRenderer, {
      canRespondToPermissions: true,
      content: [{
        id: "tool-read-only",
        input: permission.tool_input,
        name: permission.tool_name,
        type: "tool_use",
      }],
      isStreaming: false,
      onPermissionResponse: () => true,
      pendingInteractionOwner: "composer",
      pendingPermissionsByToolUseId: new Map([
        [permission.tool_use_id, permission],
      ]),
    }),
  );
  assert.match(toolHtml, /待确认/);
  assert.match(toolHtml, /echo protected/);
  assert.doesNotMatch(toolHtml, />允许</);
  assert.doesNotMatch(toolHtml, />拒绝</);
  assert.doesNotMatch(toolHtml, /data-human-interaction-surface/);

  const unmatchedHtml = await renderWithI18n(
    React.createElement(AssistantMessageContent, {
      activity: {
        emptyStreamStatus: null,
        showCursor: false,
        standalone: false,
        state: null,
      },
      direct: {
        projection: { content: [], streamingIndexes: new Set() },
        visible: false,
      },
      environment: {
        canRespondToPermissions: true,
        hiddenToolNames: [],
        mode: "dm_live",
        onPermissionResponse: () => true,
      },
      final: {
        content: null,
        isStreaming: false,
        mentions: [],
        streamingIndexes: new Set(),
        visible: false,
      },
      permissions: {
        all: [permission],
        matchedByToolUseId: new Map(),
        owner: "composer",
        unmatched: [permission],
      },
      process: {
        anchorRef: { current: null },
        expanded: false,
        projection: { content: [], streamingIndexes: new Set() },
        summary: "",
        toggle: () => {},
        visible: false,
      },
      showMaxTokensWarning: false,
    }),
  );
  assert.doesNotMatch(unmatchedHtml, /data-human-interaction-surface/);
  assert.doesNotMatch(unmatchedHtml, />允许</);
  assert.doesNotMatch(unmatchedHtml, />拒绝</);
});
