// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

async function loadI18nValue(locale = "zh") {
  const { MESSAGES } = await import("@/shared/i18n/messages.ts");
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
  const { I18N_CONTEXT } = await import("@/shared/i18n/i18n-context.ts");
  const value = await loadI18nValue(locale);
  return renderToStaticMarkup(
    React.createElement(
      I18N_CONTEXT.Provider,
      { value },
      element,
    ),
  );
}

test("Goal ACK 未知时显示确认中而不是伪成功", async () => {
  const { projectComposerFooterStatus } = await import("@/features/conversation/shared/composer/components/footer/composer-footer-model.ts");
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

test("DM Composer keeps direct Session permission and model controls", async () => {
  const { ComposerSessionControls } = await import("@/features/conversation/shared/composer/components/footer/composer-session-controls.tsx");
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
    effort: "",
    effortLevels: [],
    ensureTargetsLoaded: () => {},
    error: null,
    hasModelOverride: false,
    inheritedModel: "agent-model",
    inheritedPermissionMode: "default",
    inheritedProvider: "agent-provider",
    isDangerousPermission: false,
    modelBusy: false,
    modelLabel: "agent-model",
    permissionLabel: "默认",
    providerOptions: null,
    resetModel: () => {},
    resetTarget: () => {},
    updateEffort: () => {},
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
  const { projectConversationTodos } = await import("@/features/conversation/shared/todos/todo-projection-model.ts");
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
  const { projectConversationTodos } = await import("@/features/conversation/shared/todos/todo-projection-model.ts");
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
  const { projectConversationTodos } = await import("@/features/conversation/shared/todos/todo-projection-model.ts");
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
