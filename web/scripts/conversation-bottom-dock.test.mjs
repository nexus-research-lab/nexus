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

async function renderWithI18n(element, locale = "zh") {
  const { I18N_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-context.ts",
  );
  const { MESSAGES } = await server.ssrLoadModule(
    "/src/shared/i18n/messages.ts",
  );
  return renderToStaticMarkup(
    React.createElement(
      I18N_CONTEXT.Provider,
      {
        value: {
          locale,
          setLocale: () => {},
          t: (key, params = {}) => Object.entries(params).reduce(
            (message, [name, value]) => message.replaceAll(
              `{${name}}`,
              String(value),
            ),
            MESSAGES[locale][key] ?? key,
          ),
        },
      },
      element,
    ),
  );
}

test("anchored overlay end alignment follows the trigger without leaving the viewport", async () => {
  const { resolveAnchoredOverlayPosition } = await server.ssrLoadModule(
    "/src/shared/ui/overlay/anchored-overlay-model.ts",
  );
  const originalWindow = globalThis.window;
  globalThis.window = {
    innerHeight: 600,
    innerWidth: 800,
  };
  try {
    const position = resolveAnchoredOverlayPosition({
      align: "end",
      anchor: {
        getBoundingClientRect: () => ({
          bottom: 540,
          height: 40,
          left: 660,
          right: 700,
          top: 500,
          width: 40,
        }),
      },
      estimatedHeight: 104,
      maxHeight: 320,
      minHeight: 44,
      minWidth: 248,
      placement: "top",
    });
    assert.equal(position.left, 452);
    assert.equal(position.width, 248);
    assert.equal(position.placement, "top");

    globalThis.window.innerWidth = 240;
    const narrowPosition = resolveAnchoredOverlayPosition({
      align: "end",
      anchor: {
        getBoundingClientRect: () => ({
          bottom: 540,
          height: 40,
          left: 190,
          right: 230,
          top: 500,
          width: 40,
        }),
      },
      estimatedHeight: 104,
      maxHeight: 320,
      minHeight: 44,
      minWidth: 248,
      placement: "top",
    });
    assert.equal(narrowPosition.left, 12);
    assert.equal(narrowPosition.width, 216);
  } finally {
    globalThis.window = originalWindow;
  }
});

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

