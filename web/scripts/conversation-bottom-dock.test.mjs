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

async function loadI18nValue(locale = "zh") {
  const { MESSAGES } = await server.ssrLoadModule(
    "/src/shared/i18n/messages.ts",
  );
  return {
    locale,
    setLocale: () => {},
    t: (key, params = {}) => Object.entries(params).reduce(
      (message, [name, value]) => message.replaceAll(
        `{${name}}`,
        String(value),
      ),
      MESSAGES[locale][key] ?? key,
    ),
  };
}

async function renderWithI18n(element, locale = "zh") {
  const { I18N_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-context.ts",
  );
  const value = await loadI18nValue(locale);
  return renderToStaticMarkup(
    React.createElement(
      I18N_CONTEXT.Provider,
      { value },
      element,
    ),
  );
}

test("上下文圆环只显示 runtime 快照，并保留 Room 每个 Agent 的最近值", async () => {
  const {
    ComposerContextUsage,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/footer/composer-context-usage.tsx",
  );
  const {
    projectComposerContextUsage,
    projectContextUsage,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/footer/composer-context-usage-model.ts",
  );
  const { AGENT_SESSION_EVENT_HANDLERS } = await server.ssrLoadModule(
    "/src/hooks/agent/transport/handlers/session-event-handlers.ts",
  );
  const usage = {
    max_tokens: 258_000,
    percentage: 75.96,
    total_tokens: 196_000,
  };

  assert.deepEqual(projectContextUsage(usage), {
    maxTokens: 258_000,
    percentage: 76,
    toneClassName: "text-(--text-soft)",
    totalTokens: 196_000,
  });
  assert.equal(projectContextUsage(null), null);
  const emptyHtml = await renderWithI18n(
    React.createElement(ComposerContextUsage, { usage: null }),
  );
  assert.match(emptyHtml, /data-context-usage-slot="empty"/);
  assert.doesNotMatch(emptyHtml, /<button/);
  const html = await renderWithI18n(
    React.createElement(ComposerContextUsage, { usage }),
  );
  assert.match(html, /data-context-usage-slot="ready"/);
  assert.match(html, /data-context-usage="76"/);
  assert.match(html, /上下文窗口已用 76%/);
  assert.match(html, /196\.0K/);
  assert.match(html, /258\.0K/);
  assert.equal(
    (html.match(/stroke-width="2"/g) ?? []).length,
    2,
    "context track and progress use the same restrained 2px stroke",
  );

  const groupedProjection = projectComposerContextUsage({
    items: [
      { agentId: "amy", name: "Amy", usage },
      {
        agentId: "devin",
        name: "Devin",
        usage: { ...usage, percentage: 88, total_tokens: 227_040 },
      },
    ],
    usage: null,
  });
  assert.equal(groupedProjection.grouped, true);
  assert.equal(groupedProjection.summary.percentage, 88);
  assert.deepEqual(
    groupedProjection.items.map((item) => item.name),
    ["Amy", "Devin"],
  );
  const groupedHtml = await renderWithI18n(
    React.createElement(ComposerContextUsage, {
      items: [
        { agentId: "amy", name: "Amy", usage },
        { agentId: "devin", name: "Devin", usage: null },
      ],
      usage: null,
    }),
  );
  assert.match(groupedHtml, /Room 上下文窗口，2 个 Agent，最高已用 76%/);

  let usageByAgent = {};
  const context = {
    scope: {
      isCurrentSessionEvent: (sessionKey) => sessionKey === "room-session",
    },
    state: {
      setContextUsageByAgent: (update) => {
        usageByAgent = typeof update === "function"
          ? update(usageByAgent)
          : update;
      },
    },
  };
  for (const agentId of ["amy", "devin"]) {
    AGENT_SESSION_EVENT_HANDLERS.context_usage({
      agent_id: agentId,
      data: usage,
      event_type: "context_usage",
      protocol_version: 2,
      session_key: "room-session",
      timestamp: 1,
    }, context);
  }
  assert.deepEqual(Object.keys(usageByAgent), ["amy", "devin"]);
});

