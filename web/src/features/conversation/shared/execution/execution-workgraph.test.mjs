// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";



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
  const halfWidth = (node.width ?? node.size) / 2;
  const halfHeight = (node.height ?? node.size) / 2;
  const left = node.x - halfWidth;
  const right = node.x + halfWidth;
  const top = node.y - halfHeight;
  const bottom = node.y + halfHeight;
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

function orthogonalPathCrossesGroupInterior(pathValue, group) {
  const left = group.x;
  const right = group.x + group.width;
  const top = group.y;
  const bottom = group.y + group.height;
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

test("WorkGraph model keeps the managed/runtime boundary and current node summary", async () => {
  const {
    compactExecutionNodeObjective,
    hasExecutionGraph,
    hasManagedExecutionGraph,
    isExecutionActivityVisible,
    normalizeExecutionNodeDisplayText,
    resolveExecutionGraphNodeStatus,
    resolveExecutionPrimaryAgentNodes,
    resolveExecutionNodeSummary,
    resolveExecutionWorkGraphHeaderModel,
  } = await import("@/features/conversation/shared/execution/execution-process-model.ts");
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
  const reviewedCycles = structuredClone(execution);
  reviewedCycles.work_items = [{
    ...reviewedCycles.work_items[1],
    status: "accepted",
    attempts: [],
  }];
  reviewedCycles.graph.nodes = [
    {
      id: "attempt-old",
      kind: "agent",
      visibility: "primary",
      work_item_id: "build",
      attempt_id: "attempt-old",
      agent_id: "builder",
      run_status: "succeeded",
      position: 1,
    },
    {
      id: "review:submission-old",
      kind: "gate",
      visibility: "primary",
      work_item_id: "build",
      attempt_id: "attempt-old",
      agent_id: "lead",
      lifecycle_status: "rejected",
      position: 1,
    },
    {
      id: "build",
      kind: "agent",
      visibility: "primary",
      work_item_id: "build",
      attempt_id: "attempt-new",
      agent_id: "builder",
      responsibility_status: "accepted",
      run_status: "succeeded",
      position: 1,
    },
    {
      id: "review:submission-new",
      kind: "gate",
      visibility: "primary",
      work_item_id: "build",
      attempt_id: "attempt-new",
      agent_id: "lead",
      lifecycle_status: "accepted",
      position: 1,
    },
  ];
  assert.equal(
    resolveExecutionGraphNodeStatus(
      reviewedCycles.graph.nodes[0],
      reviewedCycles.work_items[0],
    ),
    "submitted",
    "an old succeeded Attempt must not inherit the latest Work Item acceptance",
  );
  assert.equal(
    resolveExecutionNodeSummary(reviewedCycles).currentNode.id,
    "build",
    "focus stays on the current stable Work Item cycle",
  );
  assert.deepEqual(
    resolveExecutionPrimaryAgentNodes(reviewedCycles).map((node) => node.id),
    ["build"],
    "the Agent shortcut represents the latest cycle instead of an old Submission",
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
  const headerExecution = {
    ...execution,
    completion_blockers: [" waiting for final review ", ""],
  };
  assert.deepEqual(resolveExecutionWorkGraphHeaderModel(headerExecution), {
    currentNodeId: "attempt-child",
    status: "active",
    statusLabelKey: "execution.status_active",
    summary: "实现 UI",
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
  const { buildExecutionGraphLayout } = await import("@/features/conversation/shared/execution/execution-workgraph-layout.ts");
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
    addedLayout.edges.map((edge) => `${edge.sourceId}->${edge.targetId}`).sort(),
    [
      "research->build",
      "build->attempt-child",
      "research->review",
      "build->integrate",
      "review->integrate",
    ].sort(),
  );
  assert.notEqual(
    addedLayout.nodes.find((node) => node.node.id === "build").x,
    addedLayout.nodes.find((node) => node.node.id === "review").x,
  );
  assert.equal(
    addedLayout.nodes.find((node) => node.node.id === "build").y,
    addedLayout.nodes.find((node) => node.node.id === "review").y,
  );
  assert.deepEqual(
    addedLayout.ports.map((port) => ({
      edges: port.edgeIds,
      group: port.groupId,
      role: port.role,
      side: port.side,
    })).sort((left, right) => left.role.localeCompare(right.role)),
    [
      {
        edges: ["dependency:research:build"],
        group: "build",
        role: "target",
        side: "top",
      },
      {
        edges: ["dependency:build:integrate"],
        group: "build",
        role: "source",
        side: "bottom",
      },
    ].sort((left, right) => left.role.localeCompare(right.role)),
    "cross-subgraph edges attach to stable frame ports instead of the core node",
  );
  const buildFrame = addedLayout.groups.find((group) => group.id === "build");
  const buildIncomingPort = addedLayout.ports.find((port) => port.role === "target");
  const buildOutgoingPort = addedLayout.ports.find((port) => port.role === "source");
  assert.equal(buildIncomingPort.y, buildFrame.y);
  assert.equal(buildOutgoingPort.y, buildFrame.y + buildFrame.height);
  const incomingFrameEdge = addedLayout.edges.find(
    (edge) => edge.id === "dependency:research:build",
  );
  const outgoingFrameEdge = addedLayout.edges.find(
    (edge) => edge.id === "dependency:build:integrate",
  );
  assert.deepEqual(orthogonalPathPoints(incomingFrameEdge.path).at(-1), {
    x: buildIncomingPort.x,
    y: buildIncomingPort.y - 6,
  }, "the arrowhead keeps a visible gap before the frame port");
  assert.deepEqual(orthogonalPathPoints(outgoingFrameEdge.path)[0], {
    x: buildOutgoingPort.x,
    y: buildOutgoingPort.y,
  });
  const reviewIncomingEdge = addedLayout.edges.find(
    (edge) => edge.id === "dependency:research:review",
  );
  const reviewTarget = addedLayout.nodes.find(
    (node) => node.node.id === "review",
  );
  const reviewIncomingPoints = orthogonalPathPoints(reviewIncomingEdge.path);
  assert.deepEqual(reviewIncomingPoints.at(-1), {
    x: reviewTarget.x,
    y: reviewTarget.y - reviewTarget.height / 2 - 6,
  });
  assert.equal(
    reviewIncomingPoints.at(-2).x,
    reviewIncomingPoints.at(-1).x,
    "obstacle routing preserves a perpendicular final approach to a node",
  );
  assert.ok(
    incomingFrameEdge.targetTailPath && outgoingFrameEdge.sourceTailPath,
    "frame ports retain on-demand tails to the exact semantic endpoints",
  );
  for (const edge of addedLayout.edges) {
    for (const group of addedLayout.groups) {
      if (group.nodeIds.includes(edge.sourceId) && group.nodeIds.includes(edge.targetId)) {
        continue;
      }
      assert.equal(
        orthogonalPathCrossesGroupInterior(edge.path, group),
        false,
        `${edge.id} must treat unrelated subgraph ${group.id} as a hard obstacle`,
      );
    }
  }
  const crossSubgraphControl = structuredClone(branched);
  crossSubgraphControl.graph.edges.push({
    id: "retry:integrate:attempt-child",
    kind: "retry",
    source_node_id: "integrate",
    target_node_id: "attempt-child",
  });
  const crossSubgraphControlLayout = buildExecutionGraphLayout(
    crossSubgraphControl,
  );
  const crossControlEdge = crossSubgraphControlLayout.edges.find(
    (edge) => edge.id === "retry:integrate:attempt-child",
  );
  const crossControlPort = crossSubgraphControlLayout.ports.find(
    (port) => port.edgeIds.includes("retry:integrate:attempt-child"),
  );
  assert.ok(crossControlPort, "cross-subgraph control edges also use frame ports");
  assert.ok(crossControlEdge.targetTailPath);
  assert.equal(
    orthogonalPathCrossesGroupInterior(
      crossControlEdge.path,
      crossSubgraphControlLayout.groups.find((group) => group.id === "build"),
    ),
    false,
    "cross-subgraph control edges obey the same hard frame obstacle",
  );

  const directionalControl = structuredClone(branched);
  directionalControl.graph.edges.push({
    id: "retry:build:review",
    kind: "retry",
    source_node_id: "build",
    target_node_id: "review",
  });
  const directionalControlLayout = buildExecutionGraphLayout(directionalControl);
  const directionalControlEdge = directionalControlLayout.edges.find(
    (edge) => edge.id === "retry:build:review",
  );
  const directionalControlTarget = directionalControlLayout.nodes.find(
    (node) => node.node.id === "review",
  );
  const directionalControlPoints = orthogonalPathPoints(directionalControlEdge.path);
  const directionalControlEndpoint = directionalControlPoints.at(-1);
  const directionalControlPrevious = directionalControlPoints.at(-2);
  assert.deepEqual(directionalControlEndpoint, {
    x: directionalControlTarget.x,
    y: directionalControlTarget.y - directionalControlTarget.height / 2 - 6,
  }, "cross-root arrows align to the selected target side and keep node clearance");
  assert.equal(
    directionalControlPrevious.x,
    directionalControlEndpoint.x,
    "the final arrow segment approaches perpendicular to the target side",
  );

  const crossing = structuredClone(execution);
  crossing.graph.nodes = [
    ["source-a", 0],
    ["source-b", 1],
    ["target-c", 2],
    ["target-d", 3],
  ].map(([id, position]) => ({
    id,
    kind: "agent",
    visibility: "primary",
    work_item_id: id,
    responsibility_status: "waiting",
    position,
  }));
  crossing.graph.edges = [
    {
      id: "dependency:source-a:target-d",
      kind: "dependency",
      source_node_id: "source-a",
      target_node_id: "target-d",
    },
    {
      id: "dependency:source-b:target-c",
      kind: "dependency",
      source_node_id: "source-b",
      target_node_id: "target-c",
    },
  ];
  crossing.work_items = crossing.graph.nodes.map((node) => ({
    id: node.id,
    logical_key: node.id,
    kind: "produce",
    subject: node.id,
    objective: node.id,
    deliverable: node.id,
    acceptance_criteria: ["done"],
    dependency_ids: [],
    required: true,
    position: node.position,
    status: "waiting",
    updated_at: "2026-07-31T10:02:00Z",
  }));
  const crossingLayout = buildExecutionGraphLayout(crossing);
  const crossingNode = (id) => crossingLayout.nodes.find((node) => node.node.id === id);
  assert.ok(
    crossingNode("target-d").x < crossingNode("target-c").x,
    "a layer reorders only when the deterministic barycenter sweep removes a crossing",
  );
  assert.equal(
    orthogonalPathsShareSegment(
      crossingLayout.edges[0].path,
      crossingLayout.edges[1].path,
    ),
    false,
    "unrelated routes keep independent tracks instead of creating a false shared bus",
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
    nestedLayout.junctions.map((junction) => ({
      edges: junction.edgeIds,
      kind: junction.kind,
    })),
    [{
      edges: [
        "spawn:build:attempt-child",
        "spawn:build:attempt-child-second",
      ],
      kind: "fan-out",
    }],
    "same-source edges still share a marked local bus inside one subgraph",
  );
  assert.equal(
    orthogonalPathsShareNonTerminalSegment(
      nestedLayout.edges.find(
        (edge) => edge.id === "spawn:build:attempt-child",
      ).path,
      nestedLayout.edges.find(
        (edge) => edge.id === "spawn:build:attempt-child-second",
      ).path,
    ),
    true,
  );
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

test("WorkGraph interaction model collapses, searches, and fits large graphs without mutating topology", async () => {
  const {
    clampExecutionGraphZoom,
    nextExecutionGraphSearchResult,
    projectExecutionGraphCollapse,
    resolveExecutionGraphAnchoredScroll,
    resolveExecutionGraphFitZoom,
    resolveExecutionGraphGroupTrace,
    resolveExecutionGraphInitialScroll,
    resolveExecutionGraphNodeAncestors,
    resolveExecutionGraphPanPadding,
    resolveExecutionGraphTrace,
    resolveExecutionGraphWheelZoom,
    resolveExecutionWorkspaceReference,
    searchExecutionGraphNodes,
  } = await import("@/features/conversation/shared/execution/execution-workgraph-interaction-model.ts");
  const { buildExecutionGraphLayout } = await import("@/features/conversation/shared/execution/execution-workgraph-layout.ts");
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
  const traced = resolveExecutionGraphTrace([
    ...buildExecutionGraphLayout(searchable).edges,
    {
      id: "loop:attempt-child:build",
      kind: "loop_back",
      sourceId: "attempt-child",
      targetId: "build",
    },
  ], "attempt-child");
  assert.deepEqual([...traced.nodeIds], ["attempt-child", "build", "research"]);
  assert.deepEqual([...traced.edgeIds], [
    "spawn:build:attempt-child",
    "dependency:research:build",
    "loop:attempt-child:build",
  ]);
  const groupTrace = resolveExecutionGraphGroupTrace(
    buildExecutionGraphLayout(searchable).edges,
    ["build", "attempt-child"],
  );
  assert.deepEqual([...groupTrace.nodeIds].sort(), [
    "build",
    "attempt-child",
    "research",
    "integrate",
  ].sort());
  assert.deepEqual([...groupTrace.edgeIds].sort(), [
    "dependency:research:build",
    "spawn:build:attempt-child",
    "dependency:build:integrate",
  ].sort());
  const collapsedLayout = buildExecutionGraphLayout(
    searchable,
    700,
    collapsed.hiddenNodeIds,
  );
  assert.equal(
    collapsedLayout.nodes.some((node) => node.node.id === "attempt-child"),
    false,
  );
  // 搜索只覆盖结构字段：运行摘要与 Artifact 属于按需读取的节点详情，不参与检索。
  assert.deepEqual(searchExecutionGraphNodes(searchable, "disconnected"), []);
  assert.deepEqual(searchExecutionGraphNodes(searchable, "reports/result.md"), []);
  assert.deepEqual(searchExecutionGraphNodes(searchable, "attempt-child"), [
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
  assert.deepEqual(resolveExecutionGraphInitialScroll({
    contentHeight: 700,
    contentWidth: 448,
    panPaddingX: 800,
    panPaddingY: 500,
    viewportHeight: 1_000,
    viewportWidth: 1_600,
    zoom: 1,
  }), {
    left: 224,
    top: 350,
  });
  assert.deepEqual(resolveExecutionGraphInitialScroll({
    contentHeight: 1_400,
    contentWidth: 448,
    panPaddingX: 800,
    panPaddingY: 500,
    viewportHeight: 1_000,
    viewportWidth: 1_600,
    zoom: 1,
  }), {
    left: 224,
    top: 500,
  });
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

test("geometry cache reuses routes, refreshes facts, and invalidates structural changes", async () => {
  const { createExecutionGraphLayoutCache, buildExecutionGraphLayout } = await import("./execution-workgraph-layout.ts");
  const cached = createExecutionGraphLayoutCache();
  const first = cached(execution, 800);
  const refreshed = structuredClone(execution);
  refreshed.graph.nodes[0].lifecycle_status = "failed";
  refreshed.graph.nodes[0].error_summary = "provider unavailable";
  refreshed.work_items[0].objective = "Updated objective";
  refreshed.graph.edges[0].summary = "Updated evidence";
  const second = cached(refreshed, 800);
  assert.equal(second.groups, first.groups);
  assert.equal(second.ports, first.ports);
  assert.equal(second.junctions, first.junctions);
  assert.equal(second.nodes[0].node, refreshed.graph.nodes[0]);
  assert.equal(second.nodes[0].item, refreshed.work_items[0]);
  assert.deepEqual(second, buildExecutionGraphLayout(refreshed, 800));
  for (const [view, width, hidden, presentation] of [
    [refreshed, 400, new Set(), "icon"],
    [refreshed, 800, new Set(["attempt-child"]), "icon"],
    [refreshed, 800, new Set(), "summary"],
    [{ ...refreshed, graph: { ...refreshed.graph, edges: refreshed.graph.edges.slice(1) } }, 800, new Set(), "icon"],
  ]) {
    assert.deepEqual(cached(view, width, hidden, presentation), buildExecutionGraphLayout(view, width, hidden, presentation));
  }
});


test("retry shortcuts and local return corridors avoid intervening runs in both densities", async () => {
  const { buildExecutionGraphLayout } = await import("./execution-workgraph-layout.ts");
  const nodes = [
    { id: "owner", kind: "agent", visibility: "primary" },
    { id: "a", kind: "tool", parent_node_id: "owner" },
    { id: "b", kind: "subagent", parent_node_id: "owner" },
    { id: "c", kind: "tool", parent_node_id: "owner" },
    { id: "d", kind: "tool", parent_node_id: "b" },
    { id: "e", kind: "tool", parent_node_id: "b" },
  ].map((node, position) => ({ visibility: "nested", work_item_id: "", position, ...node }));
  const edges = nodes.filter((node) => node.parent_node_id).map((node) => ({
    id: `own:${node.id}`, source_node_id: node.parent_node_id, target_node_id: node.id,
    kind: node.kind === "subagent" ? "spawn" : "invoke",
  }));
  for (const [id, kind, from, to] of [
    ["skip", "retry", "a", "c"],
    ["adjacent", "retry", "d", "e"],
    ["return", "loop_back", "b", "owner"],
    ["nested-return", "loop_back", "d", "b"],
    ["self-return", "loop_back", "c", "c"],
  ]) edges.push({ id, kind, source_node_id: from, target_node_id: to });
  const view = { ...execution, work_items: [], graph: { nodes, edges } };
  for (const presentation of ["icon", "summary"]) {
    const layout = buildExecutionGraphLayout(view, 380, new Set(), presentation);
    const byId = Object.fromEntries(layout.nodes.map((node) => [node.node.id, node]));
    const group = layout.groups[0];
    for (const edge of layout.edges.filter((edge) => ["retry", "loop_back"].includes(edge.kind))) {
      const points = orthogonalPathPoints(edge.path);
      assert.doesNotMatch(edge.path, /[CQ]/);
      for (let i = 1; i < points.length; i++) {
        assert.ok(points[i].x === points[i - 1].x || points[i].y === points[i - 1].y);
      }
      for (const node of layout.nodes) {
        if (![edge.sourceId, edge.targetId].includes(node.node.id)) {
          assert.equal(orthogonalPathCrossesNode(edge.path, node), false, `${presentation}: ${edge.id} crosses ${node.node.id}`);
        }
      }
      for (const point of points) {
        assert.ok(point.x >= group.x + 16 && point.x <= group.x + group.width - 16);
        assert.ok(point.y >= group.y + 16 && point.y <= group.y + group.height - 16);
      }
      if (edge.id === "adjacent") assert.equal(points.length, 2);
      if (edge.id === "skip") assert.ok(points.length > 2, "a skipped sibling requires an under-row corridor");
      if (edge.id === "return") {
        assert.ok(Math.max(...points.map((point) => point.y)) < byId.d.y - byId.d.height / 2,
          "a return from a subagent leaves before its deeper children, without touring the group bottom");
      }
    }
    assert.deepEqual(buildExecutionGraphLayout({ ...view, graph: { nodes, edges: [...edges].reverse() } }, 380, new Set(), presentation), layout);
  }
});
