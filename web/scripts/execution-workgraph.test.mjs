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

function orthogonalPathPoints(pathValue) {
  return Array.from(
    pathValue.matchAll(/[ML]\s+(-?\d+(?:\.\d+)?)\s+(-?\d+(?:\.\d+)?)/g),
    (match) => ({ x: Number(match[1]), y: Number(match[2]) }),
  );
}

function orthogonalPathSegments(pathValue) {
  const points = orthogonalPathPoints(pathValue);
  return points.slice(1).flatMap((point, index) => {
    const previous = points[index];
    if (Math.abs(previous.x - point.x) < 0.5) {
      return [{
        axis: "vertical",
        fixed: previous.x,
        start: Math.min(previous.y, point.y),
        end: Math.max(previous.y, point.y),
      }];
    }
    return [{
      axis: "horizontal",
      fixed: previous.y,
      start: Math.min(previous.x, point.x),
      end: Math.max(previous.x, point.x),
    }];
  });
}

function orthogonalPathsShareSegment(leftPath, rightPath) {
  return orthogonalPathSegments(leftPath).some((left) => (
    orthogonalPathSegments(rightPath).some((right) => (
      left.axis === right.axis
      && Math.abs(left.fixed - right.fixed) < 0.5
      && Math.min(left.end, right.end) - Math.max(left.start, right.start) > 0.5
    ))
  ));
}

function orthogonalPathsShareNonTerminalSegment(leftPath, rightPath) {
  const leftSegments = orthogonalPathSegments(leftPath).slice(0, -1);
  const rightSegments = orthogonalPathSegments(rightPath).slice(0, -1);
  return leftSegments.some((left) => (
    rightSegments.some((right) => (
      left.axis === right.axis
      && Math.abs(left.fixed - right.fixed) < 0.5
      && Math.min(left.end, right.end) - Math.max(left.start, right.start) > 0.5
    ))
  ));
}

