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

test("WorkGraph model keeps dependency depth and current node summary", async () => {
  const {
    resolveExecutionNodeSummary,
    resolveExecutionNodeWindow,
    resolveWorkItemDepths,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-process-model.ts",
  );
  assert.deepEqual(resolveWorkItemDepths(execution), {
    research: 0,
    build: 1,
    integrate: 2,
  });
  assert.deepEqual(resolveExecutionNodeSummary(execution), {
    current: execution.work_items[1],
    currentStep: 2,
    summary: "实现 UI",
    totalCount: 3,
  });
  assert.deepEqual(resolveExecutionNodeWindow(execution, "build"), {
    hiddenAfter: 0,
    hiddenBefore: 0,
    items: execution.work_items,
  });
});

test("WorkGraph layout reflows when Plan nodes are added or removed", async () => {
  const { buildExecutionGraphLayout } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-workgraph-layout.ts",
  );
  const branched = structuredClone(execution);
  branched.version += 1;
  branched.work_items.splice(2, 0, {
    id: "review",
    logical_key: "review",
    kind: "review",
    subject: "并行复核",
    objective: "复核实现",
    deliverable: "复核结论",
    acceptance_criteria: ["结论明确"],
    dependency_ids: ["research"],
    required: true,
    position: 2,
    status: "ready",
    owner_agent_id: "researcher",
    updated_at: "2026-07-31T10:02:00Z",
  });
  branched.work_items[3].dependency_ids = ["build", "review"];
  branched.work_items[3].position = 3;

  const addedLayout = buildExecutionGraphLayout(branched);
  assert.equal(addedLayout.nodes.length, 4);
  assert.deepEqual(
    addedLayout.edges.map((edge) => `${edge.sourceId}->${edge.targetId}`),
    ["research->build", "research->review", "build->integrate", "review->integrate"],
  );
  assert.equal(
    addedLayout.nodes.find((node) => node.item.id === "build").x,
    addedLayout.nodes.find((node) => node.item.id === "review").x,
  );
  assert.notEqual(
    addedLayout.nodes.find((node) => node.item.id === "build").y,
    addedLayout.nodes.find((node) => node.item.id === "review").y,
  );

  const reduced = structuredClone(branched);
  reduced.version += 1;
  reduced.work_items = reduced.work_items.filter((item) => item.id !== "build");
  reduced.work_items.find((item) => item.id === "integrate").dependency_ids = ["review"];
  const reducedLayout = buildExecutionGraphLayout(reduced);
  assert.equal(reducedLayout.nodes.length, 3);
  assert.deepEqual(
    reducedLayout.edges.map((edge) => `${edge.sourceId}->${edge.targetId}`),
    ["research->review", "review->integrate"],
  );
});

test("WorkGraph panel follows Task density and exposes the current node rail", async () => {
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
  assert.match(html, /data-execution-node-rail/);
  assert.match(html, /data-execution-node-agent="researcher"/);
  assert.match(html, /data-execution-node-agent="builder"/);
  assert.match(html, /data-execution-node-connection/);
  assert.match(html, /第 2 \/ 3 节点/);
  assert.match(html, /实现 UI/);
  assert.doesNotMatch(html, /Lead/);
  assert.doesNotMatch(html, /data-workspace-task-panel/);
});

test("Expanded WorkGraph is an interactive Agent-avatar DAG", async () => {
  const { ExecutionWorkGraphCanvas } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-workgraph-canvas.tsx",
  );
  const html = await renderWithI18n(
    React.createElement(ExecutionWorkGraphCanvas, {
      currentId: "build",
      directory,
      execution,
    }),
  );
  assert.match(html, /data-execution-node-map/);
  assert.match(html, /data-execution-workgraph-canvas/);
  assert.match(html, /data-execution-node-detail-mode="popover"/);
  assert.match(html, /data-execution-edge-layer/);
  assert.match(html, /data-execution-edge-source="research"/);
  assert.match(html, /data-execution-edge-target="build"/);
  assert.match(html, /data-execution-current-node="true"/);
  assert.doesNotMatch(html, /data-execution-node-selected="true"/);
  assert.doesNotMatch(html, /data-execution-selected-node-detail/);
  assert.match(html, /data-execution-node-agent="researcher"/);
  assert.match(html, /data-execution-node-agent="builder"/);
  assert.doesNotMatch(html, /验收标准/);
  assert.doesNotMatch(html, /依赖.*1/);
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