test("标题栏与 Composer 自身边缘羽化且不改变滚动几何", async () => {
  const { ConversationPanelViewportArea } = await server.ssrLoadModule(
    "/src/features/conversation/shared/conversation-panel-layout.tsx",
  );
  const viewportHtml = renderToStaticMarkup(
    React.createElement(
      ConversationPanelViewportArea,
      null,
      React.createElement("div", null, "message"),
    ),
  );
  assert.match(viewportHtml, />message</);
  assert.doesNotMatch(viewportHtml, /data-composer-edge/);

  const sharedRecipes = await readFile(
    path.join(webRoot, "src/app/styles/theme-recipes.css"),
    "utf8",
  );
  const composerFadeRule = sharedRecipes.match(
    /\.nexus-chat-composer-edge::before\s*\{([\s\S]*?)\}/,
  )?.[1] ?? "";
  assert.match(composerFadeRule, /position:\s*absolute/);
  assert.match(composerFadeRule, /top:\s*-\d+px/);
  assert.match(composerFadeRule, /bottom:\s*0/);
  assert.match(composerFadeRule, /pointer-events:\s*none/);
  assert.match(composerFadeRule, /linear-gradient/);
  assert.doesNotMatch(composerFadeRule, /\b(?:margin|padding)(?:-|:)/);
  assert.match(
    sharedRecipes,
    /\[data-conversation-status-stack\]:has\(> \*\)\s*\+\s*\[data-conversation-composer-anchor\]\s*\.nexus-chat-composer-edge::before\s*\{[\s\S]*?top:\s*0[\s\S]*?background:\s*var\(--background\)/,
  );

  const headerSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/room/surface/layout/room-surface-header.tsx",
    ),
    "utf8",
  );
  const headerStyles = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/room/surface/room-conversation-header-edge.css",
    ),
    "utf8",
  );
  const mobileHeaderSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/room/surface/mobile/room-mobile-header.tsx",
    ),
    "utf8",
  );
  const roomSurfaceContentSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/room/surface/layout/room-surface-content.tsx",
    ),
    "utf8",
  );
  const desktopTopFadeRule = headerStyles.match(
    /\.nexus-room-conversation-reading-edge::before\s*\{([\s\S]*?)\}/,
  )?.[1] ?? "";
  assert.match(headerSource, /data-room-conversation-header-edge="true"/);
  assert.match(
    roomSurfaceContentSource,
    /data-room-conversation-reading-edge="true"/,
  );
  assert.match(
    mobileHeaderSource,
    /data-room-conversation-header-edge="true"/,
  );
  assert.match(
    mobileHeaderSource,
    /nexus-room-conversation-header-edge--mobile/,
  );
  assert.match(
    headerStyles,
    /\.nexus-room-conversation-reading-edge\s*\{[\s\S]*?overflow:\s*hidden/,
  );
  assert.match(desktopTopFadeRule, /top:\s*0/);
  assert.match(
    headerStyles,
    /\.nexus-room-conversation-header-edge--mobile::after\s*\{\s*top:\s*100%/,
  );
  assert.match(
    headerStyles,
    /\.nexus-room-conversation-reading-edge::before,\s*\n\.nexus-room-conversation-header-edge--mobile::after\s*\{[\s\S]*?position:\s*absolute[\s\S]*?z-index:\s*10[\s\S]*?pointer-events:\s*none[\s\S]*?linear-gradient/,
  );
  assert.doesNotMatch(
    desktopTopFadeRule,
    /\b(?:margin|padding)(?:-|:)/,
  );
  assert.doesNotMatch(
    headerStyles,
    /\.nexus-room-conversation-header-edge::after\s*\{/,
  );
  assert.match(
    headerStyles,
    /\.nexus-room-conversation-header-edge\s*>\s*\.shell-region-header\s*\{[\s\S]*?box-shadow:\s*none/,
  );
});

test("Room header keeps view and member controls on one spacing rhythm", async () => {
  const headerStyles = await readFile(
    path.join(
      webRoot,
      "src/shared/ui/workspace/surface/workspace-surface-header.css",
    ),
    "utf8",
  );
  const memberSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/room/group/header/group-member-avatar-stack.tsx",
    ),
    "utf8",
  );

  assert.match(headerStyles, /--workspace-header-control-gap:\s*4px/);
  assert.match(
    headerStyles,
    /\.workspace-surface-header-tool-cluster\s*\{[\s\S]*?gap:\s*var\(--workspace-header-control-gap\)/,
  );
  assert.match(memberSource, /\bh-9\b/);
  assert.match(memberSource, /\bgap-1\.5\b/);
  assert.match(memberSource, /\bpx-2\.5\b/);
});

test("shared WebSocket session leases keep a live Room bound until its last consumer leaves", async () => {
  const { SessionBindingLeaseRegistry } = await server.ssrLoadModule(
    "/src/lib/websocket/session-binding-leases.ts",
  );
  const sent = [];
  let connected = true;
  const registry = new SessionBindingLeaseRegistry(
    (message) => {
      sent.push(message);
      return { disposition: "sent" };
    },
    () => connected,
  );
  const firstLease = {};
  const secondLease = {};
  const binding = {
    type: "bind_session",
    session_key: "room:group:conversation-1",
    room_id: "room-1",
    conversation_id: "conversation-1",
  };

  const releaseFirst = registry.acquire(firstLease, binding);
  const releaseSecond = registry.acquire(secondLease, binding);
  assert.deepEqual(
    sent.map((message) => message.type),
    ["bind_session", "bind_session"],
  );

  releaseFirst();
  assert.equal(
    sent.some((message) => message.type === "unbind_session"),
    false,
  );

  connected = false;
  registry.replay();
  connected = true;
  registry.replay();
  assert.equal(
    sent.filter((message) => message.type === "bind_session").length,
    3,
  );

  releaseSecond();
  releaseSecond();
  assert.deepEqual(sent.at(-1), {
    type: "unbind_session",
    session_key: "room:group:conversation-1",
  });
  assert.equal(
    sent.filter((message) => message.type === "unbind_session").length,
    1,
  );
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
  assert.match(html, /第 2 \/ 3 步/);
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

test("Room and DM stack Goal, Task, and scroll controls upward from the Composer", async () => {
  const { ConversationPanelBottomArea } = await server.ssrLoadModule(
    "/src/features/conversation/shared/conversation-panel-layout.tsx",
  );
  const stackedHtml = renderToStaticMarkup(
    React.createElement(
      ConversationPanelBottomArea,
      {
        activity: React.createElement("button", { "data-test-task-layer": true }, "task"),
        goal: React.createElement("div", { "data-test-goal-layer": true }, "goal"),
        isMobileLayout: false,
        providerWarningVisible: false,
        scrollToLatest: {
          direction: null,
          isLoading: false,
          onClick: () => {},
          unreadCount: 0,
          visible: false,
        },
      },
      React.createElement("div", { "data-test-composer-layer": true }, "composer"),
    ),
  );
  assert.ok(
    stackedHtml.indexOf("data-test-task-layer")
      < stackedHtml.indexOf("data-test-goal-layer"),
  );
  assert.ok(
    stackedHtml.indexOf("data-test-goal-layer")
      < stackedHtml.indexOf("data-test-composer-layer"),
  );

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
    layoutSource.indexOf("data-conversation-bottom-stack")
      < layoutSource.indexOf("data-conversation-status-stack"),
    "the Composer bottom stack must own its status layer",
  );
  assert.ok(
    layoutSource.indexOf("<ConversationPanelFloatingControls")
      < layoutSource.indexOf("data-conversation-status-stack"),
    "Task and scroll controls must stack above the Goal status layer",
  );
  assert.ok(
    layoutSource.indexOf("data-conversation-status-stack")
      < layoutSource.indexOf("data-conversation-composer-anchor"),
    "Goal/provider content must sit directly above the Composer",
  );
  const bottomStackStart = layoutSource.indexOf(
    "data-conversation-bottom-stack",
  );
  const composerAnchorStart = layoutSource.indexOf(
    "data-conversation-composer-anchor",
  );
  assert.ok(
    layoutSource.indexOf("<ConversationPanelFloatingControls", bottomStackStart)
      < composerAnchorStart,
    "Task and scroll controls must use the whole bottom stack as their anchor",
  );

  const recipes = await readFile(
    path.join(webRoot, "src/app/styles/theme-recipes.css"),
    "utf8",
  );
  assert.doesNotMatch(recipes, /nexus-conversation-pre-composer/);
  assert.doesNotMatch(recipes, /padding-bottom:\s*3\.5rem/);

  const goalModelSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/shared/goal/goal-model.ts",
    ),
    "utf8",
  );
  assert.match(goalModelSource, /CONVERSATION_CONTENT_LANE_CLASS_NAME/);
  assert.match(goalModelSource, /rounded-\[16px\]/);
  assert.match(goalModelSource, /shadow-\(--surface-control-shadow\)/);
  assert.doesNotMatch(goalModelSource, /GOAL_PANEL_SURFACE_CLASS_NAME\s*=[\s\S]*?border-b/);
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

test("DM and Room pending interactions replace the Composer input in one stable queue", async () => {
  const { buildComposerInteractionQueue } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/interaction/composer-interaction-model.ts",
  );
  const { ComposerInteractionSurface } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/interaction/composer-interaction-surface.tsx",
  );
  const { getPermissionScopeActionLabelKey } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/interaction/composer-permission-model.ts",
  );
  const { ComposerPanel } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-panel.tsx",
  );
  const { getReadablePermissionSuggestions } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/blocks/tool/tool-block-model.ts",
  );
  const {
    removePendingPermission,
    upsertPendingPermission,
  } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/permission/pending-permission-state.ts",
  );
  const first = {
    agent_id: "agent-1",
    request_id: "request-first",
    summary: "旧快照",
    tool_input: { command: "echo old" },
    tool_name: "Bash",
  };
  const second = {
    agent_id: "agent-2",
    request_id: "request-second",
    summary: "第二项",
    tool_input: { file_path: "/tmp/second" },
    tool_name: "Read",
  };
  const latest = {
    ...first,
    suggestions: [{
      behavior: "allow",
      destination: "localSettings",
      rules: [{
        rule_content: "echo updated",
        tool_name: "Bash",
      }],
      type: "addRules",
    }],
    summary: "最新快照",
    tool_input: { command: "echo updated" },
  };

  let pending = upsertPendingPermission([], first);
  pending = upsertPendingPermission(pending, second);
  pending = upsertPendingPermission(pending, latest);
  assert.deepEqual(pending, [latest, second]);
  assert.deepEqual(removePendingPermission(pending, first.request_id), [second]);
  assert.deepEqual(
    getReadablePermissionSuggestions(latest.suggestions).map(({ label }) => label),
    ["写入本地设置"],
  );
  assert.equal(
    getPermissionScopeActionLabelKey(
      latest.tool_name,
      latest.suggestions[0],
    ),
    "composer.permission_add_bash_allow_rule",
  );

  const queue = buildComposerInteractionQueue(pending);
  assert.equal(queue.current?.request_id, first.request_id);
  assert.equal(queue.kind, "permission");
  assert.equal(queue.total, 2);

  const interaction = React.createElement(ComposerInteractionSurface, {
    agentAvatarMap: {
      "agent-1": null,
      "agent-2": null,
    },
    agentNameMap: {
      "agent-1": "Researcher",
      "agent-2": "Reviewer",
    },
    onResponse: () => true,
    permissions: pending,
  });
  const interactionHtml = await renderWithI18n(interaction);
  assert.match(interactionHtml, /data-composer-interaction-surface="true"/);
  assert.match(
    interactionHtml,
    /data-composer-interaction-agent-id="agent-1"/,
  );
  assert.match(interactionHtml, /Researcher/);
  assert.match(interactionHtml, /data-composer-interaction-requester="true"/);
  assert.match(
    interactionHtml,
    /data-pending-interaction-request-id="request-first"/,
  );
  assert.match(interactionHtml, /1 \/ 2/);
  assert.match(interactionHtml, /echo updated/);
  assert.doesNotMatch(interactionHtml, /\/tmp\/second/);
  assert.match(interactionHtml, />允许本次</);
  assert.match(interactionHtml, /aria-label="选择允许范围"/);
  assert.match(interactionHtml, />拒绝</);
  const interactionEnglishHtml = await renderWithI18n(interaction, "en");
  assert.match(interactionEnglishHtml, />Allow once</);
  assert.match(interactionEnglishHtml, />Deny</);
  assert.match(interactionEnglishHtml, />Terminal</);
  assert.match(
    interactionEnglishHtml,
    /aria-label="Choose permission scope"/,
  );
  assert.doesNotMatch(interactionEnglishHtml, /允许本次|拒绝|终端/);

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
  assert.match(replacedHtml, /data-composer-edge="true"/);
  assert.match(replacedHtml, /\bnexus-chat-composer-edge\b/);
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
  assert.match(inputHtml, /data-composer-edge="true"/);
  assert.match(inputHtml, /\bnexus-chat-composer-edge\b/);
  assert.match(inputHtml, /data-composer-surface="input"/);
  assert.match(inputHtml, /<textarea/);
  assert.match(inputHtml, /data-composer-powered-by="true"/);

  const roomProjectionSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/room/group/chat/panel/controller/group-chat-panel-projection.ts",
    ),
    "utf8",
  );
  const roomViewSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/room/group/chat/panel/view/group-chat-panel-view.tsx",
    ),
    "utf8",
  );
  assert.match(
    roomProjectionSource,
    /composerInteraction:[\s\S]*pending_permissions/,
  );
  assert.match(
    roomViewSource,
    /ComposerInteractionSurface[\s\S]*interactionIdentity[\s\S]*interactionSurface/,
  );
});