function orthogonalPathCrossesNode(pathValue, node) {
  const half = node.size / 2;
  const left = node.x - half;
  const right = node.x + half;
  const top = node.y - half;
  const bottom = node.y + half;
  return orthogonalPathSegments(pathValue).some((segment) => {
    if (segment.axis === "vertical") {
      return segment.fixed > left
        && segment.fixed < right
        && Math.min(segment.end, bottom) - Math.max(segment.start, top) > 0.5;
    }
    return segment.fixed > top
      && segment.fixed < bottom
      && Math.min(segment.end, right) - Math.max(segment.start, left) > 0.5;
  });
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

test("WorkGraph model keeps the managed/runtime boundary and current node summary", async () => {
  const {
    compactExecutionNodeObjective,
    hasExecutionGraph,
    hasManagedExecutionGraph,
    isExecutionActivityVisible,
    normalizeExecutionNodeDisplayText,
    resolveExecutionPrimaryAgentNodes,
    resolveExecutionNodeSummary,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-process-model.ts",
  );
  assert.equal(hasManagedExecutionGraph(execution), true);
  assert.equal(hasExecutionGraph(execution), true);
  assert.equal(isExecutionActivityVisible(execution), true);
  const planWithoutItems = structuredClone(execution);
  planWithoutItems.work_items = [];
  assert.equal(hasManagedExecutionGraph(planWithoutItems), false);
  const itemsWithoutActivePlan = structuredClone(execution);
  itemsWithoutActivePlan.plan.status = "proposed";
  assert.equal(hasManagedExecutionGraph(itemsWithoutActivePlan), false);
  assert.deepEqual(
    resolveExecutionPrimaryAgentNodes(execution).map((node) => node.id),
    ["research", "build"],
  );
  const withLead = structuredClone(execution);
  withLead.graph.nodes.push({
    id: "lead-round",
    kind: "agent",
    visibility: "primary",
    work_item_id: "",
    agent_id: "lead",
    lifecycle_status: "running",
    position: 99,
  });
  withLead.graph.edges.push({
    id: "coordination:lead:research",
    kind: "coordination",
    source_node_id: "lead-round",
    target_node_id: "research",
  });
  assert.deepEqual(
    resolveExecutionPrimaryAgentNodes(withLead).map((node) => node.id),
    ["lead-round", "research", "build"],
    "the coordinator remains first even when its runtime node arrived later",
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
  assert.equal(
    normalizeExecutionNodeDisplayText("__nexus_interrupt_without_message__"),
    "",
  );
  assert.equal(
    normalizeExecutionNodeDisplayText(
      "Page failed <nexus_room_no_reply/> __nexus_internal_control__",
    ),
    "Page failed",
  );
});

test("WorkGraph layout reflows without treating containment as dependency", async () => {
  const { buildExecutionGraphLayout } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-workgraph-layout.ts",
  );
  const contained = structuredClone(execution);
  contained.work_items[2].dependency_ids = [];
  contained.work_items[2].parent_work_item_id = "build";
  delete contained.graph;
  assert.equal(
    buildExecutionGraphLayout(contained).edges.some((edge) => (
      edge.sourceId === "build" && edge.targetId === "integrate"
    )),
    false,
    "Work Item containment must not become a readiness/layout dependency",
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
  assert.notEqual(
    addedLayout.nodes.find((node) => node.node.id === "build").x,
    addedLayout.nodes.find((node) => node.node.id === "review").x,
  );
  assert.equal(
    addedLayout.nodes.find((node) => node.node.id === "build").y,
    addedLayout.nodes.find((node) => node.node.id === "review").y,
  );

  const nestedOwnership = structuredClone(execution);
  nestedOwnership.graph.nodes.push(
    {
      id: "attempt-child-second",
      kind: "subagent",
      visibility: "nested",
      work_item_id: "build",
      parent_node_id: "build",
      subject_id: "sdk-task-child-second",
      name: "Second helper",
      lifecycle_status: "running",
      position: 2,
    },
    {
      id: "tool-first-child",
      kind: "tool",
      visibility: "nested",
      work_item_id: "build",
      parent_node_id: "attempt-child",
      subject_id: "tool-first-child",
      name: "Read",
      lifecycle_status: "failed",
      position: 3,
    },
    {
      id: "tool-second-child",
      kind: "tool",
      visibility: "nested",
      work_item_id: "build",
      parent_node_id: "attempt-child-second",
      subject_id: "tool-second-child",
      name: "Bash",
      lifecycle_status: "running",
      position: 4,
    },
  );
  nestedOwnership.graph.edges.push(
    {
      id: "spawn:build:attempt-child-second",
      kind: "spawn",
      source_node_id: "build",
      target_node_id: "attempt-child-second",
    },
    {
      id: "invoke:attempt-child:tool-first-child",
      kind: "invoke",
      source_node_id: "attempt-child",
      target_node_id: "tool-first-child",
    },
    {
      id: "invoke:attempt-child-second:tool-second-child",
      kind: "invoke",
      source_node_id: "attempt-child-second",
      target_node_id: "tool-second-child",
    },
  );
  const nestedLayout = buildExecutionGraphLayout(nestedOwnership);
  assert.deepEqual(
    nestedLayout.groups.map((group) => [group.id, group.nodeIds]),
    [
      [
        "build",
        [
          "build",
          "attempt-child",
          "tool-first-child",
          "attempt-child-second",
          "tool-second-child",
        ],
      ],
    ],
    "one primary Agent frame contains the full runtime tree without nested Subagent frames",
  );
  const buildGroup = nestedLayout.groups.find((group) => group.id === "build");
  const firstChildNode = nestedLayout.nodes.find(
    (node) => node.node.id === "attempt-child",
  );
  const firstChildTool = nestedLayout.nodes.find(
    (node) => node.node.id === "tool-first-child",
  );
  const secondChildNode = nestedLayout.nodes.find(
    (node) => node.node.id === "attempt-child-second",
  );
  const secondChildTool = nestedLayout.nodes.find(
    (node) => node.node.id === "tool-second-child",
  );
  assert.equal(
    firstChildNode.x,
    firstChildTool.x,
    "a Subagent with one tool stays on one vertical tree lane",
  );
  assert.equal(
    secondChildNode.x,
    secondChildTool.x,
    "each sibling Subagent keeps its own descendant lane",
  );
  assert.ok(
    firstChildTool.y > firstChildNode.y,
    "Subagent tools expand downward from their actual owner",
  );
  assert.ok(
    nestedLayout.edges.every((edge) => !/[CQ]/.test(edge.path)),
    "dense responsibility and ownership edges use orthogonal polylines instead of curves",
  );
  assert.ok(
    buildGroup.y + buildGroup.height
      > firstChildTool.y + firstChildTool.size / 2,
    "the primary frame encloses the Subagent descendants",
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
  assert.equal(
    constrainedLayout.nodes[1].x,
    constrainedLayout.nodes[0].x,
    "the main responsibility chain stays on one vertical spine",
  );
  assert.ok(
    constrainedLayout.nodes[1].y > constrainedLayout.nodes[0].y,
    "the main responsibility chain flows from top to bottom after clustering",
  );
});

test("Planless runtime graph promotes active tools and keeps ordinary tools in detail", async () => {
  const {
    hasExecutionGraph,
    hasManagedExecutionGraph,
    isExecutionActivityVisible,
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
  assert.equal(hasExecutionGraph(runtimeExecution), true);
  assert.equal(
    isExecutionActivityVisible(runtimeExecution),
    false,
    "runtime-only observations do not expose the Composer WorkGraph dock",
  );

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
  assert.equal(layout.groups[0].id, "agent-run-1");
  assert.deepEqual(layout.groups[0].nodeIds, ["agent-run-1", "tool-run-1"]);
  assert.deepEqual(
    layout.edges.map((edge) => `${edge.kind}:${edge.sourceId}->${edge.targetId}`),
    ["invoke:agent-run-1->tool-run-1"],
  );
  assert.ok(
    layout.nodes.find((node) => node.node.id === "tool-run-1").y
      > layout.nodes.find((node) => node.node.id === "agent-run-1").y,
    "runtime child layers expand below their owning Agent",
  );

  const missingEdge = structuredClone(runtimeExecution);
  missingEdge.graph.edges = [];
  const repairedLayout = buildExecutionGraphLayout(missingEdge);
  assert.deepEqual(
    repairedLayout.edges.map((edge) => `${edge.kind}:${edge.sourceId}->${edge.targetId}`),
    ["invoke:agent-run-1->tool-run-1"],
    "a visible child with durable parent identity never becomes an orphan icon",
  );

  const retriedExecution = structuredClone(runtimeExecution);
  retriedExecution.graph.nodes[1] = {
    ...retriedExecution.graph.nodes[1],
    error_code: "fetch_failed",
    error_summary: "The requested page could not be reached.",
    lifecycle_status: "failed",
  };
  retriedExecution.graph.nodes.push({
    id: "tool-run-retry",
    kind: "tool",
    visibility: "nested",
    work_item_id: "",
    parent_node_id: "agent-run-1",
    subject_id: "tool-3",
    name: "search",
    lifecycle_status: "running",
    position: 3,
  });
  retriedExecution.graph.edges.push(
    {
      id: "control-return-1",
      kind: "loop_back",
      source_node_id: "tool-run-1",
      target_node_id: "agent-run-1",
    },
    {
      id: "invoke-retry",
      kind: "invoke",
      source_node_id: "agent-run-1",
      target_node_id: "tool-run-retry",
    },
    {
      id: "retry-1",
      kind: "retry",
      source_node_id: "tool-run-1",
      target_node_id: "tool-run-retry",
    },
  );
  const retriedLayout = buildExecutionGraphLayout(retriedExecution);
  assert.deepEqual(
    retriedLayout.edges.map((edge) => edge.kind),
    ["invoke", "loop_back", "invoke", "retry"],
  );
  assert.deepEqual(
    retriedLayout.edges.map((edge) => edge.paired),
    [true, true, false, false],
    "a forward edge and its exact loop-back are exposed as one visual pair",
  );
  assert.ok(
    retriedLayout.nodes.find((node) => node.node.id === "tool-run-retry").y
      > retriedLayout.nodes.find((node) => node.node.id === "agent-run-1").y,
    "an Agent-chosen retry remains in the downward runtime child layer",
  );
  assert.equal(
    retriedLayout.nodes.find((node) => node.node.id === "tool-run-retry").y,
    retriedLayout.nodes.find((node) => node.node.id === "tool-run-1").y,
    "sibling runtime children share the same top-to-bottom depth",
  );
  assert.ok(
    retriedLayout.nodes.find((node) => node.node.id === "tool-run-retry").x
      > retriedLayout.nodes.find((node) => node.node.id === "tool-run-1").x,
    "new sibling runtime children are appended from left to right",
  );
  const failedToolLayout = retriedLayout.nodes.find(
    (node) => node.node.id === "tool-run-1",
  );
  const agentLayout = retriedLayout.nodes.find(
    (node) => node.node.id === "agent-run-1",
  );
  const loopBackLayout = retriedLayout.edges.find(
    (edge) => edge.kind === "loop_back",
  );
  const forwardLayout = retriedLayout.edges.find(
    (edge) => edge.id === "invoke-1",
  );
  const retryLayout = retriedLayout.edges.find(
    (edge) => edge.kind === "retry",
  );
  const loopBackPoints = orthogonalPathPoints(loopBackLayout.path);
  assert.ok(
    Math.abs(loopBackPoints[0].x - failedToolLayout.x) < 0.5
      && Math.abs(
        loopBackPoints[0].y
          - (failedToolLayout.y + failedToolLayout.size / 2),
      ) < 0.5,
    "a return leaves downward from the child like a normal process edge",
  );
  assert.ok(
    Math.abs(
      Math.abs(loopBackPoints.at(-1).x - agentLayout.x)
        - agentLayout.size / 2,
    ) < 0.5
      && Math.abs(loopBackPoints.at(-1).y - agentLayout.y) < 0.5,
    "the outer U-shaped return enters through the parent side port",
  );
  assert.equal(
    orthogonalPathsShareSegment(loopBackLayout.path, forwardLayout.path),
    false,
    "an exact forward/return pair may cross once but never shares a visible segment",
  );
  assert.doesNotMatch(loopBackLayout.path, /[CQ]/);
  assert.match(loopBackLayout.path, / L .* L .* L /);
  assert.ok(
    retryLayout.path.startsWith(
      `M ${failedToolLayout.x} ${failedToolLayout.y + failedToolLayout.size / 2}`,
    ),
    "a same-level retry uses a compact rail below its sibling nodes",
  );
  assert.doesNotMatch(retryLayout.path, /[CQ]/);
  assert.match(retryLayout.path, / L .* L .* L /);

  const downwardRetry = structuredClone(retriedExecution);
  downwardRetry.graph.edges.push({
    id: "retry-agent-child",
    kind: "retry",
    source_node_id: "agent-run-1",
    target_node_id: "tool-run-1",
  });
  const downwardRetryLayout = buildExecutionGraphLayout(downwardRetry);
  const downwardAgentLayout = downwardRetryLayout.nodes.find(
    (node) => node.node.id === "agent-run-1",
  );
  const downwardRetryEdge = downwardRetryLayout.edges.find(
    (edge) => edge.id === "retry-agent-child",
  );
  const downwardToolLayout = downwardRetryLayout.nodes.find(
    (node) => node.node.id === "tool-run-1",
  );
  const downwardRetryPoints = orthogonalPathPoints(downwardRetryEdge.path);
  const downwardForwardEdge = downwardRetryLayout.edges.find(
    (edge) => edge.id === "invoke-1",
  );
  assert.ok(
    Math.abs(downwardRetryPoints[0].x - downwardAgentLayout.x) < 0.5
      && Math.abs(
        downwardRetryPoints[0].y
          - (downwardAgentLayout.y - downwardAgentLayout.size / 2),
      ) < 0.5
      && Math.abs(
        Math.abs(downwardRetryPoints.at(-1).x - downwardToolLayout.x)
          - downwardToolLayout.size / 2,
      ) < 0.5,
    "a downward retry first leaves above its source layer and returns through the target side",
  );
  assert.equal(
    orthogonalPathsShareSegment(
      downwardRetryEdge.path,
      downwardForwardEdge.path,
    ),
    false,
    "a downward retry stays off the normal invoke edge",
  );

  const wideReturn = structuredClone(runtimeExecution);
  for (let index = 0; index < 7; index += 1) {
    const id = `tool-wide-${index}`;
    wideReturn.graph.nodes.push({
      id,
      kind: "tool",
      visibility: "nested",
      work_item_id: "",
      parent_node_id: "agent-run-1",
      subject_id: id,
      name: "search",
      lifecycle_status: "failed",
      position: index + 3,
    });
    wideReturn.graph.edges.push({
      id: `invoke:${id}`,
      kind: "invoke",
      source_node_id: "agent-run-1",
      target_node_id: id,
    });
  }
  wideReturn.graph.edges.push({
    id: "wide-control-return",
    kind: "loop_back",
    source_node_id: "tool-run-1",
    target_node_id: "agent-run-1",
  });
  const wideReturnLayout = buildExecutionGraphLayout(wideReturn);
  const wideSourceLayout = wideReturnLayout.nodes.find(
    (node) => node.node.id === "tool-run-1",
  );
  const wideTargetLayout = wideReturnLayout.nodes.find(
    (node) => node.node.id === "agent-run-1",
  );
  const wideReturnEdge = wideReturnLayout.edges.find(
    (edge) => edge.id === "wide-control-return",
  );
  const wideForwardEdge = wideReturnLayout.edges.find(
    (edge) => edge.id === "invoke-1",
  );
  const wideReturnPoints = orthogonalPathPoints(wideReturnEdge.path);
  assert.ok(
    Math.abs(wideReturnPoints[0].x - wideSourceLayout.x) < 0.5
      && Math.abs(
        wideReturnPoints[0].y
          - (wideSourceLayout.y + wideSourceLayout.size / 2),
      ) < 0.5,
    "a wide return first follows the normal downward flow out of its source",
  );
  assert.ok(
    Math.abs(
      Math.abs(wideReturnPoints.at(-1).x - wideTargetLayout.x)
        - wideTargetLayout.size / 2,
    ) < 0.5
      && Math.abs(wideReturnPoints.at(-1).y - wideTargetLayout.y) < 0.5,
    "a wide return enters the target from its dedicated outer corridor",
  );
  assert.equal(
    orthogonalPathsShareSegment(wideReturnEdge.path, wideForwardEdge.path),
    false,
    "a dense ownership fan never forces the return back onto its forward edge",
  );
  assert.doesNotMatch(wideReturnEdge.path, /[CQ]/);

  const crowdedReturns = structuredClone(wideReturn);
  for (const index of [0, 1]) {
    crowdedReturns.graph.edges.push({
      id: `wide-control-return-${index}`,
      kind: "loop_back",
      source_node_id: `tool-wide-${index}`,
      target_node_id: "agent-run-1",
    });
  }
  const crowdedReturnLayout = buildExecutionGraphLayout(crowdedReturns);
  const crowdedReturnEdges = crowdedReturnLayout.edges.filter(
    (edge) => edge.kind === "loop_back",
  );
  const crowdedReturnGroup = crowdedReturnLayout.groups.find(
    (group) => group.id === "agent-run-1",
  );
  const controlFrameSafeGap = 16;
  assert.equal(crowdedReturnEdges.length, 3);
  for (const returnEdge of crowdedReturnEdges) {
    for (const point of orthogonalPathPoints(returnEdge.path)) {
      assert.ok(
        point.x >= 0
          && point.x <= crowdedReturnLayout.width
          && point.y >= 0
          && point.y <= crowdedReturnLayout.height,
        `return ${returnEdge.id} remains inside the interactive canvas`,
      );
      assert.ok(
        point.x >= crowdedReturnGroup.x
          && point.x <= crowdedReturnGroup.x + crowdedReturnGroup.width
          && point.y >= crowdedReturnGroup.y
          && point.y <= crowdedReturnGroup.y + crowdedReturnGroup.height,
        `return ${returnEdge.id} remains inside its owning subgraph frame`,
      );
      assert.ok(
        point.x >= crowdedReturnGroup.x + controlFrameSafeGap
          && point.x <= crowdedReturnGroup.x
            + crowdedReturnGroup.width
            - controlFrameSafeGap
          && point.y >= crowdedReturnGroup.y + controlFrameSafeGap
          && point.y <= crowdedReturnGroup.y
            + crowdedReturnGroup.height
            - controlFrameSafeGap,
        `return ${returnEdge.id} keeps a visible safe gap from its subgraph frame`,
      );
    }
  }
  for (let left = 0; left < crowdedReturnEdges.length; left += 1) {
    const returnEdge = crowdedReturnEdges[left];
    const sourceId = returnEdge.sourceId;
    const matchingForward = crowdedReturnLayout.edges.find((edge) => (
      edge.kind === "invoke"
      && edge.sourceId === "agent-run-1"
      && edge.targetId === sourceId
    ));
    assert.equal(
      orthogonalPathsShareSegment(returnEdge.path, matchingForward.path),
      false,
      `return ${returnEdge.id} stays off its exact forward branch`,
    );
    for (const node of crowdedReturnLayout.nodes) {
      if (node.node.id === returnEdge.sourceId
        || node.node.id === returnEdge.targetId) {
        continue;
      }
      assert.equal(
        orthogonalPathCrossesNode(returnEdge.path, node),
        false,
        `return ${returnEdge.id} never crosses node ${node.node.id}`,
      );
    }
    for (let right = left + 1; right < crowdedReturnEdges.length; right += 1) {
      assert.equal(
        orthogonalPathsShareNonTerminalSegment(
          returnEdge.path,
          crowdedReturnEdges[right].path,
        ),
        true,
        "same-side returns merge onto one shared U-shaped flow bus",
      );
    }
  }
});

test("WorkGraph Tool nodes use semantic action icon categories", async () => {
  const { resolveExecutionToolVisualKind } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-tool-visual.ts",
  );
  const { ExecutionNodeAvatar } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-node-avatar.tsx",
  );
  assert.deepEqual(
    [
      "WebSearch",
      "browser.web-fetch",
      "mcp__browser__navigate",
      "Bash",
      "mcp__node_repl__js",
      "mcp__filesystem__write_file",
      "mcp__slack__send_message",
      "mcp__imagegen__generate_image",
      "mcp__github__list_issues",
      "Read",
      "mcp__nexus_execution__submit_work",
    ].map((name) => resolveExecutionToolVisualKind(name)),
    [
      "search",
      "fetch",
      "browser",
      "terminal",
      "terminal",
      "write",
      "send",
      "generate",
      "external",
      "inspect",
      "workflow",
    ],
  );
  const fetchIcon = renderToStaticMarkup(React.createElement(ExecutionNodeAvatar, {
    agent: null,
    kind: "tool",
    size: "nested",
    status: "accepted",
    title: "WebFetch",
    toolName: "WebFetch",
  }));
  const inspectIcon = renderToStaticMarkup(React.createElement(ExecutionNodeAvatar, {
    agent: null,
    kind: "tool",
    size: "nested",
    status: "accepted",
    title: "Read",
    toolName: "Read",
  }));
  assert.match(fetchIcon, /data-execution-tool-visual="fetch"/);
  assert.doesNotMatch(fetchIcon, /lucide-wrench/);
  assert.match(inspectIcon, /data-execution-tool-visual="inspect"/);
  assert.doesNotMatch(inspectIcon, /lucide-wrench/);
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
    layout.nodes.find((node) => node.node.kind === "gate").y
      > layout.nodes.find((node) => node.node.kind === "agent").y,
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
    layout.nodes.find((node) => node.node.kind === "gate").y
      > layout.nodes.find((node) => node.node.kind === "agent").y,
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
  assert.match(
    html,
    /h-8 w-8 rounded-\[10px\]/,
    "the Dock Agent frame matches the chat message avatar's 32px footprint",
  );
  assert.doesNotMatch(html, /h-11 w-11 rounded-\[13px\]/);
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
  assert.match(panelSource, /size="dock"/);
  assert.doesNotMatch(panelSource, /ExecutionWorkGraphCanvas/);
  assert.doesNotMatch(panelSource, /ANCHORED_OVERLAY_MOTION_CLASS_NAME/);
});

test("WorkGraph interaction model collapses, searches, and fits large graphs without mutating topology", async () => {
  const {
    clampExecutionGraphZoom,
    nextExecutionGraphSearchResult,
    projectExecutionGraphCollapse,
    resolveExecutionGraphAnchoredScroll,
    resolveExecutionGraphFitZoom,
    resolveExecutionGraphNodeAncestors,
    resolveExecutionGraphPanPadding,
    resolveExecutionGraphWheelZoom,
    resolveExecutionWorkspaceReference,
    searchExecutionGraphNodes,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-workgraph-interaction-model.ts",
  );
  const { buildExecutionGraphLayout } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-workgraph-layout.ts",
  );
  const searchable = structuredClone(execution);
  const child = searchable.graph.nodes.find((node) => node.id === "attempt-child");
  child.runs = [{
    id: "runtime-child-1",
    status: "failed",
    error_summary: "Browser session disconnected",
    artifacts: [{
      id: "workspace_file:tool-1:reports/result.md",
      type: "workspace_file_artifact",
      path: "reports/result.md",
      source_tool_use_id: "tool-1",
    }],
  }];

  const collapsed = projectExecutionGraphCollapse(searchable, new Set(["build"]));
  assert.deepEqual([...collapsed.hiddenNodeIds], ["attempt-child"]);
  assert.equal(collapsed.descendantCountByNodeId.get("build"), 1);
  assert.deepEqual(
    resolveExecutionGraphNodeAncestors(searchable, "attempt-child"),
    ["build"],
  );
  const collapsedLayout = buildExecutionGraphLayout(
    searchable,
    700,
    collapsed.hiddenNodeIds,
  );
  assert.equal(
    collapsedLayout.nodes.some((node) => node.node.id === "attempt-child"),
    false,
  );
  assert.deepEqual(searchExecutionGraphNodes(searchable, "disconnected"), [
    "attempt-child",
  ]);
  assert.deepEqual(searchExecutionGraphNodes(searchable, "reports/result.md"), [
    "attempt-child",
  ]);
  assert.equal(
    nextExecutionGraphSearchResult(["research", "build"], "build", 1),
    "research",
  );
  assert.equal(clampExecutionGraphZoom(9), 1.5);
  assert.equal(clampExecutionGraphZoom(0.1), 0.5);
  assert.equal(resolveExecutionGraphFitZoom({
    contentHeight: 600,
    contentWidth: 1_000,
    viewportHeight: 400,
    viewportWidth: 600,
  }), 0.58);
  assert.equal(resolveExecutionGraphPanPadding(1_000), 500);
  assert.equal(resolveExecutionGraphPanPadding(0), 48);
  assert.deepEqual(resolveExecutionGraphAnchoredScroll({
    currentZoom: 1,
    nextZoom: 1.5,
    panPaddingX: 100,
    panPaddingY: 80,
    scrollLeft: 100,
    scrollTop: 80,
    viewportX: 300,
    viewportY: 200,
  }), {
    contentX: 300,
    contentY: 200,
    scrollLeft: 250,
    scrollTop: 180,
  });
  assert.equal(resolveExecutionGraphWheelZoom(1, -50), 1.1);
  assert.equal(resolveExecutionGraphWheelZoom(1, 2), 0.99);
  assert.equal(resolveExecutionWorkspaceReference("reports/result.md"), "reports/result.md");
  assert.equal(resolveExecutionWorkspaceReference("../outside.txt"), null);
  assert.equal(resolveExecutionWorkspaceReference("https://example.com/result"), null);
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
  assert.match(
    html,
    /class="absolute origin-top-left overflow-visible" data-execution-workgraph-canvas/,
  );
  assert.match(html, /data-execution-board-grid/);
  assert.match(html, /data-execution-board-panning="false"/);
  assert.match(html, /data-execution-board-space-pan="false"/);
  assert.match(html, /data-execution-pan-padding-x="48"/);
  assert.match(html, /data-execution-pan-padding-y="48"/);
  assert.match(html, /data-execution-workgraph-controls/);
  assert.match(html, /data-execution-workgraph-scale="1"/);
  assert.match(html, /data-execution-collapse-node="build"/);
  assert.match(html, /data-execution-node-detail-mode="popover"/);
  assert.match(html, /data-execution-edge-layer/);
  assert.match(html, /data-execution-edge-source="research"/);
  assert.match(html, /data-execution-edge-target="build"/);
  assert.match(html, /data-execution-edge-kind="spawn"/);
  assert.match(html, /data-execution-edge-target="attempt-child"/);
  assert.match(html, /data-execution-edge-line-hit="dependency:research:build"/);
  assert.match(html, /button[^>]+data-execution-edge-hit-target="dependency:research:build"/);
  assert.match(html, /data-execution-edge-hit-kind="dependency"/);
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
  assert.doesNotMatch(html, />依赖\s*1\s*</);

  const canvasSource = await readFile(
    path.join(
      webRoot,
      "src/features/conversation/shared/execution/execution-workgraph-canvas.tsx",
    ),
    "utf8",
  );
  assert.match(canvasSource, /ExecutionNodeInspector/);
  assert.match(canvasSource, /ExecutionEdgeInspector/);
  assert.match(canvasSource, /ExecutionNodeRunList/);
  assert.match(canvasSource, /ExecutionNodeRunHistory/);
  assert.match(canvasSource, /ExecutionWorkGraphControls/);
  assert.match(canvasSource, /setPointerCapture/);
  assert.match(canvasSource, /scrollLeft = gesture\.scrollLeft - deltaX/);
  assert.match(canvasSource, /event\.button === 2/);
  assert.match(canvasSource, /event\.code === "Space"/);
  assert.match(canvasSource, /resolveExecutionGraphWheelZoom/);
  assert.match(canvasSource, /handlePinchMove/);
  assert.match(canvasSource, /onDoubleClick/);
  assert.match(canvasSource, /viewport\.scrollBy/);
  assert.match(canvasSource, /data-execution-edge-paired/);
  assert.match(canvasSource, /strokeLinejoin="round"/);
  assert.match(
    canvasSource,
    /color-mix\(in srgb, var\(--warning\) 62%, var\(--icon-muted\)\)/,
  );
  assert.match(canvasSource, /isExecutionGraphInteractiveTarget\(event\.target\)/);
  assert.match(canvasSource, /closeGraphDetails\(\)/);
  assert.match(canvasSource, /data-execution-selected-node-detail-mode="popover"/);
  assert.match(canvasSource, /bg-\(--surface-popover-background\)/);
  assert.match(canvasSource, /border-\(--surface-popover-border\)/);
  assert.match(canvasSource, /shadow-\(--surface-popover-shadow\)/);
  assert.match(canvasSource, /execution\.error_summary/);
  assert.match(canvasSource, /execution\.result_summary/);
  assert.match(canvasSource, /execution\.control_return_observed/);
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
        isStale: false,
        lastSuccessfulAt: null,
        refresh: () => {},
      },
      taskRuns: [],
    }),
  );
  assert.match(html, /data-execution-workgraph-surface/);
  assert.match(html, /data-execution-workgraph-canvas/);
  assert.match(html, /实现 UI/);

  const runtimeOnlyHTML = await renderWithI18n(
    React.createElement(ExecutionWorkGraphSurface, {
      directory,
      resource: {
        dismiss: () => {},
        error: null,
        execution: {
          ...execution,
          plan: null,
          work_items: [],
        },
        isLoading: false,
        isStale: false,
        lastSuccessfulAt: null,
        refresh: () => {},
      },
      taskRuns: [],
    }),
  );
  assert.doesNotMatch(runtimeOnlyHTML, /data-execution-workgraph-canvas/);
  assert.match(runtimeOnlyHTML, />当前会话还没有工作图</);

  const partialHTML = await renderWithI18n(
    React.createElement(ExecutionWorkGraphSurface, {
      directory,
      resource: {
        dismiss: () => {},
        error: "read failed",
        execution: {
          ...execution,
          graph: {
            ...execution.graph,
            runtime_node_total: 44,
            runtime_nodes_truncated: true,
          },
        },
        isLoading: false,
        isStale: true,
        lastSuccessfulAt: Date.now(),
        refresh: () => {},
      },
      taskRuns: [],
    }),
  );
  assert.match(partialHTML, /data-execution-workgraph-partial="true"/);
  assert.match(partialHTML, /data-execution-workgraph-stale="true"/);
  assert.match(partialHTML, />部分</);
  assert.match(partialHTML, />未同步</);

  const { buildRoomHeaderTabs } = await server.ssrLoadModule(
    "/src/features/conversation/room/surface/header/room-header-tabs.ts",
  );
  const keyAsLabel = (key) => key;
  assert.equal(
    buildRoomHeaderTabs(keyAsLabel)
      .some((tab) => tab.key === "workgraph"),
    true,
  );

  const [
    shellSource,
    dmControllerSource,
    groupControllerSource,
    dmProjectionSource,
    groupProjectionSource,
    desktopSurfaceSource,
    mobileSurfaceSource,
    headerSource,
    headerOverflowSource,
    desktopLayoutControllerSource,
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
        "src/features/conversation/room/surface/layout/room-surface-content.tsx",
      ), "utf8"),
      readFile(path.join(
        webRoot,
        "src/features/conversation/room/surface/mobile/room-mobile-surface.tsx",
      ), "utf8"),
      readFile(path.join(
        webRoot,
        "src/features/conversation/room/surface/header/room-header-tabs.ts",
      ), "utf8"),
      readFile(path.join(
        webRoot,
        "src/features/conversation/room/surface/header/use-room-header-overflow-tabs.ts",
      ), "utf8"),
      readFile(path.join(
        webRoot,
        "src/features/conversation/room/surface/layout/use-room-surface-layout-controller.ts",
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
  assert.doesNotMatch(
    desktopSurfaceSource,
    /hasManagedExecutionGraph/,
  );
  assert.match(
    mobileSurfaceSource,
    /hasManagedExecutionGraph\(\s*executionResource\.execution,?\s*\)/,
  );
  assert.match(shellSource, /executionResource=\{executionResource\}/);
  assert.match(headerSource, /key: "workgraph"/);
  assert.doesNotMatch(headerSource, /workgraphAvailable/);
  assert.doesNotMatch(headerOverflowSource, /workgraph:/);
  assert.doesNotMatch(
    desktopLayoutControllerSource,
    /activeSurfaceTab === "workgraph"/,
  );
  assert.match(headerCss, /workspace-surface-header-with-session-tabs[\s\S]*32px/);
});

