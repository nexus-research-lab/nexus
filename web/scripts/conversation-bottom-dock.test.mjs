import assert from "node:assert/strict";
import fs from "node:fs/promises";
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
  assert.match(html, /class="h-4 w-4 -rotate-90"/);
  assert.doesNotMatch(html, /class="h-5 w-5 -rotate-90"/);
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

test("Goal Composer separates lead control, runtime status, and submit rows", async () => {
  const css = await fs.readFile(
    path.join(webRoot, "src/app/styles/theme-recipes.css"),
    "utf8",
  );
  const goalLayoutStart = css.indexOf(
    '.nexus-chat-composer-footer[data-goal-mode="true"] {',
  );
  const narrowLayoutStart = css.indexOf(
    "@container nexus-chat-composer (max-width: 460px)",
  );
  assert.notEqual(goalLayoutStart, -1);
  assert.notEqual(narrowLayoutStart, -1);

  const goalLayout = css.slice(goalLayoutStart, narrowLayoutStart);
  assert.match(
    goalLayout,
    /grid-template-columns:\s*minmax\(0, 1fr\) auto;/,
  );
  assert.match(goalLayout, /grid-template-areas:\s*"leading trailing";/);
  assert.match(goalLayout, /align-items:\s*start;/);
  assert.match(
    goalLayout,
    /\.nexus-chat-composer-footer-leading\s*\{[\s\S]*?flex-wrap:\s*wrap;/,
  );
  assert.match(
    goalLayout,
    /\.nexus-chat-composer-runtime-status\s*\{[\s\S]*?flex-basis:\s*100%;/,
  );
  assert.match(
    goalLayout,
    /\.nexus-chat-composer-footer-brand\s*\{[\s\S]*?display:\s*none;/,
  );

  const narrowLayout = css.slice(
    narrowLayoutStart,
    css.indexOf(".input-shell.ui-search-input-shell", narrowLayoutStart),
  );
  assert.match(
    narrowLayout,
    /grid-template-areas:\s*"leading"\s*"trailing";/,
  );
  assert.match(
    narrowLayout,
    /\.nexus-chat-composer-submit\s*\{[\s\S]*?margin-inline-start:\s*auto;/,
  );
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

test("Agent launches render as compact clickable task entries without redundant current-Agent text", async () => {
  const { ContentRenderer } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/item/view/content/content-renderer.tsx",
  );
  const content = [
    {
      id: "tool-agent-timeline",
      input: {
        description: "调研俄乌战争当前态势和时间线",
        subagent_type: "research",
      },
      name: "Agent",
      type: "tool_use",
    },
    {
      description: "调研俄乌战争当前态势和时间线",
      last_tool_name: "Agent",
      task_id: "task-timeline",
      tool_use_id: "tool-agent-timeline",
      type: "task_progress",
    },
    {
      id: "tool-agent-economy",
      input: {
        description: "调研俄乌战争经济影响与技术演变",
        subagent_type: "research",
      },
      name: "Task",
      type: "tool_use",
    },
    {
      description: "调研俄乌战争经济影响与技术演变",
      last_tool_name: "Agent",
      task_id: "task-economy",
      tool_use_id: "tool-agent-economy",
      type: "task_progress",
    },
  ];
  const html = await renderWithI18n(
    React.createElement(ContentRenderer, {
      content,
      isStreaming: true,
      onOpenSubagentTask: () => {},
      workspaceAgentId: "agent-lucy",
    }),
  );

  assert.equal(
    (html.match(/data-subagent-task-tool-group="true"/g) ?? []).length,
    1,
  );
  assert.equal(
    (html.match(/data-subagent-task-tool-entry="true"/g) ?? []).length,
    2,
  );
  assert.equal(
    (html.match(/data-subagent-task-avatar="true"/g) ?? []).length,
    2,
  );
  assert.equal(
    (html.match(/data-subagent-task-status=/g) ?? []).length,
    2,
  );
  assert.equal((html.match(/\bw-60\b/g) ?? []).length, 2);
  assert.match(html, /调研俄乌战争当前态势和时间线/);
  assert.match(html, /调研俄乌战争经济影响与技术演变/);
  assert.match(html, /<button/);
  assert.doesNotMatch(html, /当前 Agent/);
  assert.doesNotMatch(html, /bg-primary\/5/);
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
