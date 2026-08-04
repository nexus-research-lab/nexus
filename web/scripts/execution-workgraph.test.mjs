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
  graph: {
    nodes: [
      {
        id: "research",
        kind: "agent",
        visibility: "primary",
        work_item_id: "research",
        agent_id: "researcher",
        responsibility_status: "accepted",
        position: 0,
      },
      {
        id: "build",
        kind: "agent",
        visibility: "primary",
        work_item_id: "build",
        attempt_id: "attempt-root",
        agent_id: "builder",
        agent_round_id: "agent-round-build-1",
        responsibility_status: "running",
        run_status: "running",
        position: 1,
      },
      {
        id: "attempt-child",
        kind: "subagent",
        visibility: "nested",
        work_item_id: "build",
        attempt_id: "attempt-child",
        parent_node_id: "build",
        subject_id: "sdk-task-child",
        name: "Research helper",
        run_status: "running",
        position: 1,
      },
      {
        id: "integrate",
        kind: "agent",
        visibility: "primary",
        work_item_id: "integrate",
        responsibility_status: "waiting",
        position: 2,
      },
    ],
    edges: [
      {
        id: "dependency:research:build",
        kind: "dependency",
        source_node_id: "research",
        target_node_id: "build",
      },
      {
        id: "spawn:build:attempt-child",
        kind: "spawn",
        source_node_id: "build",
        target_node_id: "attempt-child",
      },
      {
        id: "dependency:build:integrate",
        kind: "dependency",
        source_node_id: "build",
        target_node_id: "integrate",
      },
    ],
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
          id: "attempt-root",
          assignment_id: "assignment-build",
          executor_kind: "agent",
          executor_agent_id: "builder",
          agent_round_id: "agent-round-build-1",
          status: "running",
          created_at: "2026-07-31T10:00:30Z",
        },
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
    compactExecutionNodeObjective,
    hasManagedExecutionGraph,
    isExecutionActivityVisible,
    resolveExecutionPrimaryAgentNodes,
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
  assert.equal(hasManagedExecutionGraph(execution), true);
  assert.equal(isExecutionActivityVisible(execution), true);
  assert.deepEqual(
    resolveExecutionPrimaryAgentNodes(execution).map((node) => node.id),
    ["research", "build"],
  );
  const completed = structuredClone(execution);
  completed.status = "completed";
  assert.equal(isExecutionActivityVisible(completed), false);
  assert.deepEqual(resolveExecutionNodeSummary(execution), {
    current: execution.work_items[1],
    currentNode: execution.graph.nodes[2],
    currentStep: 2,
    summary: "实现 UI",
    totalCount: 3,
  });
  assert.deepEqual(resolveExecutionNodeWindow(execution, "build"), {
    hiddenAfter: 0,
    hiddenBefore: 0,
    items: execution.work_items,
  });
  assert.equal(
    compactExecutionNodeObjective(
      "Researcher 收集与 Room 工作图相关的公开资料",
      "Researcher",
    ),
    "收集与 Room 工作图相关的公开资料",
  );
  assert.equal(
    compactExecutionNodeObjective("Researcher-led source review", "Researcher"),
    "Researcher-led source review",
  );
  assert.equal(
    compactExecutionNodeObjective("Researcher - source review", "Researcher"),
    "source review",
  );

  const contained = structuredClone(execution);
  contained.work_items[2].dependency_ids = [];
  contained.work_items[2].parent_work_item_id = "build";
  assert.equal(
    resolveWorkItemDepths(contained).integrate,
    0,
    "Work Item containment must not become a readiness/layout dependency",
  );
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
  branched.graph.nodes.splice(3, 0, {
    id: "review",
    kind: "agent",
    visibility: "primary",
    work_item_id: "review",
    agent_id: "researcher",
    responsibility_status: "ready",
    position: 2,
  });
  branched.graph.nodes.find((node) => node.id === "integrate").position = 3;
  branched.graph.edges = [
    {
      id: "dependency:research:build",
      kind: "dependency",
      source_node_id: "research",
      target_node_id: "build",
    },
    {
      id: "spawn:build:attempt-child",
      kind: "spawn",
      source_node_id: "build",
      target_node_id: "attempt-child",
    },
    {
      id: "dependency:research:review",
      kind: "dependency",
      source_node_id: "research",
      target_node_id: "review",
    },
    {
      id: "dependency:build:integrate",
      kind: "dependency",
      source_node_id: "build",
      target_node_id: "integrate",
    },
    {
      id: "dependency:review:integrate",
      kind: "dependency",
      source_node_id: "review",
      target_node_id: "integrate",
    },
  ];

  const addedLayout = buildExecutionGraphLayout(branched);
  assert.equal(addedLayout.nodes.length, 5);
  assert.deepEqual(
    addedLayout.edges.map((edge) => `${edge.sourceId}->${edge.targetId}`),
    [
      "research->build",
      "build->attempt-child",
      "research->review",
      "build->integrate",
      "review->integrate",
    ],
  );
  assert.equal(
    addedLayout.nodes.find((node) => node.node.id === "build").x,
    addedLayout.nodes.find((node) => node.node.id === "review").x,
  );
  assert.notEqual(
    addedLayout.nodes.find((node) => node.node.id === "build").y,
    addedLayout.nodes.find((node) => node.node.id === "review").y,
  );

  const reduced = structuredClone(branched);
  reduced.version += 1;
  reduced.work_items = reduced.work_items.filter((item) => item.id !== "build");
  reduced.work_items.find((item) => item.id === "integrate").dependency_ids = ["review"];
  reduced.graph.nodes = reduced.graph.nodes.filter((node) => (
    node.work_item_id !== "build"
  ));
  reduced.graph.edges = [
    {
      id: "dependency:research:review",
      kind: "dependency",
      source_node_id: "research",
      target_node_id: "review",
    },
    {
      id: "dependency:review:integrate",
      kind: "dependency",
      source_node_id: "review",
      target_node_id: "integrate",
    },
  ];
  const reducedLayout = buildExecutionGraphLayout(reduced);
  assert.equal(reducedLayout.nodes.length, 3);
  assert.deepEqual(
    reducedLayout.edges.map((edge) => `${edge.sourceId}->${edge.targetId}`),
    ["research->review", "review->integrate"],
  );

  const constrainedLayout = buildExecutionGraphLayout(execution, 340);
  assert.equal(constrainedLayout.width, 340);
  assert.ok(
    constrainedLayout.nodes[1].x > constrainedLayout.nodes[0].x,
    "the main responsibility chain remains left-to-right after clustering",
  );
});

