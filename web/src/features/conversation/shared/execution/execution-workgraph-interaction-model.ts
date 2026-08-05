/**
 * INPUT: 权威 Execution Graph 与用户本地的折叠、搜索、缩放意图。
 * OUTPUT: 可隐藏节点、祖先路径、全文检索结果和有界 viewport 比例。
 * POS: WorkGraph 画布的纯交互模型；不改变后端拓扑、节点状态或 Agent 路线。
 */
import type {
  ExecutionGraphNodeView,
  ExecutionView,
} from "@/types/conversation/execution";

import {
  orderedExecutionGraphNodes,
  resolveExecutionGraphNodeItem,
} from "./execution-process-model";

const EXECUTION_GRAPH_MIN_ZOOM = 0.5;
const EXECUTION_GRAPH_MAX_ZOOM = 1.5;
export const EXECUTION_GRAPH_ZOOM_STEP = 0.1;

interface ExecutionGraphCollapseProjection {
  descendantCountByNodeId: Map<string, number>;
  hiddenNodeIds: Set<string>;
}

interface ExecutionGraphHierarchy {
  childrenByNodeId: Map<string, string[]>;
  parentByNodeId: Map<string, string>;
}

export function projectExecutionGraphCollapse(
  execution: ExecutionView,
  collapsedNodeIds: ReadonlySet<string>,
): ExecutionGraphCollapseProjection {
  const nodes = orderedExecutionGraphNodes(execution);
  const hierarchy = buildExecutionGraphHierarchy(execution, nodes);
  const descendantCountByNodeId = new Map<string, number>();
  const hiddenNodeIds = new Set<string>();
  for (const node of nodes) {
    const descendants = executionGraphDescendants(node.id, hierarchy);
    if (descendants.length > 0) {
      descendantCountByNodeId.set(node.id, descendants.length);
    }
    if (collapsedNodeIds.has(node.id)) {
      for (const descendantId of descendants) {
        hiddenNodeIds.add(descendantId);
      }
    }
  }
  return { descendantCountByNodeId, hiddenNodeIds };
}

export function resolveExecutionGraphNodeAncestors(
  execution: ExecutionView,
  nodeId: string,
): string[] {
  const nodes = orderedExecutionGraphNodes(execution);
  const hierarchy = buildExecutionGraphHierarchy(execution, nodes);
  const result: string[] = [];
  const visited = new Set<string>([nodeId]);
  let parentId = hierarchy.parentByNodeId.get(nodeId);
  while (parentId && !visited.has(parentId)) {
    result.push(parentId);
    visited.add(parentId);
    parentId = hierarchy.parentByNodeId.get(parentId);
  }
  return result;
}

export function searchExecutionGraphNodes(
  execution: ExecutionView,
  query: string,
): string[] {
  const normalized = normalizeSearchText(query);
  if (!normalized) {
    return [];
  }
  return orderedExecutionGraphNodes(execution)
    .filter((node) => executionGraphNodeSearchText(execution, node).includes(normalized))
    .map((node) => node.id);
}

export function nextExecutionGraphSearchResult(
  resultIds: readonly string[],
  currentId: string | null,
  direction: -1 | 1,
): string | null {
  if (resultIds.length === 0) {
    return null;
  }
  const currentIndex = currentId ? resultIds.indexOf(currentId) : -1;
  if (currentIndex < 0) {
    return direction > 0 ? resultIds[0] : resultIds[resultIds.length - 1];
  }
  return resultIds[(currentIndex + direction + resultIds.length) % resultIds.length];
}

export function clampExecutionGraphZoom(value: number): number {
  if (!Number.isFinite(value)) {
    return 1;
  }
  return Math.min(
    EXECUTION_GRAPH_MAX_ZOOM,
    Math.max(EXECUTION_GRAPH_MIN_ZOOM, Math.round(value * 100) / 100),
  );
}