test("round 结束前后 Composer 提交动作保持稳定几何", async () => {
  const { ComposerSubmitButton } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/composer-submit-button.tsx",
  );
  const props = {
    isDisabled: true,
    isGoalCreating: false,
    isGoalMode: false,
    isPreparingAttachments: false,
    onSend: () => {},
    onStop: () => {},
    sendLabel: "发送消息",
    stopLabel: "停止生成",
  };
  const sendHtml = renderToStaticMarkup(
    React.createElement(ComposerSubmitButton, {
      ...props,
      shouldStop: false,
    }),
  );
  const stopHtml = renderToStaticMarkup(
    React.createElement(ComposerSubmitButton, {
      ...props,
      shouldStop: true,
    }),
  );
  for (const actionHtml of [sendHtml, stopHtml]) {
    assert.match(actionHtml, /\bnexus-chat-composer-submit\b/);
    assert.match(actionHtml, /\bmin-h-8\b/);
    assert.doesNotMatch(actionHtml, /nexus-chat-composer-submit-slot/);
  }
  assert.doesNotMatch(sendHtml, /\bnexus-chat-composer-submit-stop\b/);
  assert.match(stopHtml, /\bnexus-chat-composer-submit-stop\b/);

  const recipeSource = await readFile(
    path.join(webRoot, "src/app/styles/theme-recipes.css"),
    "utf8",
  );
  assert.match(
    recipeSource,
    /\.nexus-chat-composer-submit \{[\s\S]*?width: 2rem;[\s\S]*?padding-inline: 0;[\s\S]*?border-radius: 999px;/,
  );
  assert.match(
    recipeSource,
    /\.nexus-chat-composer-submit:disabled \{[\s\S]*?opacity: 1;[\s\S]*?background: var\(--text-soft\);[\s\S]*?color: #fff;/,
  );
  assert.doesNotMatch(recipeSource, /nexus-chat-composer-submit-slot/);
});

test("Composer 回复阶段只保留停止快捷键提示", async () => {
  const { projectComposerFooterStatus } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/footer/composer-footer-model.ts",
  );
  const projection = projectComposerFooterStatus({
    activeError: null,
    copy: {
      compacting: "正在压缩上下文",
      goalCreating: "正在创建 Goal",
      preparingAttachments: "正在准备附件",
      replying: "回复中",
      sending: "发送中",
      stopHint: "[ESC 停止]",
    },
    isGoalCreating: false,
    isPreparingAttachments: false,
    runtimeActivity: "replying",
  });

  assert.equal(projection.message, null);
  assert.equal(projection.frames, null);
  assert.equal(projection.hint, "[ESC 停止]");
});

test("Action Menu 的空 footer 使用稳定引用，避免定位状态自循环", async () => {
  const source = await readFile(
    path.join(webRoot, "src/shared/ui/menu/action-menu.tsx"),
    "utf8",
  );

  assert.match(
    source,
    /const EMPTY_ACTION_MENU_ITEMS: UiActionMenuItem\[\] = \[\];/,
  );
  assert.equal(
    (source.match(/footerItems = EMPTY_ACTION_MENU_ITEMS/g) ?? []).length,
    2,
  );
  assert.doesNotMatch(source, /footerItems = \[\]/);
});

test("anchored overlay style clears the unused vertical axis", async () => {
  const source = await readFile(
    path.join(webRoot, "src/shared/ui/overlay/anchored-overlay-layer.ts"),
    "utf8",
  );

  assert.match(source, /bottom: position\.bottom \?\? "auto"/);
  assert.match(source, /top: position\.top \?\? "auto"/);
});