test("Planless runtime graph promotes active tools and keeps ordinary tools in detail", async () => {
  const {
    hasManagedExecutionGraph,
    resolveExecutionNodeSummary,
    orderedExecutionGraphNodes,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-process-model.ts",
  );
  const { buildExecutionGraphLayout } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-workgraph-layout.ts",
  );
  const runtimeExecution = {
    id: "round:round-1",
    session_key: "agent:nexus:workspace:dm:1",
    objective: "",
    status: "active",
    version: 1,
    progress: {
      total: 0,
      required: 0,
      accepted: 0,
      running: 0,
      blocked: 0,
      submitted: 0,
      ready: 0,
      waiting: 0,
      changes_requested: 0,
      failed: 0,
      cancelled: 0,
    },
    graph: {
      nodes: [
        {
          id: "agent-run-1",
          kind: "agent",
          visibility: "primary",
          work_item_id: "",
          agent_id: "builder",
          agent_round_id: "agent-round-1",
          subject_id: "agent-round-1",
          name: "agent",
          lifecycle_status: "running",
          position: 0,
        },
        {
          id: "tool-run-1",
          kind: "tool",
          visibility: "nested",
          work_item_id: "",
          parent_node_id: "agent-run-1",
          subject_id: "tool-1",
          name: "search",
          lifecycle_status: "running",
          position: 1,
        },
        {
          id: "tool-run-2",
          kind: "tool",
          visibility: "detail",
          work_item_id: "",
          parent_node_id: "agent-run-1",
          subject_id: "tool-2",
          name: "read_file",
          lifecycle_status: "succeeded",
          position: 2,
        },
      ],
      edges: [
        {
          id: "invoke-1",
          kind: "invoke",
          source_node_id: "agent-run-1",
          target_node_id: "tool-run-1",
        },
        {
          id: "invoke-2",
          kind: "invoke",
          source_node_id: "agent-run-1",
          target_node_id: "tool-run-2",
        },
      ],
    },
    work_items: [],
    created_at: "2026-08-03T10:00:00Z",
    updated_at: "2026-08-03T10:00:01Z",
  };

  assert.equal(hasManagedExecutionGraph(runtimeExecution), false);

  assert.deepEqual(
    orderedExecutionGraphNodes(runtimeExecution).map((node) => node.id),
    ["agent-run-1", "tool-run-1"],
  );
  const summary = resolveExecutionNodeSummary(runtimeExecution);
  assert.equal(summary.currentNode.id, "tool-run-1");
  assert.equal(summary.currentStep, 2);
  assert.equal(summary.totalCount, 2);
  assert.equal(summary.summary, "search");
  const layout = buildExecutionGraphLayout(runtimeExecution);
  assert.equal(layout.nodes.length, 2);
  assert.equal(layout.groups.length, 1);
  assert.deepEqual(layout.groups[0].nodeIds, ["agent-run-1", "tool-run-1"]);
  assert.deepEqual(
    layout.edges.map((edge) => `${edge.kind}:${edge.sourceId}->${edge.targetId}`),
    ["invoke:agent-run-1->tool-run-1"],
  );
  assert.ok(
    layout.nodes.find((node) => node.node.id === "tool-run-1").y
      > layout.nodes.find((node) => node.node.id === "agent-run-1").y,
    "runtime children expand below their owning Agent",
  );

  const missingEdge = structuredClone(runtimeExecution);
  missingEdge.graph.edges = [];
  const repairedLayout = buildExecutionGraphLayout(missingEdge);
  assert.deepEqual(
    repairedLayout.edges.map((edge) => `${edge.kind}:${edge.sourceId}->${edge.targetId}`),
    ["invoke:agent-run-1->tool-run-1"],
    "a visible child with durable parent identity never becomes an orphan icon",
  );
});