test("Composer growth is capped and collapsed file tools show only the leaf name", async () => {
  const {
    COMPOSER_TEXTAREA_MAX_HEIGHT_PX,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/composer-styles.ts",
  );
  const {
    getCompactToolInputSummary,
    getToolInputSummary,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/tool-activity.ts",
  );
  const {
    buildToolBlockViewModel,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/blocks/tool/tool-block-model.ts",
  );
  const {
    buildProcessSummary,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/process/message-process-summary.ts",
  );
  const absolutePath = "/Users/test/workspace/output/permission_test.txt";
  const toolInput = { file_path: absolutePath };
  const toolUse = {
    id: "tool-write-file",
    input: toolInput,
    name: "Write",
    type: "tool_use",
  };
  const model = buildToolBlockViewModel({
    status: "running",
    toolUse,
  });

  assert.equal(
    COMPOSER_TEXTAREA_MAX_HEIGHT_PX,
    120,
    "Composer should stop growing after roughly five text lines",
  );
  assert.equal(getCompactToolInputSummary(toolInput), "permission_test.txt");
  assert.equal(getToolInputSummary(toolInput), absolutePath);
  assert.equal(model.collapsedDetailText, "permission_test.txt");
  assert.equal(model.expandedDetailText, absolutePath);
  assert.equal(
    buildProcessSummary({
      pendingPermissionCount: 0,
      processContent: [toolUse],
    }),
    "1 次动作 · 最近：写入内容：permission_test.txt",
  );
});