test("anchored overlay end alignment follows the trigger without leaving the viewport", async () => {
  const {
    areAnchoredOverlayPositionsEqual,
    resolveAnchoredOverlayPosition,
  } = await server.ssrLoadModule(
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
    assert.equal(
      areAnchoredOverlayPositionsEqual(position, { ...position }),
      true,
    );
    assert.equal(
      areAnchoredOverlayPositionsEqual(position, {
        ...position,
        left: position.left + 1,
      }),
      false,
    );

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
  const hidden = await renderWithI18n(
    React.createElement(ScrollToLatestButton, {
      isLoading: false,
      onClick: () => {},
      visible: false,
    }),
  );
  const visible = await renderWithI18n(
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
  const hidden = await renderWithI18n(
    React.createElement(
      ConversationPanelViewport,
      { floatingDockOccupied: false, isMobileLayout: false, viewport },
      React.createElement("div", null, "message"),
    ),
  );
  const occupied = await renderWithI18n(
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

test("加载更早消息的状态跟随界面语言", async () => {
  const { ConversationPanelViewport } = await server.ssrLoadModule(
    "/src/features/conversation/shared/conversation-panel-layout.tsx",
  );
  const viewport = {
    error: null,
    isHistoryLoading: true,
    scrollRef: { current: null },
  };
  const element = React.createElement(
    ConversationPanelViewport,
    { floatingDockOccupied: false, isMobileLayout: false, viewport },
    React.createElement("div", null, "message"),
  );
  const chinese = await renderWithI18n(element);
  const english = await renderWithI18n(element, "en");

  assert.match(chinese, /正在加载更早消息\.\.\./);
  assert.match(english, /Loading earlier messages\.\.\./);
  assert.doesNotMatch(english, /正在加载/);
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

test("Room 右侧上下文面使用软分栏而不是硬竖线", async () => {
  const [
    resizeHandleSource,
    splitStyles,
    surfaceSource,
    threadPanelSource,
    auxiliaryPanelSource,
  ] = await Promise.all([
    "src/shared/ui/layout/panel-resize-handle.tsx",
    "src/features/conversation/room/surface/layout/room-surface-split.css",
    "src/features/conversation/room/surface/layout/room-surface-content.tsx",
    "src/features/conversation/room/surface/layout/room-thread-inline-panel.tsx",
    "src/features/conversation/room/surface/layout/room-surface-auxiliary-panel.tsx",
  ].map((file) => readFile(path.join(webRoot, file), "utf8")));

  assert.match(surfaceSource, /nexus-room-surface-split/);
  assert.match(surfaceSource, /nexus-room-surface-conversation/);
  for (const panelSource of [threadPanelSource, auxiliaryPanelSource]) {
    assert.match(panelSource, /nexus-room-surface-side-panel/);
    assert.match(panelSource, /variant="gutter"/);
    assert.doesNotMatch(panelSource, /\bborder-l\b|\bdivider-subtle\b/);
  }
  assert.match(resizeHandleSource, /relative w-2 shrink-0 self-stretch/);
  assert.match(resizeHandleSource, /cursor-col-resize/);
  assert.doesNotMatch(
    resizeHandleSource,
    /<span|border-l-\[6px\]|border-y-\[5px\]|group-hover\/resize/,
  );
  const sidePanelRule = splitStyles.match(
    /\.nexus-room-surface-side-panel\s*\{([\s\S]*?)\}/,
  )?.[1] ?? "";
  assert.match(
    sidePanelRule,
    /background:\s*color-mix\([\s\S]*?var\(--surface-panel-background\) 72%[\s\S]*?box-shadow:\s*-8px 0 20px -18px color-mix\([\s\S]*?var\(--shadow-color\) 14%/,
  );
  assert.match(
    splitStyles,
    /\.nexus-room-surface-split\s*\{[\s\S]*?background:\s*var\(--surface-canvas-background\)/,
  );
  assert.match(
    splitStyles,
    /\.nexus-room-surface-conversation\s*\{[\s\S]*?background:\s*transparent/,
  );
  assert.doesNotMatch(sidePanelRule, /border(?:-left)?:|margin-left:/);
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
      sessionSettingsController: {
        busy: false,
        ensureTargetsLoaded: () => {},
        error: null,
        hasModelOverride: false,
        hasPermissionOverride: false,
        inheritedModel: "agent-model",
        inheritedPermissionMode: "default",
        inheritedProvider: "agent-provider",
        isDangerousPermission: false,
        modelBusy: false,
        modelLabel: "agent-model",
        permissionLabel: "默认",
        providerOptions: null,
        resetModel: () => {},
        resetPermission: () => {},
        resetTarget: () => {},
        saving: false,
        scope: {
          initialTargetId: "agent-1",
          runtimeKind: "nxs",
          targets: [
            {
              agentId: "agent-1",
              defaultModel: "agent-model",
              defaultPermissionMode: "default",
              defaultProvider: "agent-provider",
              name: "Nexus",
              sessionKey: "agent:agent-1:ws:group:conversation-1",
            },
            {
              agentId: "agent-2",
              defaultModel: "",
              defaultPermissionMode: "acceptEdits",
              defaultProvider: "",
              name: "Amy",
              sessionKey: "agent:agent-2:ws:group:conversation-1",
            },
          ],
        },
        selectTarget: () => {},
        settings: {
          model: "",
          permission_mode: "",
          provider: "",
        },
        target: {
          agentId: "agent-1",
          defaultModel: "agent-model",
          defaultPermissionMode: "default",
          defaultProvider: "agent-provider",
          name: "Nexus",
          sessionKey: "agent:agent-1:ws:group:conversation-1",
        },
        targetViews: [
          {
            busy: false,
            modelLabel: "agent-model",
            target: {
              agentId: "agent-1",
              defaultModel: "agent-model",
              defaultPermissionMode: "default",
              defaultProvider: "agent-provider",
              name: "Nexus",
              sessionKey: "agent:agent-1:ws:group:conversation-1",
            },
          },
          {
            busy: false,
            modelLabel: "global-model",
            target: {
              agentId: "agent-2",
              defaultModel: "",
              defaultPermissionMode: "acceptEdits",
              defaultProvider: "",
              name: "Amy",
              sessionKey: "agent:agent-2:ws:group:conversation-1",
            },
          },
        ],
        updateModel: () => {},
        updatePermission: () => {},
      },
      sessionSettingsDisabled: false,
      showPoweredByNexus: true,
      submit: {
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
  assert.match(html, /aria-label="当前 Session 权限"/);
  assert.match(html, /aria-label="当前 Session 模型"/);
  assert.doesNotMatch(html, /aria-label="Agent 设置"/);
  assert.doesNotMatch(html, />agent-model</);
  assert.match(
    html,
    /nexus-chat-composer-footer-trailing[^"]*\bgap-2\b/,
    "Composer controls keep the same 8px rhythm on both sides",
  );
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

test("DM Composer keeps direct Session permission and model controls", async () => {
  const { ComposerSessionControls } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/footer/composer-session-controls.tsx",
  );
  const target = {
    agentId: "agent-1",
    defaultModel: "agent-model",
    defaultPermissionMode: "default",
    defaultProvider: "agent-provider",
    name: "Nexus",
    sessionKey: "agent:agent-1:session-1",
  };
  const controller = {
    busy: false,
    ensureTargetsLoaded: () => {},
    error: null,
    hasModelOverride: false,
    hasPermissionOverride: false,
    inheritedModel: "agent-model",
    inheritedPermissionMode: "default",
    inheritedProvider: "agent-provider",
    isDangerousPermission: false,
    modelBusy: false,
    modelLabel: "agent-model",
    permissionLabel: "默认",
    providerOptions: null,
    resetModel: () => {},
    resetPermission: () => {},
    resetTarget: () => {},
    saving: false,
    scope: {
      initialTargetId: target.agentId,
      runtimeKind: "nxs",
      targets: [target],
    },
    selectTarget: () => {},
    settings: {
      model: "",
      permission_mode: "",
      provider: "",
    },
    target,
    targetViews: [{
      busy: false,
      modelLabel: "agent-model",
      target,
    }],
    updateModel: () => {},
    updatePermission: () => {},
  };
  const html = await renderWithI18n(
    React.createElement(
      React.Fragment,
      null,
      React.createElement(ComposerSessionControls, {
        controller,
        disabled: false,
        slot: "leading",
      }),
      React.createElement(ComposerSessionControls, {
        controller,
        disabled: false,
        slot: "trailing",
      }),
    ),
  );

  assert.match(html, /aria-label="当前 Session 权限"/);
  assert.match(html, /aria-label="当前 Session 模型"/);
  assert.match(html, />agent-model</);
  assert.doesNotMatch(html, /aria-label="Agent 设置"/);
});

test("新 Agent 与新 Session 默认自动接受编辑", async () => {
  const [agentOptionsSource, runtimeOptionsSource] = await Promise.all([
    readFile(path.join(webRoot, "src/lib/agent-options.ts"), "utf8"),
    readFile(path.join(webRoot, "src/config/runtime-options.ts"), "utf8"),
  ]);

  assert.match(
    agentOptionsSource,
    /DEFAULT_AGENT_PERMISSION_MODE = "acceptEdits"/,
  );
  assert.match(
    runtimeOptionsSource,
    /permission_mode: DEFAULT_AGENT_PERMISSION_MODE/,
  );
});

test("Session setting menus expose concrete choices and a separate reset action", async () => {
  const {
    buildResetSessionSettingItem,
    buildSessionModelItems,
    buildSessionPermissionItems,
    RESET_SESSION_SETTING_VALUE,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/footer/composer-session-control-options.tsx",
  );
  const controller = {
    inheritedModel: "model-a",
    inheritedPermissionMode: "default",
    inheritedProvider: "provider-a",
    providerOptions: {
      items: [{
        display_name: "Provider A",
        models: [{
          display_name: "Model A",
          model_id: "model-a",
        }],
        provider: "provider-a",
      }],
    },
    settings: {
      model: "",
      permission_mode: "",
      provider: "",
    },
  };
  const translate = (key) => key;
  const modelItems = buildSessionModelItems(controller);
  const permissionItems = buildSessionPermissionItems(controller, translate);
  const resetItem = buildResetSessionSettingItem(true, translate);

  assert.equal(modelItems.length, 1);
  assert.equal(modelItems[0].active, true);
  assert.equal(permissionItems.length, 5);
  assert.equal(permissionItems[0].active, true);
  assert.equal(
    permissionItems.every((item) => item.description),
    true,
    "permission choices retain a concise second line",
  );
  assert.equal(
    permissionItems.every((item) => item.icon),
    true,
    "permission choices retain a semantic leading icon",
  );
  assert.equal(resetItem.value, RESET_SESSION_SETTING_VALUE);
  assert.equal(resetItem.disabled, true);
  assert.equal(
    permissionItems.some((item) => item.value === RESET_SESSION_SETTING_VALUE),
    false,
    "reset remains below the concrete choices",
  );

  const menuStyleSource = await readFile(
    path.join(webRoot, "src/shared/ui/menu/menu-styles.ts"),
    "utf8",
  );
  assert.match(
    menuStyleSource,
    /radius-control-lg/,
    "4px inset menu rows stay concentric with the 16px popover",
  );

  const roomModelSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/shared/composer/components/footer/composer-room-model-control.tsx",
    ),
    "utf8",
  );
  assert.match(
    roomModelSource,
    /canHoverSelect[\s\S]*onPointerEnter/,
    "hovering a Room Agent opens its model choices without another detail page",
  );
  assert.doesNotMatch(
    roomModelSource,
    /RoomAgentSettingsDetail|RoomSettingsDetailRow/,
    "Room model selection no longer carries the obsolete Agent detail layer",
  );
  const sessionControlsSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/shared/composer/components/footer/composer-session-controls.tsx",
    ),
    "utf8",
  );
  assert.match(
    sessionControlsSource,
    /SESSION_PERMISSION_MENU_WIDTH = 288/,
    "permission menus retain their readable width",
  );
  assert.match(
    sessionControlsSource,
    /SESSION_MODEL_MENU_WIDTH = 256/,
    "DM model choices use the compact menu width",
  );
  assert.match(
    sessionControlsSource,
    /ariaLabel=\{t\("composer\.session_model"\)\}[\s\S]*density="compact"/,
    "DM model choices use compact Action Menu rows",
  );
  assert.match(
    roomModelSource,
    /ROOM_MODEL_MENU_WIDTH = 256/,
    "Room model choices use the compact model width",
  );
  assert.match(
    roomModelSource,
    /ROOM_MODEL_AGENT_MENU_WIDTH = 224/,
    "Room Agent selection stays narrower than the model choices",
  );
  assert.match(
    roomModelSource,
    /UiActionMenuContent[\s\S]*density="compact"/,
    "Room model choices reuse compact Action Menu rows",
  );
  assert.match(
    roomModelSource,
    /function RoomModelPanel[\s\S]*OVERLAY_SURFACE_CLASS_NAME/,
    "Room cascade surfaces stay on stable child panels instead of the resizing root",
  );
  assert.match(
    roomModelSource,
    /Math\.max\(agentHeight, modelHeight\)/,
    "Room cascade reserves a stable height before the first Agent hover",
  );
});

test("Workspace Task uses a centered step-summary capsule and an absolute upward detail", async () => {
  const { WorkspaceTaskPanel } = await server.ssrLoadModule(
    "/src/shared/ui/workspace/surface/workspace-task-strip.tsx",
  );
  const { resolveWorkspaceTaskState } = await server.ssrLoadModule(
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
  assert.deepEqual(resolveWorkspaceTaskState(todos), {
    summary: {
      completedCount: 1,
      currentStep: 2,
      hasRunningTask: true,
      summary: "正在核对布局",
      totalCount: 3,
    },
    todos,
  });
  assert.deepEqual(resolveWorkspaceTaskState([
    { status: "pending" },
    { task: "兼容旧任务字段", status: "in_progress" },
    { content: null, status: "completed" },
  ]), {
    summary: {
      completedCount: 0,
      currentStep: 1,
      hasRunningTask: true,
      summary: "兼容旧任务字段",
      totalCount: 1,
    },
    todos: [{
      content: "兼容旧任务字段",
      status: "in_progress",
    }],
  });
  assert.equal(resolveWorkspaceTaskState(null), null);
  const html = await renderWithI18n(
    React.createElement(WorkspaceTaskPanel, {
      source: {
        agentId: "researcher",
        avatar: null,
        name: "Researcher",
      },
      todos,
    }),
  );

  assert.match(html, /data-workspace-task-panel="true"/);
  assert.match(html, /data-workspace-task-trigger="true"/);
  assert.match(html, /data-workspace-task-summary="正在核对布局"/);
  assert.match(html, /data-workspace-task-agent-id="researcher"/);
  assert.match(html, /Researcher/);
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
  assert.ok(
    taskSource.indexOf("data-workspace-task-expanded-source")
      < taskSource.indexOf("data-workspace-task-progress-label"),
    "the Agent identity must lead the expanded process header",
  );
});

test("Room progress stays isolated by Agent and selection follows the latest process until chosen", async () => {
  const { projectConversationTodoProcesses } = await server.ssrLoadModule(
    "/src/features/conversation/shared/todos/todo-projection-model.ts",
  );
  const { resolveRoomTaskSelection } = await server.ssrLoadModule(
    "/src/features/conversation/room/group/chat/panel/view/room-workspace-task-model.ts",
  );
  const sessionKey = "room:conversation";
  const assistantMessage = ({
    agentId,
    content,
    index,
    roundId,
  }) => ({
    agent_id: agentId,
    content: [{
      id: `todo-${index}`,
      input: { todos: content },
      name: "TodoWrite",
      type: "tool_use",
    }],
    message_id: `message-${index}`,
    role: "assistant",
    round_id: roundId,
    session_key: sessionKey,
    timestamp: index,
  });
  const processes = projectConversationTodoProcesses([
    assistantMessage({
      agentId: "lead",
      content: [{ content: "整合结论", status: "in_progress" }],
      index: 1,
      roundId: "round-lead",
    }),
    assistantMessage({
      agentId: "researcher",
      content: [{ content: "核对来源", status: "in_progress" }],
      index: 2,
      roundId: "round-researcher",
    }),
  ], sessionKey);

  assert.deepEqual(processes.map((process) => ({
    agentId: process.agentId,
    latestTaskEventIndex: process.latestTaskEventIndex,
    todos: process.todos,
  })), [
    {
      agentId: "lead",
      latestTaskEventIndex: 0,
      todos: [{ content: "整合结论", status: "in_progress" }],
    },
    {
      agentId: "researcher",
      latestTaskEventIndex: 1,
      todos: [{ content: "核对来源", status: "in_progress" }],
    },
  ]);

  const members = [
    { agent_id: "lead", avatar: null, name: "Lead" },
    { agent_id: "researcher", avatar: null, name: "Researcher" },
    { agent_id: "analyst", avatar: null, name: "Analyst" },
  ];
  const automaticSelection = resolveRoomTaskSelection(
    processes,
    members,
    null,
  );
  assert.equal(automaticSelection.process.agentId, "researcher");
  assert.deepEqual(
    automaticSelection.members.map((member) => member.agent_id),
    ["lead", "researcher"],
  );
  assert.equal(
    resolveRoomTaskSelection(processes, members, "lead").process.agentId,
    "lead",
  );

  const roomTaskPanelSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/room/group/chat/panel/view/room-workspace-task-panel.tsx",
    ),
    "utf8",
  );
  assert.match(roomTaskPanelSource, /<RoomAgentSwitcher/);
  assert.match(roomTaskPanelSource, /variant="task"/);

  const roomAgentSwitcherSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/room/surface/room-agent-switcher.tsx",
    ),
    "utf8",
  );
  assert.match(
    roomAgentSwitcherSource,
    /variant === "panel" \? "w-28 shrink-0" : "w-full max-w-36"/,
  );
  assert.match(roomAgentSwitcherSource, /flex h-7 w-full min-w-0/);
});

test("TodoWrite normalizes persisted task aliases and rejects malformed items", async () => {
  const { projectConversationTodos } = await server.ssrLoadModule(
    "/src/features/conversation/shared/todos/todo-projection-model.ts",
  );
  const sessionKey = "agent:finance:ws:dm:legacy";
  const todos = projectConversationTodos([{
    agent_id: "finance",
    content: [{
      id: "legacy-todo-write",
      input: {
        todos: [
          {
            activeForm: " Analyzing account propagation ",
            status: "completed",
            task: " 分析压测科目变动传导至完整三张报表的解决方案 ",
          },
          {
            active_form: "编写新版需求文档",
            content: "编写新版需求文档并做好版本管理",
            status: "in_progress",
          },
          null,
          {status: "pending", task: ""},
          {status: "blocked", task: "无效状态"},
        ],
      },
      name: "TodoWrite",
      type: "tool_use",
    }],
    message_id: "legacy-assistant",
    role: "assistant",
    round_id: "legacy-round",
    session_key: sessionKey,
    timestamp: 1,
  }], sessionKey);

  assert.deepEqual(todos, [
    {
      active_form: "Analyzing account propagation",
      content: "分析压测科目变动传导至完整三张报表的解决方案",
      status: "completed",
    },
    {
      active_form: "编写新版需求文档",
      content: "编写新版需求文档并做好版本管理",
      status: "in_progress",
    },
  ]);
});

test("Room and DM stack Goal, Task, and scroll controls upward from the Composer", async () => {
  const { ConversationPanelBottomArea } = await server.ssrLoadModule(
    "/src/features/conversation/shared/conversation-panel-layout.tsx",
  );
  const stackedHtml = await renderWithI18n(
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
  assert.match(dmView, /model\.todos\.length > 0[\s\S]*<WorkspaceTaskPanel[\s\S]*source=\{model\.taskSource\}[\s\S]*todos=\{model\.todos\}/);
  assert.match(groupView, /model\.taskProcesses\.length > 0[\s\S]*<RoomWorkspaceTaskPanel[\s\S]*processes=\{model\.taskProcesses\}[\s\S]*roomMembers=\{model\.taskProcessMembers\}/);
  assert.match(groupView, /room-workspace-task-panel/);

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
  const html = await renderWithI18n(
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
  const localization = await loadI18nValue();
  assert.deepEqual(
    getReadablePermissionSuggestions(
      latest.suggestions,
      localization,
    ).map(({ label }) => label),
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
  assert.match(
    interactionHtml,
    /class="[^"]*\bradius-control-sm\b[^"]*\bw-28\b[^"]*" data-composer-permission-action="deny"/,
  );
  assert.match(
    interactionHtml,
    /class="[^"]*\bradius-control-sm\b[^"]*\bw-28\b[^"]*" data-composer-permission-action="allow"/,
  );
  assert.doesNotMatch(
    interactionHtml,
    /rounded-(?:full|l-full|r-full)/,
  );
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
    localization: await loadI18nValue(),
    status: "running",
    toolUse,
  });
  const englishModel = buildToolBlockViewModel({
    localization: await loadI18nValue("en"),
    status: "success",
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
  assert.equal(englishModel.statusText, "Completed");
  assert.equal(englishModel.toolTitle, "Write content");
  assert.deepEqual(
    buildProcessSummary({
      pendingPermissionCount: 0,
      processContent: [toolUse],
    }),
    {
      kind: "details",
      latestDetail: {
        detail: "permission_test.txt",
        kind: "tool",
        toolName: "Write",
      },
      metrics: [{ count: 1, kind: "action" }],
    },
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
  assert.match(questionHtml, /需要你的回应/);
  assert.match(questionHtml, /这次分析采用哪种研究口径？/);
  assert.match(questionHtml, /ask-user-question-option/);
  assert.match(questionHtml, /type="radio"/);
  assert.match(questionHtml, /没有合适选项？直接输入回答…/);
  assert.match(questionHtml, />拒绝</);
  assert.match(questionHtml, /继续协作/);
  assert.doesNotMatch(
    questionHtml,
    /ask-user-question-card|ask-user-question-submit|border-l-2/,
    "structured questions should stay inside one Composer surface",
  );

  const englishQuestionHtml = await renderWithI18n(
    React.createElement(ComposerInteractionSurface, {
      onResponse: () => true,
      permissions: [question],
    }),
    "en",
  );
  assert.match(englishQuestionHtml, /Needs your response/);
  assert.match(englishQuestionHtml, /No suitable option\? Type your answer…/);
  assert.match(englishQuestionHtml, />Deny</);
  assert.match(englishQuestionHtml, />Continue</);

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
  assert.match(
    planHtml,
    /class="[^"]*\bradius-control-sm\b[^"]*\bw-24\b[^"]*" data-composer-permission-action="deny"/,
  );
  assert.match(
    planHtml,
    /class="[^"]*\bradius-control-sm\b[^"]*\bw-24\b[^"]*" data-composer-permission-action="allow"/,
  );
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
  assert.equal(
    resolvePendingInteractionOwner("room_thread_process"),
    "composer",
  );

  const toolElement = React.createElement(ContentRenderer, {
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
  });
  const toolHtml = await renderWithI18n(toolElement);
  assert.match(toolHtml, /待确认/);
  assert.match(toolHtml, /echo protected/);
  assert.match(toolHtml, /surface-muted-background/);
  assert.doesNotMatch(toolHtml, /--warning|animate-pulse/);
  assert.doesNotMatch(toolHtml, />允许</);
  assert.doesNotMatch(toolHtml, />拒绝</);
  assert.doesNotMatch(toolHtml, /data-human-interaction-surface/);
  const englishToolHtml = await renderWithI18n(toolElement, "en");
  assert.match(englishToolHtml, /Run command/);
  assert.match(englishToolHtml, /Waiting for approval/);
  assert.doesNotMatch(englishToolHtml, /执行命令|待确认/);

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
    { label: "等待确认", state: "waiting_permission" },
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
