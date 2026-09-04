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

test("Goal ACK 未知时显示确认中而不是伪成功", async () => {
  const { projectComposerFooterStatus } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/components/footer/composer-footer-model.ts",
  );
  const projection = projectComposerFooterStatus({
    activeError: null,
    copy: {
      compacting: "正在压缩上下文",
      goalCreating: "正在创建 Goal",
      goalConfirming: "正在确认目标是否已受理",
      preparingAttachments: "正在准备附件",
      replying: "回复中",
      sending: "发送中",
      stopHint: "[ESC 停止]",
    },
    isGoalConfirming: true,
    isGoalCreating: true,
    isPreparingAttachments: false,
    runtimeActivity: null,
  });
  assert.equal(projection.message, "正在确认目标是否已受理");
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

test("terminal task notifications settle status without replacing task identity", async () => {
  const { projectConversationTodos } = await server.ssrLoadModule(
    "/src/features/conversation/shared/todos/todo-projection-model.ts",
  );
  const sessionKey = "agent:devin:ws:room:legacy";
  const longResult = "完整子 Agent 结果不属于任务标题。\n".repeat(800);
  const todos = projectConversationTodos([
    {
      agent_id: "devin",
      content: "核对并发模型",
      message_id: "task-started",
      metadata: {
        description: "核对并发模型",
        subtype: "task_started",
        task_id: "task-1",
      },
      role: "system",
      round_id: "round-1",
      session_key: sessionKey,
      timestamp: 1,
    },
    {
      agent_id: "devin",
      content: "任务状态已更新",
      message_id: "task-status-updated",
      metadata: {
        patch: { status: "in_progress" },
        subtype: "task_updated",
        task_id: "task-1",
      },
      role: "system",
      round_id: "round-1",
      session_key: sessionKey,
      timestamp: 2,
    },
    {
      agent_id: "devin",
      content: "任务状态已更新",
      message_id: "task-description-updated",
      metadata: {
        patch: {
          description: "核对协同调度",
          status: "in_progress",
        },
        subtype: "task_updated",
        task_id: "task-1",
      },
      role: "system",
      round_id: "round-1",
      session_key: sessionKey,
      timestamp: 3,
    },
    {
      agent_id: "devin",
      content: longResult,
      message_id: "task-notification",
      metadata: {
        status: "completed",
        subtype: "task_notification",
        summary: "Agent 调度调研已完成",
        task_id: "task-1",
      },
      role: "system",
      round_id: "round-1",
      session_key: sessionKey,
      timestamp: 4,
    },
    {
      agent_id: "devin",
      content: longResult,
      message_id: "orphan-task-notification",
      metadata: {
        status: "completed",
        subtype: "task_notification",
        summary: "孤立子任务已完成",
        task_id: "task-2",
      },
      role: "system",
      round_id: "round-1",
      session_key: sessionKey,
      timestamp: 5,
    },
  ], sessionKey);

  assert.deepEqual(todos, [
    {
      active_form: undefined,
      content: "核对协同调度",
      status: "completed",
    },
    {
      active_form: undefined,
      content: "孤立子任务已完成",
      status: "completed",
    },
  ]);
});

test("a new conversation round hides the previous successful TodoWrite plan", async () => {
  const { projectConversationTodos } = await server.ssrLoadModule(
    "/src/features/conversation/shared/todos/todo-projection-model.ts",
  );
  const sessionKey = "agent:lucy:ws:dm:slides";
  const todos = projectConversationTodos([
    {
      agent_id: "lucy",
      content: [{
        id: "old-todo-write",
        input: {
          todos: [{
            activeForm: "构建文档",
            content: "构建 PDF 并验证",
            status: "completed",
          }],
        },
        name: "TodoWrite",
        type: "tool_use",
      }],
      message_id: "old-assistant",
      result_summary: { is_error: false },
      role: "assistant",
      round_id: "old-round",
      session_key: sessionKey,
      timestamp: 1,
    },
    {
      agent_id: "lucy",
      content: "用 PPT 试试",
      message_id: "new-user",
      role: "user",
      round_id: "new-round",
      session_key: sessionKey,
      timestamp: 2,
    },
  ], sessionKey);

  assert.deepEqual(todos, []);
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
  const denyActionTag = planHtml.match(
    /<button[^>]*data-composer-permission-action="deny"[^>]*>/,
  )?.[0] ?? "";
  const allowActionTag = planHtml.match(
    /<div[^>]*data-composer-permission-action="allow"[^>]*>/,
  )?.[0] ?? "";
  assert.match(denyActionTag, /\bradius-control-sm\b/);
  assert.match(denyActionTag, /\bw-24\b/);
  assert.match(allowActionTag, /data-slot="split-button"/);
  assert.match(allowActionTag, /\bradius-control-sm\b/);
  assert.match(allowActionTag, /\bw-24\b/);
});
