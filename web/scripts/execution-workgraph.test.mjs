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
    resolveExecutionGraphNodeStatus,
    resolveExecutionPrimaryAgentNodes,
    resolveExecutionNodeSummary,
    resolveExecutionWorkGraphHeaderModel,
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

test("WorkGraph sketch confirmation schedules a hidden background round without sending chat", async () => {
  const dialogSource = await readFile(path.join(
    webRoot,
    "src/features/conversation/shared/execution/workgraph-distillation-dialog.tsx",
  ), "utf8");
  const controllerSource = await readFile(path.join(
    webRoot,
    "src/features/conversation/shared/composer/controller/use-composer-controller.ts",
  ), "utf8");
  const apiSource = await readFile(path.join(
    webRoot,
    "src/lib/api/conversation/execution-api.ts",
  ), "utf8");
  assert.match(dialogSource, /scheduleWorkGraphWorkflowSaveApi\(sessionKey, workingPreview\.preview_id, \{/);
  assert.match(apiSource, /workgraph\/previews\/\$\{encodeURIComponent\(previewId\)\}\/save/);
  assert.doesNotMatch(dialogSource, /dispatchWorkGraphDistillationIntent|buildDistillationPrompt|onSendMessage/);
  assert.doesNotMatch(controllerSource, /WORKGRAPH_DISTILLATION_INTENT_EVENT|pendingWorkGraphPromptRef/);
});

test("WorkGraph Slash naming checks owner-scoped availability before save", async () => {
  const { MESSAGES } = await server.ssrLoadModule(
    "/src/shared/i18n/messages.ts",
  );
  const dialogSource = await readFile(path.join(
    webRoot,
    "src/features/conversation/shared/execution/workgraph-distillation-dialog.tsx",
  ), "utf8");
  const availabilitySource = await readFile(path.join(
    webRoot,
    "src/features/conversation/shared/execution/use-workgraph-slash-name-availability.ts",
  ), "utf8");
  const apiSource = await readFile(path.join(
    webRoot,
    "src/lib/api/conversation/execution-api.ts",
  ), "utf8");
  assert.match(dialogSource, /useWorkGraphSlashNameAvailability/);
  assert.match(dialogSource, /workflow_slash_unavailable/);
  assert.match(dialogSource, /ApiRequestError/);
  assert.match(dialogSource, /!slashNameAvailable/);
  assert.match(availabilitySource, /SLASH_NAME_CHECK_DELAY_MS = 350/);
  assert.match(availabilitySource, /new AbortController\(\)/);
  assert.match(apiSource, /workgraph\/workflows\/slash-name-availability/);
  assert.match(MESSAGES.zh["execution.workflow_slash_unavailable"], /已被占用/);
  assert.match(MESSAGES.zh["execution.workflow_slash_check_failed"], /暂时无法检查/);
});

test("WorkGraph sketch editor reuses DM and applies a validated graph revision", async () => {
  const { MESSAGES } = await server.ssrLoadModule(
    "/src/shared/i18n/messages.ts",
  );
  const editorSource = await readFile(path.join(
    webRoot,
    "src/features/conversation/shared/execution/workgraph-metadata-editor-dialog.tsx",
  ), "utf8");
  const dialogSource = await readFile(path.join(
    webRoot,
    "src/features/conversation/shared/execution/workgraph-distillation-dialog.tsx",
  ), "utf8");
  const panelModelSource = await readFile(path.join(
    webRoot,
    "src/features/conversation/room/dm/panel/controller/use-dm-chat-panel-model.ts",
  ), "utf8");
  const panelViewSource = await readFile(path.join(
    webRoot,
    "src/features/conversation/room/dm/panel/view/dm-chat-panel-view.tsx",
  ), "utf8");
  const feedSource = await readFile(path.join(
    webRoot,
    "src/features/conversation/shared/feed/conversation-feed.tsx",
  ), "utf8");
  const followScrollSource = await readFile(path.join(
    webRoot,
    "src/features/conversation/shared/timeline/scroll/use-follow-scroll.ts",
  ), "utf8");
  const apiSource = await readFile(path.join(
    webRoot,
    "src/lib/api/conversation/execution-api.ts",
  ), "utf8");
  assert.match(editorSource, /startWorkGraphWorkflowEditorApi/);
  assert.match(editorSource, /agents\.find\(\(item\) => item\.agent_id === editor\.agent_id\)/);
  assert.match(editorSource, /useAgentStore\(\(state\) => state\.agents\)/);
  assert.match(editorSource, /catalogAgents\.find\(\(item\) => item\.agent_id === editor\.agent_id\)/);
  assert.match(editorSource, /if \(!hasEditorAgent\) \{\s*await loadAgents\(\);\s*\}/);
  assert.doesNotMatch(editorSource, /\}, \[agents, locale, sessionKey, t, updateEditor\]\);/);
  assert.match(editorSource, /\}, \[loadAgents, sessionKey, startAttempt, updateEditor\]\);/);
  assert.doesNotMatch(editorSource, /getAgents\(\)/);
  assert.match(editorSource, /<DmChatPanel/);
  assert.match(editorSource, /embeddedEditor=/);
  assert.match(editorSource, /visibleAfterUnixMilli: editor\.display_after_unix_milli/);
  assert.match(editorSource, /execution\.workflow_editor_intro_title/);
  assert.match(editorSource, /examples: \[\]/);
  assert.match(editorSource, /examplesLabel: ""/);
  assert.match(editorSource, /footer: ""/);
  assert.doesNotMatch(editorSource, /workflow_editor_intro_(?:examples|example_|footer)/);
  assert.match(editorSource, /--conversation-composer-backdrop:var\(--surface-muted-background\)/);
  assert.match(panelModelSource, /embeddedEditor\.introduction/);
  assert.match(panelModelSource, /initialScrollAnchor: embeddedEditor \? "top" : "bottom"/);
  assert.match(panelModelSource, /liveContentAlignment: embeddedEditor \? "start" : "end"/);
  assert.match(panelViewSource, /data-embedded-editor-introduction/);
  assert.match(panelViewSource, /<ConversationFeed[\s\S]*leadingContent=/);
  assert.doesNotMatch(panelViewSource, /<EmbeddedEditorIntroduction[^>]*\/>\s*<ConversationFeed/);
  assert.match(feedSource, /props\.leadingContent == null/);
  assert.match(feedSource, /\{leadingContent\}\s*\{source\.roundIds\.map/);
  assert.match(followScrollSource, /isNewSession && initialScrollAnchor === "top"/);
  assert.ok(
    followScrollSource.indexOf('isNewSession && initialScrollAnchor === "top"')
      < followScrollSource.indexOf("if (shouldFollowLatestRef.current)"),
    "the editor's initial top anchor must reset prior FOLLOW/READING state before normal bottom following",
  );
  assert.doesNotMatch(panelViewSource, /send_message|sendMessage|messages\.push/);
  const introduction = MESSAGES.zh["execution.workflow_editor_intro_description"];
  assert.match(introduction, /告诉我需要怎么改/);
  assert.ok(introduction.length <= 28);
  assert.match(panelViewSource, /examples\.length \?/);
  assert.match(panelViewSource, /footer \?/);
  assert.doesNotMatch(editorSource, /<UiDialogHeader/);
  assert.match(editorSource, /<UiDialogCloseButton/);
  assert.doesNotMatch(editorSource, /<UiDialogFooter/);
  assert.match(editorSource, /applyWorkGraphWorkflowEditorApi/);
  assert.match(editorSource, /getWorkGraphWorkflowEditorApi/);
  assert.match(editorSource, /selectWorkGraphWorkflowEditorVersionApi/);
  assert.match(editorSource, /editor\.versions\.map/);
  assert.match(editorSource, /current\.selected_revision/);
  assert.doesNotMatch(editorSource, /closeWorkGraphWorkflowEditorApi/);
  assert.match(editorSource, /ExecutionWorkGraphCanvas/);
  assert.match(editorSource, /projectWorkGraphWorkflowCanvasExecution/);
  assert.match(editorSource, /<div className="flex min-h-0 flex-1">\s*<ExecutionWorkGraphCanvas/);
  assert.doesNotMatch(editorSource, /NamedWorkGraphSketch/);
  assert.ok(
    editorSource.indexOf("<DmChatPanel") < editorSource.indexOf("<ExecutionWorkGraphCanvas"),
    "editor should place the standard DM conversation before the shared Room/DM WorkGraph canvas",
  );
  assert.match(editorSource, /md:grid-cols-\[minmax\(360px,0\.42fr\)_minmax\(0,0\.58fr\)\]/);
  assert.doesNotMatch(editorSource, /workflow_editor_chat_label/);
  assert.doesNotMatch(editorSource, /\{currentPreview\.description\}/);
  assert.match(editorSource, /execution\.workflow_editor_retry/);
  assert.match(editorSource, /onApply\(applied\)/);
  assert.match(dialogSource, /setWorkingPreview\(nextPreview\)/);
  assert.match(dialogSource, /<WorkGraphWorkflowCanvasPreview/);
  assert.match(dialogSource, /workflow=\{workingPreview\}/);
  assert.doesNotMatch(editorSource, /messages\.map|reviseWorkGraphWorkflowMetadataApi/);
  assert.match(apiSource, /workgraph\/editors\/\$\{encodeURIComponent\(editorId\)\}\/apply/);
  assert.match(apiSource, /workgraph\/editors\/\$\{encodeURIComponent\(editorId\)\}\/versions\/select/);
  assert.doesNotMatch(apiSource, /workgraph\/editors\/\$\{encodeURIComponent\(editorId\)\}\/messages/);
});