test("Lead review gate is a visible node and changes-requested is a back edge", async () => {
  const { buildExecutionGraphLayout } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-workgraph-layout.ts",
  );
  const reviewed = structuredClone(execution);
  reviewed.graph.nodes = [
    reviewed.graph.nodes[1],
    {
      id: "review:assignment-build",
      kind: "gate",
      visibility: "primary",
      work_item_id: "build",
      agent_id: "lead",
      subject_id: "assignment-build",
      name: "review",
      lifecycle_status: "changes_requested",
      position: 1,
    },
  ];
  reviewed.graph.edges = [
    {
      id: "review-edge",
      kind: "review",
      source_node_id: "build",
      target_node_id: "review:assignment-build",
    },
    {
      id: "loop-edge",
      kind: "loop_back",
      source_node_id: "review:assignment-build",
      target_node_id: "build",
    },
  ];
  const layout = buildExecutionGraphLayout(reviewed);
  assert.equal(layout.nodes.length, 2);
  assert.equal(
    layout.nodes.find((node) => node.node.kind === "gate").node.agent_id,
    "lead",
  );
  assert.deepEqual(layout.edges.map((edge) => edge.kind), ["review", "loop_back"]);
  assert.ok(
    layout.nodes.find((node) => node.node.kind === "gate").x
      > layout.nodes.find((node) => node.node.kind === "agent").x,
  );
});

test("Objective alignment gate reports evidence without choosing the Agent route", async () => {
  const { resolveExecutionGraphNodeStatus } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-process-model.ts",
  );
  const { buildExecutionGraphLayout } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-workgraph-layout.ts",
  );
  const alignment = structuredClone(execution);
  alignment.graph.nodes = [
    {
      id: "agent-run-alignment",
      kind: "agent",
      visibility: "primary",
      work_item_id: "",
      agent_id: "lead",
      subject_id: "agent-round-alignment",
      lifecycle_status: "running",
      position: 0,
    },
    {
      id: "gate-alignment",
      kind: "gate",
      visibility: "primary",
      work_item_id: "",
      parent_node_id: "agent-run-alignment",
      agent_id: "lead",
      subject_id: "tool-alignment",
      name: "objective_alignment",
      description: "Verification is still missing.",
      lifecycle_status: "not_aligned",
      position: 1,
    },
  ];
  alignment.graph.edges = [
    {
      id: "guard-edge",
      kind: "guard",
      source_node_id: "agent-run-alignment",
      target_node_id: "gate-alignment",
    },
    {
      id: "alignment-return",
      kind: "loop_back",
      source_node_id: "gate-alignment",
      target_node_id: "agent-run-alignment",
    },
  ];
  alignment.work_items = [];

  assert.equal(
    resolveExecutionGraphNodeStatus(alignment.graph.nodes[1], null),
    "changes_requested",
  );
  const layout = buildExecutionGraphLayout(alignment);
  assert.deepEqual(layout.edges.map((edge) => edge.kind), ["guard", "loop_back"]);
  assert.ok(
    layout.nodes.find((node) => node.node.kind === "gate").x
      > layout.nodes.find((node) => node.node.kind === "agent").x,
  );
});

