/**
 * INPUT: 权威 Execution Graph 节点/边与当前画布可用宽度。
 * OUTPUT: 保留 dependency/spawn/invoke/review/loop 语义、空间不足时才横向滚动的分层图坐标与边路径。
 * POS: 后端 Agent/Subagent/Tool/Gate Graph View 到交互画布之间的无状态布局投影。
 */
import type {
  ExecutionGraphEdgeKind,
  ExecutionGraphEdgeView,
  ExecutionGraphNodeView,
  ExecutionView,
  ExecutionWorkItemView,
} from "@/types/conversation/execution";

import {
  orderedExecutionGraphNodes,
  orderedExecutionItems,
} from "./execution-process-model";

const AGENT_NODE_SIZE = 48;
const SUBAGENT_NODE_SIZE = 38;
const PREFERRED_HORIZONTAL_STEP = 118;
const MIN_HORIZONTAL_STEP = 82;
const VERTICAL_STEP = 74;
const HORIZONTAL_PADDING = 36;
const VERTICAL_PADDING = 28;
const MIN_CANVAS_WIDTH = 340;
const MIN_CANVAS_HEIGHT = 136;

export interface ExecutionGraphNodeLayout {
  item: ExecutionWorkItemView | null;
  node: ExecutionGraphNodeView;
  size: number;
  x: number;
  y: number;
}

export interface ExecutionGraphEdgeLayout {
  id: string;
  kind: ExecutionGraphEdgeKind;
  path: string;
  sourceId: string;
  targetId: string;
}

export interface ExecutionGraphLayout {
  edges: ExecutionGraphEdgeLayout[];
  height: number;
  nodes: ExecutionGraphNodeLayout[];
  width: number;
}

export function buildExecutionGraphLayout(
  execution: ExecutionView,
  availableWidth?: number,
): ExecutionGraphLayout {
  const constrainedWidth = normalizeAvailableWidth(availableWidth);
  const graphNodes = orderedExecutionGraphNodes(execution);
  if (graphNodes.length === 0) {
    return {
      edges: [],
      height: MIN_CANVAS_HEIGHT,
      nodes: [],
      width: constrainedWidth === null
        ? MIN_CANVAS_WIDTH
        : Math.min(MIN_CANVAS_WIDTH, constrainedWidth),
    };
  }

  const visibleNodeIds = new Set(graphNodes.map((node) => node.id));
  const graphEdges = executionGraphEdges(execution).filter((edge) => (
    visibleNodeIds.has(edge.source_node_id)
    && visibleNodeIds.has(edge.target_node_id)
  ));
  const depthById = resolveGraphNodeDepths(graphNodes, graphEdges);
  const layers = new Map<number, ExecutionGraphNodeView[]>();
  let maxDepth = 0;
  for (const node of graphNodes) {
    const depth = depthById[node.id] ?? 0;
    maxDepth = Math.max(maxDepth, depth);
    const layer = layers.get(depth) ?? [];
    layer.push(node);
    layers.set(depth, layer);
  }

  const maxLayerSize = Math.max(
    1,
    ...Array.from(layers.values(), (layer) => layer.length),
  );
  const horizontalStep = resolveHorizontalStep(maxDepth, constrainedWidth);
  const naturalWidth = HORIZONTAL_PADDING * 2
    + AGENT_NODE_SIZE
    + maxDepth * horizontalStep;
  const naturalHeight = VERTICAL_PADDING * 2
    + AGENT_NODE_SIZE
    + (maxLayerSize - 1) * VERTICAL_STEP;
  const minimumWidth = constrainedWidth === null
    ? MIN_CANVAS_WIDTH
    : Math.min(MIN_CANVAS_WIDTH, constrainedWidth);
  const width = Math.max(minimumWidth, naturalWidth);
  const height = Math.max(MIN_CANVAS_HEIGHT, naturalHeight);
  const horizontalOffset = (width - naturalWidth) / 2;
  const itemById = new Map(
    orderedExecutionItems(execution).map((item) => [item.id, item]),
  );
  const nodes: ExecutionGraphNodeLayout[] = [];

  for (let depth = 0; depth <= maxDepth; depth += 1) {
    const layer = layers.get(depth) ?? [];
    const layerHeight = (layer.length - 1) * VERTICAL_STEP;
    layer.forEach((node, index) => {
      nodes.push({
        item: itemById.get(node.work_item_id) ?? null,
        node,
        size: node.kind === "subagent" || node.kind === "tool"
          ? SUBAGENT_NODE_SIZE
          : AGENT_NODE_SIZE,
        x: horizontalOffset
          + HORIZONTAL_PADDING
          + AGENT_NODE_SIZE / 2
          + depth * horizontalStep,
        y: height / 2 - layerHeight / 2 + index * VERTICAL_STEP,
      });
    });
  }

  const nodeById = new Map(nodes.map((node) => [node.node.id, node]));
  const edges: ExecutionGraphEdgeLayout[] = [];
  for (const edge of graphEdges) {
    const source = nodeById.get(edge.source_node_id);
    const target = nodeById.get(edge.target_node_id);
    if (!source || !target) {
      continue;
    }
    edges.push({
      id: edge.id,
      kind: edge.kind,
      path: buildEdgePath(source, target),
      sourceId: edge.source_node_id,
      targetId: edge.target_node_id,
    });
  }

  return { edges, height, nodes, width };
}