test("Execution MCP names render as semantic activity instead of raw transport names", async () => {
  const { getToolInputSummary, getToolTitle } = await server.ssrLoadModule(
    "/src/features/conversation/shared/message/tool-activity.ts",
  );
  assert.equal(
    getToolTitle("mcp__nexus_execution__prepare_plan_execution"),
    "封存计划提案",
  );
  assert.equal(
    getToolTitle("mcp__nexus_execution__plan_execution"),
    "提交计划提案",
  );
  assert.equal(
    getToolInputSummary({
      proposal_id: "proposal-secret",
      proposal_digest: "digest-secret",
    }),
    null,
  );
});

test("Execution resource uses realtime invalidation with a bounded visible fallback", async () => {
  const [resourceSource, shellSource, messageHandlerSource, sessionHandlerSource] =
    await Promise.all([
      readFile(path.join(
        webRoot,
        "src/features/conversation/shared/execution/use-execution-resource.ts",
      ), "utf8"),
      readFile(path.join(
        webRoot,
        "src/features/conversation/room/surface/room-surface-shell.tsx",
      ), "utf8"),
      readFile(path.join(
        webRoot,
        "src/hooks/agent/transport/handlers/agent-message-event-handlers.ts",
      ), "utf8"),
      readFile(path.join(
        webRoot,
        "src/hooks/agent/transport/handlers/session-event-handlers.ts",
      ), "utf8"),
    ]);

  assert.match(resourceSource, /ACTIVE_EXECUTION_FALLBACK_POLL_MS = 30_000/);
  assert.match(resourceSource, /EXECUTION_INVALIDATION_DEBOUNCE_MS = 200/);
  assert.match(resourceSource, /document\.visibilityState === "visible"/);
  assert.doesNotMatch(resourceSource, /conversationActive/);
  assert.doesNotMatch(resourceSource, /"paused",/);
  assert.match(shellSource, /executionEventRevision/);
  assert.match(shellSource, /eventType === "agent_round_status"/);
  assert.match(messageHandlerSource, /onRoomEvent\?\.\(event\.event_type/);
  assert.equal(
    (sessionHandlerSource.match(/onRoomEvent\?\.\(event\.event_type/g) ?? []).length,
    3,
  );
});