test("WorkGraph node Task uses exact Agent round correlation", async () => {
  const { resolveExecutionNodeTaskRun } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-node-task-model.ts",
  );
  const { ExecutionNodeTaskList } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-node-task-list.tsx",
  );
  const run = {
    agentId: "builder",
    agentRoundId: "agent-round-build-1",
    latestTaskEventIndex: 8,
    todos: [
      { content: "确认协议", status: "completed" },
      { content: "实现节点", status: "completed" },
      { content: "接入进程", status: "in_progress", active_form: "正在接入进程" },
      { content: "补充测试", status: "pending" },
      { content: "运行检查", status: "pending" },
      { content: "整理结果", status: "pending" },
    ],
  };
  const buildItem = execution.work_items.find((item) => item.id === "build");
  assert.equal(resolveExecutionNodeTaskRun(buildItem, [run]), run);
  assert.equal(resolveExecutionNodeTaskRun(buildItem, [{
    ...run,
    agentRoundId: "another-agent-round",
  }]), null);
  assert.equal(resolveExecutionNodeTaskRun(buildItem, [{
    ...run,
    agentId: "another-agent",
  }]), null);
  assert.equal(resolveExecutionNodeTaskRun({
    ...buildItem,
    attempts: [
      ...buildItem.attempts,
      {
        id: "attempt-retry",
        assignment_id: "assignment-build",
        executor_kind: "agent",
        executor_agent_id: "builder",
        agent_round_id: "agent-round-build-2",
        status: "running",
        created_at: "2026-07-31T10:02:00Z",
      },
    ],
  }, [run]), null);

  const html = await renderWithI18n(
    React.createElement(ExecutionNodeTaskList, { run }),
  );
  assert.match(html, /data-execution-node-tasks/);
  assert.match(html, /data-execution-node-task-agent-round="agent-round-build-1"/);
  assert.match(html, /正在接入进程/);
  assert.match(html, /另有 1 步/);
  assert.doesNotMatch(html, /整理结果/);
});

test("Composer WorkGraph dock exposes only primary Agent activity", async () => {
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
  assert.match(html, /data-execution-agent-activity-dock/);
  assert.match(html, /data-execution-open-workgraph/);
  assert.match(html, /data-execution-agent-activity="researcher"/);
  assert.match(html, /data-execution-agent-activity="builder"/);
  assert.match(html, /data-execution-agent-live="true"/);
  assert.match(html, /data-execution-agent-round-id="agent-round-build-1"/);
  assert.match(html, /data-execution-node-agent="researcher"/);
  assert.match(html, /data-execution-node-agent="builder"/);
  assert.doesNotMatch(html, /data-execution-node-agent="subagent:sdk-task-child"/);
  assert.match(html, /data-execution-agent-connection/);
  assert.doesNotMatch(html, /data-execution-node-connection/);
  assert.ok(
    html.lastIndexOf("data-execution-agent-activity")
      < html.indexOf("data-execution-open-workgraph"),
    "the WorkGraph control should follow the connected Agent activity rail",
  );
  assert.doesNotMatch(html, /data-execution-node-kind="tool"/);
  assert.match(html, /第 2 \/ 3 节点/);
  assert.match(html, /实现 UI/);
  assert.doesNotMatch(html, /Lead/);
  assert.doesNotMatch(html, /data-workspace-task-panel/);

  const panelSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/shared/execution/execution-process-panel.tsx",
    ),
    "utf8",
  );
  assert.match(panelSource, /max-w-\[460px\]/);
  assert.doesNotMatch(panelSource, /ExecutionWorkGraphCanvas/);
  assert.doesNotMatch(panelSource, /ANCHORED_OVERLAY_MOTION_CLASS_NAME/);
});