function executionGraphEdges(execution: ExecutionView): ExecutionGraphEdgeView[] {
  const projected = execution.graph?.edges ?? [];
  if (projected.length > 0) {
    return projected;
  }
  return orderedExecutionItems(execution).flatMap((item) => (
    (item.dependency_ids ?? []).map((dependencyId) => ({
      id: `dependency:${dependencyId}:${item.id}`,
      kind: "dependency" as const,
      source_node_id: dependencyId,
      target_node_id: item.id,
    }))
  ));
}

function resolveGraphNodeDepths(
  nodes: ExecutionGraphNodeView[],
  edges: ExecutionGraphEdgeView[],
): Record<string, number> {
  const nodeIds = new Set(nodes.map((node) => node.id));
  const upstreamByNodeId = new Map<string, string[]>();
  for (const edge of edges) {
    if (edge.kind === "loop_back") {
      continue;
    }
    if (!nodeIds.has(edge.source_node_id) || !nodeIds.has(edge.target_node_id)) {
      continue;
    }
    const upstream = upstreamByNodeId.get(edge.target_node_id) ?? [];
    upstream.push(edge.source_node_id);
    upstreamByNodeId.set(edge.target_node_id, upstream);
  }
  const result: Record<string, number> = {};
  const resolveDepth = (nodeId: string, visiting: Set<string>): number => {
    if (result[nodeId] !== undefined) {
      return result[nodeId];
    }
    if (visiting.has(nodeId)) {
      return 0;
    }
    const nextVisiting = new Set(visiting).add(nodeId);
    let depth = 0;
    for (const upstreamId of upstreamByNodeId.get(nodeId) ?? []) {
      depth = Math.max(depth, resolveDepth(upstreamId, nextVisiting) + 1);
    }
    result[nodeId] = depth;
    return depth;
  };
  for (const node of nodes) {
    resolveDepth(node.id, new Set());
  }
  return result;
}

function normalizeAvailableWidth(width: number | undefined): number | null {
  if (width === undefined || !Number.isFinite(width) || width <= 0) {
    return null;
  }
  return Math.floor(width);
}

function resolveHorizontalStep(
  maxDepth: number,
  availableWidth: number | null,
): number {
  if (maxDepth === 0 || availableWidth === null) {
    return PREFERRED_HORIZONTAL_STEP;
  }
  const fittingStep = (
    availableWidth - HORIZONTAL_PADDING * 2 - AGENT_NODE_SIZE
  ) / maxDepth;
  return Math.max(
    MIN_HORIZONTAL_STEP,
    Math.min(PREFERRED_HORIZONTAL_STEP, fittingStep),
  );
}

function buildEdgePath(
  source: ExecutionGraphNodeLayout,
  target: ExecutionGraphNodeLayout,
): string {
  if (target.x <= source.x) {
    const sourceX = source.x - source.size / 2;
    const targetX = target.x + target.size / 2;
    const arcY = Math.min(source.y, target.y) - 38;
    return [
      `M ${sourceX} ${source.y}`,
      `C ${sourceX - 24} ${arcY}`,
      `${targetX + 24} ${arcY}`,
      `${targetX} ${target.y}`,
    ].join(" ");
  }
  const sourceX = source.x + source.size / 2;
  const targetX = target.x - target.size / 2;
  const curve = Math.max(14, (targetX - sourceX) * 0.42);
  return [
    `M ${sourceX} ${source.y}`,
    `C ${sourceX + curve} ${source.y}`,
    `${targetX - curve} ${target.y}`,
    `${targetX} ${target.y}`,
  ].join(" ");
}