test("questions and plan confirmations use the same Composer replacement owner", async () => {
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
  assert.match(planHtml, />允许本次</);
  assert.match(planHtml, />拒绝</);
});

test("DM and Room messages never remount interaction options outside the Composer", async () => {
  const { resolvePendingInteractionOwner } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/message-item-projection.ts",
  );
  const { MessageActivityStatus } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/message-activity-status.tsx",
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
  assert.equal(resolvePendingInteractionOwner("room_result"), "composer");
  assert.equal(resolvePendingInteractionOwner("room_thread"), "composer");

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
  assert.match(toolHtml, /surface-muted-background/);
  assert.doesNotMatch(toolHtml, /--warning|animate-pulse/);
  assert.doesNotMatch(toolHtml, />允许</);
  assert.doesNotMatch(toolHtml, />拒绝</);
  assert.doesNotMatch(toolHtml, /data-human-interaction-surface/);

  const questionTool = {
    id: "tool-question-evidence",
    input: {
      questions: [{
        header: "芯片类型",
        multi_select: false,
        options: [
          { label: "Apple M3 / M4" },
          { label: "ARM Cortex-M3 / M4" },
        ],
        question: "这里的 M3/M4 指哪类芯片？",
      }],
    },
    name: "AskUserQuestion",
    type: "tool_use",
  };
  const questionPermission = {
    interaction_mode: "question",
    request_id: "request-question-evidence",
    tool_input: questionTool.input,
    tool_name: questionTool.name,
    tool_use_id: questionTool.id,
  };
  const questionEvidenceScenarios = [
    {
      content: [questionTool],
      name: "unmatched live question",
      pending: new Map(),
    },
    {
      content: [questionTool],
      name: "matched pending question",
      pending: new Map([[questionTool.id, questionPermission]]),
    },
    {
      content: [
        questionTool,
        {
          content: "answered",
          tool_use_id: questionTool.id,
          type: "tool_result",
        },
      ],
      name: "restored historical question",
      pending: new Map(),
    },
  ];
  for (const scenario of questionEvidenceScenarios) {
    const questionEvidenceHtml = await renderWithI18n(
      React.createElement(ContentRenderer, {
        canRespondToPermissions: true,
        content: scenario.content,
        isStreaming: false,
        onPermissionResponse: () => true,
        pendingInteractionOwner: "composer",
        pendingPermissionsByToolUseId: scenario.pending,
      }),
    );
    assert.match(
      questionEvidenceHtml,
      /等待你的确认/,
      `${scenario.name} should retain neutral tool evidence`,
    );
    assert.doesNotMatch(
      questionEvidenceHtml,
      /需要你的回应|芯片类型|Apple M3 \/ M4|ARM Cortex-M3 \/ M4|继续协作|ask-user-question|data-selected/,
      `${scenario.name} must not remount the legacy question option tree`,
    );
  }

  const activityHtml = renderToStaticMarkup(React.createElement(
    MessageActivityStatus,
    { state: "waiting_permission" },
  ));
  assert.match(activityHtml, /等待确认/);
  assert.match(activityHtml, /--text-muted/);
  assert.doesNotMatch(
    activityHtml,
    /--warning|message-activity-spinner-track/,
  );

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