test("Full WorkGraph is an interactive Agent-avatar DAG with a task inspector", async () => {
  const { ExecutionWorkGraphCanvas } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-workgraph-canvas.tsx",
  );
  const html = await renderWithI18n(
    React.createElement(ExecutionWorkGraphCanvas, {
      currentId: "attempt-child",
      directory,
      execution,
      taskRuns: [],
    }),
  );
  assert.match(html, /data-execution-node-map/);
  assert.match(html, /data-execution-workgraph-canvas/);
  assert.match(html, /data-execution-board-grid/);
  assert.match(html, /data-execution-node-detail-mode="inspector"/);
  assert.match(html, /data-execution-edge-layer/);
  assert.match(html, /data-execution-edge-source="research"/);
  assert.match(html, /data-execution-edge-target="build"/);
  assert.match(html, /data-execution-edge-kind="spawn"/);
  assert.match(html, /data-execution-edge-target="attempt-child"/);
  assert.match(html, /data-execution-subgraph-root="build"/);
  assert.match(html, /data-execution-current-node="true"/);
  assert.doesNotMatch(html, /data-execution-node-selected="true"/);
  assert.doesNotMatch(html, /data-execution-selected-node-detail/);
  assert.match(html, /data-execution-node-agent="researcher"/);
  assert.match(html, /data-execution-node-agent="builder"/);
  assert.match(html, /data-execution-node-agent="subagent:sdk-task-child"/);
  assert.match(html, /data-execution-node-kind="subagent"/);
  assert.match(html, /data:image\/svg\+xml/);
  assert.doesNotMatch(html, /验收标准/);
  assert.doesNotMatch(html, /依赖.*1/);

  const canvasSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/shared/execution/execution-workgraph-canvas.tsx",
    ),
    "utf8",
  );
  assert.match(canvasSource, /ExecutionNodeInspector/);
  assert.match(canvasSource, /execution\.acceptance/);
  assert.match(canvasSource, /execution\.submission/);
  assert.doesNotMatch(canvasSource, /ExecutionNodePopover/);
});

test("Room WorkGraph surface reuses the chat resource and keeps the bottom rail", async () => {
  const { ExecutionWorkGraphSurface } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-workgraph-surface.tsx",
  );
  const html = await renderWithI18n(
    React.createElement(ExecutionWorkGraphSurface, {
      directory,
      resource: {
        dismiss: () => {},
        error: null,
        execution,
        isLoading: false,
        refresh: () => {},
      },
      taskRuns: [],
    }),
  );
  assert.match(html, /data-execution-workgraph-surface/);
  assert.match(html, /data-execution-workgraph-canvas/);
  assert.match(html, /实现 UI/);

  const { buildRoomHeaderTabs } = await server.ssrLoadModule(
    "/src/features/conversation/room/surface/header/room-header-tabs.ts",
  );
  const keyAsLabel = (key) => key;
  assert.equal(
    buildRoomHeaderTabs(keyAsLabel, { workgraphAvailable: false })
      .some((tab) => tab.key === "workgraph"),
    false,
  );
  assert.equal(
    buildRoomHeaderTabs(keyAsLabel, { workgraphAvailable: true })
      .some((tab) => tab.key === "workgraph"),
    true,
  );

  const [
    shellSource,
    dmControllerSource,
    groupControllerSource,
    dmProjectionSource,
    groupProjectionSource,
    headerSource,
    headerCss,
  ] =
    await Promise.all([
      readFile(path.join(
        webRoot,
        "src/features/conversation/room/surface/room-surface-shell.tsx",
      ), "utf8"),
      readFile(path.join(
        webRoot,
        "src/features/conversation/room/dm/panel/controller/use-dm-chat-panel-model.ts",
      ), "utf8"),
      readFile(path.join(
        webRoot,
        "src/features/conversation/room/group/chat/panel/controller/use-group-chat-panel-model.ts",
      ), "utf8"),
      readFile(path.join(
        webRoot,
        "src/features/conversation/room/dm/panel/controller/dm-chat-panel-projection.ts",
      ), "utf8"),
      readFile(path.join(
        webRoot,
        "src/features/conversation/room/group/chat/panel/controller/group-chat-panel-projection.ts",
      ), "utf8"),
      readFile(path.join(
        webRoot,
        "src/features/conversation/room/surface/header/room-header-tabs.ts",
      ), "utf8"),
      readFile(path.join(
        webRoot,
        "src/shared/ui/workspace/surface/workspace-surface-header.css",
      ), "utf8"),
    ]);
  assert.equal((shellSource.match(/useExecutionResource\(/g) ?? []).length, 1);
  assert.doesNotMatch(dmControllerSource, /useExecutionResource/);
  assert.doesNotMatch(groupControllerSource, /useExecutionResource/);
  assert.match(dmProjectionSource, /scrollToRoundId\(roundId/);
  assert.match(groupProjectionSource, /scrollToRoundId\(roundId/);
  assert.match(dmProjectionSource, /isExecutionActivityVisible/);
  assert.match(groupProjectionSource, /isExecutionActivityVisible/);
  assert.match(shellSource, /executionResource=\{executionResource\}/);
  assert.match(headerSource, /key: "workgraph"/);
  assert.match(headerCss, /workspace-surface-header-with-session-tabs[\s\S]*32px/);
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
