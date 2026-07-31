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

const execution = {
  id: "execution-1",
  session_key: "room:conversation-1",
  scope_kind: "room",
  coordinator_agent_id: "lead",
  objective: "完成 WorkGraph UI",
  completion_criteria: ["全部必需工作项通过验收"],
  status: "active",
  version: 8,
  plan: {
    id: "plan-1",
    revision: 2,
    status: "active",
    created_at: "2026-07-31T10:00:00Z",
  },
  progress: {
    total: 3,
    required: 3,
    accepted: 1,
    running: 1,
    blocked: 0,
    submitted: 0,
    ready: 0,
    waiting: 1,
    changes_requested: 0,
    failed: 0,
    cancelled: 0,
  },
  work_items: [
    {
      id: "research",
      logical_key: "research",
      kind: "produce",
      subject: "梳理协议",
      objective: "定义 WorkGraph",
      deliverable: "协议文档",
      acceptance_criteria: ["边界完整"],
      required: true,
      position: 0,
      status: "accepted",
      owner_agent_id: "researcher",
      updated_at: "2026-07-31T10:00:00Z",
    },
    {
      id: "build",
      logical_key: "build",
      kind: "produce",
      subject: "实现 UI",
      objective: "接入 DM 与 Room",
      deliverable: "WorkGraph 面板",
      acceptance_criteria: ["Typecheck 通过"],
      dependency_ids: ["research"],
      required: true,
      position: 1,
      status: "running",
      owner_agent_id: "builder",
      attempts: [
        {
          id: "attempt-child",
          assignment_id: "assignment-build",
          parent_attempt_id: "attempt-root",
          executor_kind: "subagent",
          status: "running",
          created_at: "2026-07-31T10:01:00Z",
        },
      ],
      updated_at: "2026-07-31T10:01:00Z",
    },
    {
      id: "integrate",
      logical_key: "integrate",
      kind: "integrate",
      subject: "验收整合",
      objective: "完成闭环",
      deliverable: "可发布版本",
      acceptance_criteria: ["依赖均通过"],
      dependency_ids: ["build"],
      required: true,
      terminal: true,
      position: 2,
      status: "waiting",
      updated_at: "2026-07-31T10:01:00Z",
    },
  ],
  created_at: "2026-07-31T10:00:00Z",
  updated_at: "2026-07-31T10:01:00Z",
};

const directory = {
  lead: { avatar: null, id: "lead", name: "Lead" },
  researcher: { avatar: null, id: "researcher", name: "Researcher" },
  builder: { avatar: null, id: "builder", name: "Builder" },
};

test("WorkGraph model keeps dependency depth and delivery summary", async () => {
  const {
    buildExecutionSummaryParts,
    resolveSelectedWorkItem,
    resolveWorkItemDepths,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-process-model.ts",
  );
  assert.deepEqual(resolveWorkItemDepths(execution), {
    research: 0,
    build: 1,
    integrate: 2,
  });
  assert.equal(resolveSelectedWorkItem(execution, null).id, "build");
  assert.deepEqual(buildExecutionSummaryParts(execution), [
    { count: 1, key: "execution.summary_running" },
  ]);
});

test("WorkGraph panel exposes one authoritative compact progress surface", async () => {
  const { ExecutionProcessPanel } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-process-panel.tsx",
  );
  const html = await renderWithI18n(
    React.createElement(ExecutionProcessPanel, {
      directory,
      execution,
      onDismiss: () => {},
    }),
  );
  assert.match(html, /data-execution-process-panel/);
  assert.match(html, /data-execution-status="active"/);
  assert.match(html, /1\/3 已验收/);
  assert.match(html, /Lead/);
  assert.doesNotMatch(html, /data-workspace-task-panel/);
});

test("Work Item detail includes dependency, contract, subagent and completion boundaries", async () => {
  const { ExecutionWorkItemDetail } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-work-item-detail.tsx",
  );
  const html = await renderWithI18n(
    React.createElement(ExecutionWorkItemDetail, {
      directory,
      execution,
      item: execution.work_items[1],
    }),
  );
  assert.match(html, /实现 UI/);
  assert.match(html, /梳理协议/);
  assert.match(html, /WorkGraph 面板/);
  assert.match(html, /子智能体/);
  assert.match(html, /全部必需工作项通过验收/);
});

test("Execution MCP names render as semantic activity instead of raw transport names", async () => {
  const { getToolInputSummary, getToolTitle } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/tool-activity.ts",
  );
  assert.equal(
    getToolTitle("mcp__nexus_execution__plan_execution"),
    "建立执行计划",
  );
  assert.equal(
    getToolInputSummary({ objective: "完成前后端闭环" }),
    "完成前后端闭环",
  );
});
