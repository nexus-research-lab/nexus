/**
 * INPUT: 不含运行事实的命名 WorkGraph workflow 或临时 preview。
 * OUTPUT: 仅供共用 ExecutionWorkGraphCanvas 渲染的中性 ExecutionView 投影。
 * POS: 命名草图协议到 Room/DM 工作图画布的唯一适配层；不伪造 Agent、Attempt、Tool、Submission 或验收事实。
 */

import type {
  ExecutionGraphEdgeView,
  ExecutionGraphNodeView,
  ExecutionView,
  ExecutionWorkItemView,
} from "@/types/conversation/execution";
import type {
  WorkGraphWorkflow,
  WorkGraphWorkflowDependency,
  WorkGraphWorkflowPreview,
} from "@/types/conversation/workgraph-workflow";

const WORKFLOW_PREVIEW_TIMESTAMP = "1970-01-01T00:00:00.000Z";

export function projectWorkGraphWorkflowCanvasExecution(
  preview: WorkGraphWorkflow | WorkGraphWorkflowPreview,
  revision: number,
): ExecutionView {
  const workflowIdentity = "preview_id" in preview ? preview.preview_id : preview.id;
  const nodeIdByLogicalKey = new Map(
    preview.nodes.map((node) => [node.logical_key, workflowNodeId(node.logical_key)]),
  );
  const dependencies = preview.dependencies ?? [];
  const workItems = preview.nodes.map<ExecutionWorkItemView>((node) => ({
    id: nodeIdByLogicalKey.get(node.logical_key) ?? workflowNodeId(node.logical_key),
    logical_key: node.logical_key,
    kind: node.kind,
    subject: node.subject,
    objective: node.objective,
    deliverable: node.deliverable,
    acceptance_criteria: node.acceptance_criteria,
    dependency_ids: dependencyIdsForNode(
      node.logical_key,
      dependencies,
      nodeIdByLogicalKey,
    ),
    parent_work_item_id: node.parent_logical_key
      ? nodeIdByLogicalKey.get(node.parent_logical_key)
      : undefined,
    required: node.required,
    terminal: node.terminal,
    position: node.position,
    status: "waiting",
    updated_at: WORKFLOW_PREVIEW_TIMESTAMP,
  }));
  const graphNodes = preview.nodes.map<ExecutionGraphNodeView>((node) => {
    const id = nodeIdByLogicalKey.get(node.logical_key)
      ?? workflowNodeId(node.logical_key);
    return {
      id,
      kind: node.kind === "review" || node.kind === "verify" ? "gate" : "agent",
      visibility: "primary",
      work_item_id: id,
      parent_node_id: node.parent_logical_key
        ? nodeIdByLogicalKey.get(node.parent_logical_key)
        : undefined,
      name: node.subject,
      description: node.objective,
      responsibility_status: "waiting",
      position: node.position,
    };
  });
  const graphEdges = dependencies.flatMap<ExecutionGraphEdgeView>((dependency) => {
    const source = nodeIdByLogicalKey.get(dependency.depends_on_logical_key);
    const target = nodeIdByLogicalKey.get(dependency.logical_key);
    if (!source || !target) return [];
    return [{
      id: `workflow-edge:${dependency.depends_on_logical_key}:${dependency.logical_key}`,
      kind: "dependency",
      source_node_id: source,
      target_node_id: target,
    }];
  });
  const requiredCount = preview.nodes.filter((node) => node.required).length;

  return {
    id: `${workflowIdentity}:revision:${revision}`,
    session_key: preview.source_session_key,
    scope_kind: preview.source_session_key.startsWith("room:") ? "room" : "dm",
    objective: preview.objective,
    completion_criteria: preview.completion_criteria,
    status: "active",
    version: revision,
    plan: {
      id: `${workflowIdentity}:plan`,
      revision,
      status: "active",
      created_at: WORKFLOW_PREVIEW_TIMESTAMP,
      activated_at: WORKFLOW_PREVIEW_TIMESTAMP,
    },
    progress: {
      total: preview.nodes.length,
      required: requiredCount,
      accepted: 0,
      running: 0,
      blocked: 0,
      submitted: 0,
      ready: 0,
      waiting: preview.nodes.length,
      changes_requested: 0,
      failed: 0,
      cancelled: 0,
    },
    work_items: workItems,
    graph: {
      nodes: graphNodes,
      edges: graphEdges,
      runtime_node_total: graphNodes.length,
      runtime_edge_total: graphEdges.length,
      runtime_nodes_truncated: false,
      runtime_edges_truncated: false,
    },
    created_at: WORKFLOW_PREVIEW_TIMESTAMP,
    updated_at: WORKFLOW_PREVIEW_TIMESTAMP,
  };
}

function dependencyIdsForNode(
  logicalKey: string,
  dependencies: readonly WorkGraphWorkflowDependency[],
  nodeIdByLogicalKey: ReadonlyMap<string, string>,
): string[] | undefined {
  const ids = dependencies
    .filter((dependency) => dependency.logical_key === logicalKey)
    .map((dependency) => nodeIdByLogicalKey.get(dependency.depends_on_logical_key))
    .filter((id): id is string => Boolean(id));
  return ids.length > 0 ? ids : undefined;
}

function workflowNodeId(logicalKey: string): string {
  return `workflow-node:${logicalKey}`;
}
