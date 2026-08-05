/**
 * INPUT: 权威 Execution Graph 节点/边、当前画布可用宽度与纯 UI 隐藏节点集合。
 * OUTPUT: 主责任图横向展开、Agent 内部运行节点向下形成有界子图，并保证有父身份的可见节点始终带方向边。
 * POS: 后端 Agent/Subagent/Tool/Gate Graph View 到交互画布之间的无状态分层子图投影。
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
const NESTED_NODE_SIZE = 38;
const PREFERRED_MAIN_GAP = 72;
const MIN_MAIN_GAP = 44;
const MAIN_LAYER_VERTICAL_GAP = 36;
const NESTED_HORIZONTAL_GAP = 16;
const NESTED_VERTICAL_GAP = 46;
const GROUP_PADDING = 12;
const HORIZONTAL_PADDING = 24;
const VERTICAL_PADDING = 24;
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

export interface ExecutionGraphGroupLayout {
  height: number;
  id: string;
  nodeIds: string[];
  width: number;
  x: number;
  y: number;
}

export interface ExecutionGraphLayout {
  edges: ExecutionGraphEdgeLayout[];
  groups: ExecutionGraphGroupLayout[];
  height: number;
  nodes: ExecutionGraphNodeLayout[];
  width: number;
}

interface ExecutionGraphCluster {
  height: number;
  nodes: ExecutionGraphNodeView[];
  positions: Map<string, { x: number; y: number }>;
  root: ExecutionGraphNodeView;
  rootY: number;
  width: number;
}

export function buildExecutionGraphLayout(
  execution: ExecutionView,
  availableWidth?: number,
  hiddenNodeIds: ReadonlySet<string> = new Set(),
): ExecutionGraphLayout {
  const constrainedWidth = normalizeAvailableWidth(availableWidth);
  const graphNodes = orderedExecutionGraphNodes(execution).filter(
    (node) => !hiddenNodeIds.has(node.id),
  );
  if (graphNodes.length === 0) {
    return {
      edges: [],
      groups: [],
      height: MIN_CANVAS_HEIGHT,
      nodes: [],
      width: constrainedWidth === null
        ? MIN_CANVAS_WIDTH
        : Math.min(MIN_CANVAS_WIDTH, constrainedWidth),
    };
  }

  const graphEdges = executionGraphEdges(execution, graphNodes);
  const rootByNodeId = resolveClusterRoots(graphNodes, graphEdges);
  const nodesByRoot = new Map<string, ExecutionGraphNodeView[]>();
  for (const node of graphNodes) {
    const rootId = rootByNodeId.get(node.id) ?? node.id;
    const members = nodesByRoot.get(rootId) ?? [];
    members.push(node);
    nodesByRoot.set(rootId, members);
  }
  const graphNodeById = new Map(graphNodes.map((node) => [node.id, node]));
  const clusters = Array.from(nodesByRoot, ([rootId, members]) => (
    buildExecutionGraphCluster(
      graphNodeById.get(rootId) ?? members[0],
      members,
      graphEdges,
      rootByNodeId,
    )
  ));
  const clusterEdges = collapseClusterEdges(graphEdges, rootByNodeId);
  const depthById = resolveGraphNodeDepths(
    clusters.map((cluster) => cluster.root),
    clusterEdges,
  );
  const layers = new Map<number, ExecutionGraphCluster[]>();
  let maxDepth = 0;
  for (const cluster of clusters) {
    const depth = depthById[cluster.root.id] ?? 0;
    maxDepth = Math.max(maxDepth, depth);
    const layer = layers.get(depth) ?? [];
    layer.push(cluster);
    layers.set(depth, layer);
  }
  for (const layer of layers.values()) {
    layer.sort((left, right) => (
      left.root.position - right.root.position
      || left.root.id.localeCompare(right.root.id)
    ));
  }

  const layerWidths = Array.from({ length: maxDepth + 1 }, (_, depth) => (
    Math.max(0, ...(layers.get(depth) ?? []).map((cluster) => cluster.width))
  ));
  const layerHeights = Array.from({ length: maxDepth + 1 }, (_, depth) => {
    const layer = layers.get(depth) ?? [];
    return layer.reduce((total, cluster, index) => (
      total + cluster.height + (index === 0 ? 0 : MAIN_LAYER_VERTICAL_GAP)
    ), 0);
  });
  const mainGap = resolveMainGap(layerWidths, constrainedWidth);
  const contentWidth = layerWidths.reduce((total, value) => total + value, 0)
    + maxDepth * mainGap;
  const maxRootY = Math.max(...clusters.map((cluster) => cluster.rootY));
  const maxBelowRoot = Math.max(
    ...clusters.map((cluster) => cluster.height - cluster.rootY),
  );
  const alignedHeight = maxRootY + maxBelowRoot;
  const stackedHeight = Math.max(...layerHeights);
  const naturalWidth = HORIZONTAL_PADDING * 2 + contentWidth;
  const naturalHeight = VERTICAL_PADDING * 2
    + Math.max(alignedHeight, stackedHeight);
  const minimumWidth = constrainedWidth === null
    ? MIN_CANVAS_WIDTH
    : Math.min(MIN_CANVAS_WIDTH, constrainedWidth);
  const width = Math.max(minimumWidth, naturalWidth);
  const height = Math.max(MIN_CANVAS_HEIGHT, naturalHeight);
  const horizontalOffset = (width - contentWidth) / 2;
  const alignedRootY = VERTICAL_PADDING + maxRootY;
  const itemById = new Map(
    orderedExecutionItems(execution).map((item) => [item.id, item]),
  );
  const absolutePositionById = new Map<string, { x: number; y: number }>();
  const groups: ExecutionGraphGroupLayout[] = [];
  let layerLeft = horizontalOffset;

  for (let depth = 0; depth <= maxDepth; depth += 1) {
    const layer = layers.get(depth) ?? [];
    const layerWidth = layerWidths[depth];
    let clusterTop = layer.length === 1
      ? alignedRootY - layer[0].rootY
      : (height - layerHeights[depth]) / 2;
    for (const cluster of layer) {
      const clusterLeft = layerLeft + (layerWidth - cluster.width) / 2;
      for (const node of cluster.nodes) {
        const relative = cluster.positions.get(node.id);
        if (relative) {
          absolutePositionById.set(node.id, {
            x: clusterLeft + relative.x,
            y: clusterTop + relative.y,
          });
        }
      }
      if (cluster.nodes.length > 1) {
        groups.push({
          height: cluster.height,
          id: cluster.root.id,
          nodeIds: cluster.nodes.map((node) => node.id),
          width: cluster.width,
          x: clusterLeft,
          y: clusterTop,
        });
      }
      clusterTop += cluster.height + MAIN_LAYER_VERTICAL_GAP;
    }
    layerLeft += layerWidth + mainGap;
  }

  const nodes = graphNodes.map((node) => {
    const point = absolutePositionById.get(node.id) ?? {
      x: width / 2,
      y: height / 2,
    };
    return {
      item: itemById.get(node.work_item_id) ?? null,
      node,
      size: graphNodeSize(node),
      x: point.x,
      y: point.y,
    };
  });
  const layoutNodeById = new Map(nodes.map((node) => [node.node.id, node]));
  const edges: ExecutionGraphEdgeLayout[] = [];
  for (const edge of graphEdges) {
    const source = layoutNodeById.get(edge.source_node_id);
    const target = layoutNodeById.get(edge.target_node_id);
    if (!source || !target) {
      continue;
    }
    edges.push({
      id: edge.id,
      kind: edge.kind,
      path: rootByNodeId.get(source.node.id) === rootByNodeId.get(target.node.id)
        && !isExecutionControlEdge(edge.kind)
        ? buildNestedEdgePath(source, target)
        : isExecutionControlEdge(edge.kind)
        ? buildControlEdgePath(source, target)
        : buildEdgePath(source, target),
      sourceId: edge.source_node_id,
      targetId: edge.target_node_id,
    });
  }

  return { edges, groups, height, nodes, width };
}

function executionGraphEdges(
  execution: ExecutionView,
  nodes: ExecutionGraphNodeView[],
): ExecutionGraphEdgeView[] {
  const projected = execution.graph?.edges ?? [];
  const result = projected.length > 0
    ? [...projected]
    : orderedExecutionItems(execution).flatMap((item) => (
      (item.dependency_ids ?? []).map((dependencyId) => ({
      id: `dependency:${dependencyId}:${item.id}`,
      kind: "dependency" as const,
      source_node_id: dependencyId,
      target_node_id: item.id,
      }))
    ));
  const nodeById = new Map(nodes.map((node) => [node.id, node]));
  const visibleNodeIds = new Set(nodeById.keys());
  const filtered = result.filter((edge) => (
    visibleNodeIds.has(edge.source_node_id)
    && visibleNodeIds.has(edge.target_node_id)
  ));
  const incomingNodeIds = new Set(
    filtered
      .filter((edge) => !isExecutionControlEdge(edge.kind))
      .map((edge) => edge.target_node_id),
  );
  for (const node of nodes) {
    if (node.visibility === "primary" || incomingNodeIds.has(node.id)) {
      continue;
    }
    const parentId = resolveVisibleParentNodeId(node, nodes, nodeById);
    if (!parentId || parentId === node.id) {
      continue;
    }
    const kind = nestedEdgeKind(node);
    filtered.push({
      id: `derived:${kind}:${parentId}:${node.id}`,
      kind,
      source_node_id: parentId,
      target_node_id: node.id,
    });
    incomingNodeIds.add(node.id);
  }
  return filtered;
}

function resolveVisibleParentNodeId(
  node: ExecutionGraphNodeView,
  nodes: ExecutionGraphNodeView[],
  nodeById: Map<string, ExecutionGraphNodeView>,
): string | null {
  if (node.parent_node_id && nodeById.has(node.parent_node_id)) {
    return node.parent_node_id;
  }
  if (node.agent_round_id) {
    const exactRound = nodes.find((candidate) => (
      candidate.kind === "agent"
      && candidate.id !== node.id
      && candidate.agent_round_id === node.agent_round_id
    ));
    if (exactRound) {
      return exactRound.id;
    }
  }
  const workAgents = nodes.filter((candidate) => (
    candidate.kind === "agent"
    && candidate.id !== node.id
    && candidate.work_item_id === node.work_item_id
    && (!node.agent_id || candidate.agent_id === node.agent_id)
  ));
  return workAgents.length === 1 ? workAgents[0].id : null;
}

function nestedEdgeKind(
  node: ExecutionGraphNodeView,
): ExecutionGraphEdgeKind {
  if (node.kind === "subagent") {
    return "spawn";
  }
  if (node.kind === "tool") {
    return "invoke";
  }
  if (node.kind === "gate") {
    return "guard";
  }
  return "dependency";
}

function resolveClusterRoots(
  nodes: ExecutionGraphNodeView[],
  edges: ExecutionGraphEdgeView[],
): Map<string, string> {
  const nodeById = new Map(nodes.map((node) => [node.id, node]));
  const parentById = new Map<string, string>();
  for (const node of nodes) {
    if (node.parent_node_id && nodeById.has(node.parent_node_id)) {
      parentById.set(node.id, node.parent_node_id);
    }
  }
  for (const edge of edges) {
    const target = nodeById.get(edge.target_node_id);
    if (!target || target.visibility === "primary" || isExecutionControlEdge(edge.kind)) {
      continue;
    }
    parentById.set(target.id, edge.source_node_id);
  }
  const result = new Map<string, string>();
  const resolveRoot = (nodeId: string, visiting: Set<string>): string => {
    const cached = result.get(nodeId);
    if (cached) {
      return cached;
    }
    const node = nodeById.get(nodeId);
    if (!node || node.visibility === "primary" || visiting.has(nodeId)) {
      result.set(nodeId, nodeId);
      return nodeId;
    }
    const parentId = parentById.get(nodeId);
    if (!parentId || !nodeById.has(parentId)) {
      result.set(nodeId, nodeId);
      return nodeId;
    }
    const rootId = resolveRoot(parentId, new Set(visiting).add(nodeId));
    result.set(nodeId, rootId);
    return rootId;
  };
  for (const node of nodes) {
    resolveRoot(node.id, new Set());
  }
  return result;
}

function buildExecutionGraphCluster(
  root: ExecutionGraphNodeView,
  members: ExecutionGraphNodeView[],
  edges: ExecutionGraphEdgeView[],
  rootByNodeId: Map<string, string>,
): ExecutionGraphCluster {
  const internalEdges = edges.filter((edge) => (
    rootByNodeId.get(edge.source_node_id) === root.id
    && rootByNodeId.get(edge.target_node_id) === root.id
  ));
  const depthById = resolveGraphNodeDepths(members, internalEdges);
  const layers = new Map<number, ExecutionGraphNodeView[]>();
  let maxDepth = 0;
  for (const node of members) {
    const depth = node.id === root.id ? 0 : Math.max(1, depthById[node.id] ?? 1);
    maxDepth = Math.max(maxDepth, depth);
    const layer = layers.get(depth) ?? [];
    layer.push(node);
    layers.set(depth, layer);
  }
  for (const layer of layers.values()) {
    layer.sort((left, right) => (
      left.position - right.position
      || left.id.localeCompare(right.id)
    ));
  }
  const padded = members.length > 1;
  const padding = padded ? GROUP_PADDING : 0;
  const layerWidths: number[] = [];
  const layerHeights: number[] = [];
  for (let depth = 0; depth <= maxDepth; depth += 1) {
    const layer = layers.get(depth) ?? [];
    layerWidths[depth] = layer.reduce((total, node, index) => (
      total + graphNodeSize(node) + (index === 0 ? 0 : NESTED_HORIZONTAL_GAP)
    ), 0);
    layerHeights[depth] = Math.max(0, ...layer.map(graphNodeSize));
  }
  const contentWidth = Math.max(...layerWidths);
  const contentHeight = layerHeights.reduce((total, value) => total + value, 0)
    + maxDepth * NESTED_VERTICAL_GAP;
  const width = contentWidth + padding * 2;
  const height = contentHeight + padding * 2;
  const positions = new Map<string, { x: number; y: number }>();
  let layerTop = padding;
  for (let depth = 0; depth <= maxDepth; depth += 1) {
    const layer = layers.get(depth) ?? [];
    let nodeLeft = (width - layerWidths[depth]) / 2;
    for (const node of layer) {
      const size = graphNodeSize(node);
      positions.set(node.id, {
        x: nodeLeft + size / 2,
        y: layerTop + layerHeights[depth] / 2,
      });
      nodeLeft += size + NESTED_HORIZONTAL_GAP;
    }
    layerTop += layerHeights[depth] + NESTED_VERTICAL_GAP;
  }
  return {
    height,
    nodes: members,
    positions,
    root,
    rootY: positions.get(root.id)?.y ?? height / 2,
    width,
  };
}

function collapseClusterEdges(
  edges: ExecutionGraphEdgeView[],
  rootByNodeId: Map<string, string>,
): ExecutionGraphEdgeView[] {
  const result: ExecutionGraphEdgeView[] = [];
  const seen = new Set<string>();
  for (const edge of edges) {
    const sourceId = rootByNodeId.get(edge.source_node_id) ?? edge.source_node_id;
    const targetId = rootByNodeId.get(edge.target_node_id) ?? edge.target_node_id;
    if (sourceId === targetId) {
      continue;
    }
    const key = `${edge.kind}:${sourceId}:${targetId}`;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    result.push({
      ...edge,
      id: `cluster:${key}`,
      source_node_id: sourceId,
      target_node_id: targetId,
    });
  }
  return result;
}

function resolveGraphNodeDepths(
  nodes: ExecutionGraphNodeView[],
  edges: ExecutionGraphEdgeView[],
): Record<string, number> {
  const nodeIds = new Set(nodes.map((node) => node.id));
  const upstreamByNodeId = new Map<string, string[]>();
  for (const edge of edges) {
    if (isExecutionControlEdge(edge.kind)) {
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

function resolveMainGap(
  layerWidths: number[],
  availableWidth: number | null,
): number {
  const gapCount = Math.max(0, layerWidths.length - 1);
  if (gapCount === 0 || availableWidth === null) {
    return PREFERRED_MAIN_GAP;
  }
  const fittingStep = (
    availableWidth
    - HORIZONTAL_PADDING * 2
    - layerWidths.reduce((total, value) => total + value, 0)
  ) / gapCount;
  return Math.max(
    MIN_MAIN_GAP,
    Math.min(PREFERRED_MAIN_GAP, fittingStep),
  );
}

function graphNodeSize(node: ExecutionGraphNodeView): number {
  return node.kind === "subagent" || node.kind === "tool"
    ? NESTED_NODE_SIZE
    : AGENT_NODE_SIZE;
}

function buildNestedEdgePath(
  source: ExecutionGraphNodeLayout,
  target: ExecutionGraphNodeLayout,
): string {
  const sourceY = source.y + source.size / 2;
  const targetY = target.y - target.size / 2;
  if (targetY <= sourceY) {
    return buildEdgePath(source, target);
  }
  const curve = Math.max(14, (targetY - sourceY) * 0.48);
  return [
    `M ${source.x} ${sourceY}`,
    `C ${source.x} ${sourceY + curve}`,
    `${target.x} ${targetY - curve}`,
    `${target.x} ${targetY}`,
  ].join(" ");
}

function buildControlEdgePath(
  source: ExecutionGraphNodeLayout,
  target: ExecutionGraphNodeLayout,
): string {
  const sourceX = source.x + source.size / 2;
  const targetX = target.x + target.size / 2;
  const controlX = Math.max(sourceX, targetX) + 28;
  return [
    `M ${sourceX} ${source.y}`,
    `C ${controlX} ${source.y}`,
    `${controlX} ${target.y}`,
    `${targetX} ${target.y}`,
  ].join(" ");
}

function isExecutionControlEdge(kind: ExecutionGraphEdgeKind): boolean {
  return kind === "loop_back" || kind === "retry";
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