test("Saved WorkGraph capability reopens the same Draft editor and schedules an update", async () => {
  const directorySource = await readFile(path.join(
    webRoot,
    "src/features/capability/workgraph-distillations/workgraph-distillations-directory.tsx",
  ), "utf8");
  const apiSource = await readFile(path.join(
    webRoot,
    "src/lib/api/conversation/execution-api.ts",
  ), "utf8");

  assert.match(directorySource, /previewSavedWorkGraphWorkflowApi/);
  assert.match(directorySource, /<WorkGraphMetadataEditorDialog/);
  assert.match(directorySource, /scheduleWorkGraphWorkflowSaveApi/);
  assert.match(directorySource, /capability\.workgraph_edit/);
  assert.match(directorySource, /!item\.built_in/);
  assert.match(directorySource, /capability\.workgraph_builtin/);
  assert.match(apiSource, /workgraph\/workflows\/\$\{encodeURIComponent\(workflowId\)\}\/preview/);
  assert.match(apiSource, /workgraph\/workflows\?\$\{query\.toString\(\)\}/);
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
  assert.deepEqual(
    addedLayout.ports.map((port) => ({
      edges: port.edgeIds,
      group: port.groupId,
      role: port.role,
      side: port.side,
    })),
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
    ],
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
    y: buildIncomingPort.y,
  });
  assert.deepEqual(orthogonalPathPoints(outgoingFrameEdge.path)[0], {
    x: buildOutgoingPort.x,
    y: buildOutgoingPort.y,
  });
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
    Math.abs(loopBackPoints.at(-1).x - agentLayout.x) < 0.5
      && Math.abs(
        loopBackPoints.at(-1).y
          - (agentLayout.y + agentLayout.size / 2),
      ) < 0.5,
    "the outer U-shaped return closes on the parent's normal process anchor",
  );
  assert.equal(
    orthogonalPathsShareSegment(loopBackLayout.path, forwardLayout.path),
    true,
    "an exact forward/return pair aligns on the target's center flow segment",
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
      && Math.abs(downwardRetryPoints.at(-1).x - downwardToolLayout.x) < 0.5
      && Math.abs(
        downwardRetryPoints.at(-1).y
          - (downwardToolLayout.y - downwardToolLayout.size / 2),
      ) < 0.5,
    "a downward retry first leaves above its source layer and closes on the target's normal process anchor",
  );
  assert.equal(
    orthogonalPathsShareSegment(
      downwardRetryEdge.path,
      downwardForwardEdge.path,
    ),
    true,
    "a downward retry aligns on the target's center flow segment",
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
    Math.abs(wideReturnPoints.at(-1).x - wideTargetLayout.x) < 0.5
      && Math.abs(
        wideReturnPoints.at(-1).y
          - (wideTargetLayout.y + wideTargetLayout.size / 2),
      ) < 0.5,
    "a wide return closes on the target's normal process anchor",
  );
  assert.equal(
    orthogonalPathsShareSegment(wideReturnEdge.path, wideForwardEdge.path),
    true,
    "a dense ownership fan still aligns the return with its target flow axis",
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
      true,
      `return ${returnEdge.id} aligns with its exact forward branch at the target anchor`,
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

test("WorkGraph review rounds keep stable depth when loop edges arrive first", async () => {
  const { buildExecutionGraphLayout } = await server.ssrLoadModule(
    "/src/features/conversation/shared/execution/execution-workgraph-layout.ts",
  );
  const reviewed = structuredClone(execution);
  reviewed.work_items = [reviewed.work_items.find((item) => item.id === "build")];
  reviewed.graph.nodes = [
    {
      id: "attempt-build-v1",
      kind: "agent",
      visibility: "primary",
      work_item_id: "build",
      attempt_id: "attempt-build-v1",
      agent_id: "builder",
      position: 1,
    },
    {
      id: "review:submission-v1",
      kind: "gate",
      visibility: "primary",
      work_item_id: "build",
      attempt_id: "attempt-build-v1",
      subject_id: "submission-v1",
      lifecycle_status: "rejected",
      position: 1,
    },
    {
      id: "build",
      kind: "agent",
      visibility: "primary",
      work_item_id: "build",
      attempt_id: "attempt-build-v2",
      agent_id: "builder",
      position: 1,
    },
    {
      id: "review:submission-v2",
      kind: "gate",
      visibility: "primary",
      work_item_id: "build",
      attempt_id: "attempt-build-v2",
      subject_id: "submission-v2",
      lifecycle_status: "changes_requested",
      position: 1,
    },
  ];
  // Deliberately put both control edges before their structural review edges.
  reviewed.graph.edges = [
    {
      id: "loop-current",
      kind: "loop_back",
      source_node_id: "review:submission-v2",
      target_node_id: "build",
    },
    {
      id: "loop-next-attempt",
      kind: "loop_back",
      source_node_id: "review:submission-v1",
      target_node_id: "build",
    },
    {
      id: "review-v2",
      kind: "review",
      source_node_id: "build",
      target_node_id: "review:submission-v2",
    },
    {
      id: "review-v1",
      kind: "review",
      source_node_id: "attempt-build-v1",
      target_node_id: "review:submission-v1",
    },
  ];
  const layout = buildExecutionGraphLayout(reviewed);
  const y = Object.fromEntries(layout.nodes.map((node) => [node.node.id, node.y]));
  assert.ok(y["attempt-build-v1"] < y["review:submission-v1"]);
  assert.ok(y["review:submission-v1"] < y.build);
  assert.ok(y.build < y["review:submission-v2"]);
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
  assert.deepEqual([...groupTrace.nodeIds], [
    "build",
    "attempt-child",
    "research",
    "integrate",
  ]);
  assert.deepEqual([...groupTrace.edgeIds], [
    "dependency:research:build",
    "spawn:build:attempt-child",
    "dependency:build:integrate",
  ]);
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

const nexusCommandTitles = [
  ["execution inspect", "读取工作图"],
  ["execution invoke --operation prepare_plan_execution", "封存计划提案"],
  ["execution invoke --operation plan_execution", "提交计划提案"],
  ["execution invoke --operation abandon_execution", "终止当前执行"],
  ["execution invoke --operation assign_work", "指派工作项"],
  ["execution invoke --operation submit_work", "提交交付物"],
  ["execution invoke --operation review_work", "验收工作项"],
  ["execution invoke --operation block_work", "标记工作阻塞"],
  ["execution invoke --operation resume_work", "恢复工作项"],
  ["execution invoke --operation take_over_work", "接管工作项"],
  ["execution invoke --operation audit_execution_alignment", "审计执行对齐"],
  ["execution invoke --operation promote_execution_to_goal", "升级为 Goal"],
  ["execution invoke --operation distill_workgraph", "保存工作图草图"],
  ["goal inspect", "读取 Goal"],
  ["goal invoke --operation create_goal", "创建 Goal"],
  ["goal invoke --operation retarget_goal", "调整 Goal 目标"],
  ["goal invoke --operation audit_objective_alignment", "审计 Goal 对齐"],
  ["goal invoke --operation update_goal", "更新 Goal 状态"],
];