export function resolveExecutionGraphFitZoom({
  contentHeight,
  contentWidth,
  viewportHeight,
  viewportWidth,
}: {
  contentHeight: number;
  contentWidth: number;
  viewportHeight: number;
  viewportWidth: number;
}): number {
  if (contentHeight <= 0 || contentWidth <= 0) {
    return 1;
  }
  const horizontal = Math.max(0, viewportWidth - 24) / contentWidth;
  const vertical = Math.max(0, viewportHeight - 24) / contentHeight;
  return clampExecutionGraphZoom(Math.min(1, horizontal, vertical));
}

export function resolveExecutionWorkspaceReference(value: string): string | null {
  const normalized = value.trim().replaceAll("\\", "/");
  if (
    !normalized
    || normalized.startsWith("/")
    || normalized.startsWith("~")
    || /^[a-z][a-z0-9+.-]*:/i.test(normalized)
  ) {
    return null;
  }
  const parts = normalized.split("/");
  if (parts.some((part) => part === ".." || part === "")) {
    return null;
  }
  return normalized.replace(/^\.\//, "");
}

function buildExecutionGraphHierarchy(
  execution: ExecutionView,
  nodes: readonly ExecutionGraphNodeView[],
): ExecutionGraphHierarchy {
  const nodeById = new Map(nodes.map((node) => [node.id, node]));
  const parentByNodeId = new Map<string, string>();
  for (const node of nodes) {
    if (node.parent_node_id && nodeById.has(node.parent_node_id)) {
      parentByNodeId.set(node.id, node.parent_node_id);
    }
  }
  for (const edge of execution.graph?.edges ?? []) {
    if (
      parentByNodeId.has(edge.target_node_id)
      || !nodeById.has(edge.source_node_id)
      || !nodeById.has(edge.target_node_id)
      || !["spawn", "invoke", "guard"].includes(edge.kind)
    ) {
      continue;
    }
    parentByNodeId.set(edge.target_node_id, edge.source_node_id);
  }
  const childrenByNodeId = new Map<string, string[]>();
  for (const [childId, parentId] of parentByNodeId) {
    const children = childrenByNodeId.get(parentId) ?? [];
    children.push(childId);
    childrenByNodeId.set(parentId, children);
  }
  return { childrenByNodeId, parentByNodeId };
}

function executionGraphDescendants(
  nodeId: string,
  hierarchy: ExecutionGraphHierarchy,
): string[] {
  const result: string[] = [];
  const pending = [...(hierarchy.childrenByNodeId.get(nodeId) ?? [])];
  const visited = new Set<string>([nodeId]);
  while (pending.length > 0) {
    const current = pending.shift();
    if (!current || visited.has(current)) {
      continue;
    }
    visited.add(current);
    result.push(current);
    pending.push(...(hierarchy.childrenByNodeId.get(current) ?? []));
  }
  return result;
}

function executionGraphNodeSearchText(
  execution: ExecutionView,
  node: ExecutionGraphNodeView,
): string {
  const item = resolveExecutionGraphNodeItem(execution, node);
  const values: string[] = [
    node.id,
    node.kind,
    node.name ?? "",
    node.description ?? "",
    node.agent_id ?? "",
    node.subject_id ?? "",
    node.lifecycle_status ?? "",
    node.result_summary ?? "",
    node.error_code ?? "",
    node.error_summary ?? "",
    item?.subject ?? "",
    item?.objective ?? "",
    item?.deliverable ?? "",
    item?.block_reason ?? "",
    item?.needed_input ?? "",
    ...(item?.input_refs ?? []),
    ...(item?.submission?.result_refs ?? []),
    ...(item?.submission?.evidence ?? []),
  ];
  for (const run of node.runs ?? []) {
    values.push(
      run.id,
      run.status ?? "",
      run.result_summary ?? "",
      run.error_code ?? "",
      run.error_summary ?? "",
      ...(run.artifacts ?? []).flatMap((artifact) => [
        artifact.path,
        artifact.display_path ?? "",
        artifact.title ?? "",
        artifact.label ?? "",
      ]),
    );
  }
  for (const criterion of item?.acceptance?.criteria_results ?? []) {
    values.push(criterion.criterion, criterion.note ?? "", ...(criterion.evidence ?? []));
  }
  return normalizeSearchText(values.join(" "));
}

function normalizeSearchText(value: string): string {
  return value.trim().toLocaleLowerCase();
}
