/**
 * INPUT: 权威 Execution Graph 节点/边、当前画布可用宽度与纯 UI 隐藏节点集合。
 * OUTPUT: 主责任图自上而下展开；每个不可变 Attempt/Review 轮次保持独立层级；每个 Agent/Subagent 的直接子节点从左向右形成独立子树车道，后代始终落在真实拥有者下方；正向流程走中轴，真正成环的控制回连在所属子图框内避让节点、合流到共享 U 形正交总线并从目标的正常流程中心锚点收口。
 * POS: 后端 Agent/Subagent/Tool/Gate Graph View 到交互画布之间的无状态树形投影；只为一级 Agent/Gate 的完整运行树绘制外框，Subagent 层级只由树线表达，不再嵌套子图框。
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
const MAIN_VERTICAL_GAP = 72;
const PREFERRED_MAIN_LAYER_HORIZONTAL_GAP = 36;
const MIN_MAIN_LAYER_HORIZONTAL_GAP = 24;
const NESTED_NODE_HORIZONTAL_GAP = 16;
const NESTED_SUBGRAPH_HORIZONTAL_GAP = 40;
const NESTED_LAYER_VERTICAL_GAP = 46;
const CONTROL_EDGE_GUTTER = 18;
const CONTROL_EDGE_KIND_LANE_GAP = 8;
const CONTROL_EDGE_ROUTE_LANE_COUNT = 8;
const CONTROL_EDGE_NODE_CLEARANCE = 4;
const CONTROL_EDGE_FRAME_SAFE_GAP = 16;
const GROUP_PADDING = 40;
const HORIZONTAL_PADDING = 24;
const VERTICAL_PADDING = 24;
const MIN_CANVAS_WIDTH = 340;
const MIN_CANVAS_HEIGHT = 136;

interface ExecutionGraphNodeLayout {
  item: ExecutionWorkItemView | null;
  node: ExecutionGraphNodeView;
  size: number;
  x: number;
  y: number;
}

interface ExecutionGraphEdgeLayout {
  edge: ExecutionGraphEdgeView;
  id: string;
  kind: ExecutionGraphEdgeKind;
  paired: boolean;
  path: string;
  sourceId: string;
  targetId: string;
  x: number;
  y: number;
}

interface ExecutionGraphPathSegment {
  axis: "horizontal" | "vertical";
  fixed: number;
  start: number;
  end: number;
}

interface ExecutionControlRouteContext {
  group: ExecutionGraphGroupLayout | null;
  nodes: ExecutionGraphNodeLayout[];
}

interface ExecutionControlRouteCandidate {
  lane: number;
  path: string;
  portOffset: number;
  segments: ExecutionGraphPathSegment[];
  side: -1 | 1;
}

interface ExecutionGraphGroupLayout {
  height: number;
  id: string;
  nodeIds: string[];
  width: number;
  x: number;
  y: number;
}

interface ExecutionGraphLayout {
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
  rootX: number;
  rootY: number;
  width: number;
}

interface ExecutionGraphOwnership {
  childrenByOwnerId: Map<string, ExecutionGraphNodeView[]>;
  parentByNodeId: Map<string, string>;
}

interface ExecutionOwnershipTree {
  children: ExecutionOwnershipTree[];
  node: ExecutionGraphNodeView;
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
  const ownership = resolveExecutionGraphOwnership(graphNodes, graphEdges);
  const rootByNodeId = resolveClusterRoots(
    graphNodes,
    ownership.parentByNodeId,
  );
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
      ownership.parentByNodeId,
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

  const layerHorizontalGaps = Array.from(
    { length: maxDepth + 1 },
    (_, depth) => resolveMainLayerHorizontalGap(
      layers.get(depth) ?? [],
      constrainedWidth,
    ),
  );
  const layerWidths = Array.from({ length: maxDepth + 1 }, (_, depth) => {
    const layer = layers.get(depth) ?? [];
    return layer.reduce((total, cluster, index) => (
      total + cluster.width + (index === 0 ? 0 : layerHorizontalGaps[depth])
    ), 0);
  });
  const layerRootYs = Array.from({ length: maxDepth + 1 }, (_, depth) => (
    Math.max(0, ...(layers.get(depth) ?? []).map((cluster) => cluster.rootY))
  ));
  const layerHeights = Array.from({ length: maxDepth + 1 }, (_, depth) => {
    const layer = layers.get(depth) ?? [];
    const belowRoot = Math.max(
      0,
      ...layer.map((cluster) => cluster.height - cluster.rootY),
    );
    return layerRootYs[depth] + belowRoot;
  });
  const contentHeight = layerHeights.reduce((total, value) => total + value, 0)
    + maxDepth * MAIN_VERTICAL_GAP;
  const maxRootX = Math.max(...clusters.map((cluster) => cluster.rootX));
  const maxRightOfRoot = Math.max(
    ...clusters.map((cluster) => cluster.width - cluster.rootX),
  );
  const alignedWidth = maxRootX + maxRightOfRoot;
  const stackedWidth = Math.max(...layerWidths);
  const naturalWidth = HORIZONTAL_PADDING * 2
    + Math.max(alignedWidth, stackedWidth);
  const naturalHeight = VERTICAL_PADDING * 2 + contentHeight;
  const minimumWidth = constrainedWidth === null
    ? MIN_CANVAS_WIDTH
    : Math.min(MIN_CANVAS_WIDTH, constrainedWidth);
  const width = Math.max(minimumWidth, naturalWidth);
  const height = Math.max(MIN_CANVAS_HEIGHT, naturalHeight);
  const verticalOffset = (height - contentHeight) / 2;
  const alignedRootX = (width - alignedWidth) / 2 + maxRootX;
  const itemById = new Map(
    orderedExecutionItems(execution).map((item) => [item.id, item]),
  );
  const absolutePositionById = new Map<string, { x: number; y: number }>();
  let layerTop = verticalOffset;

  for (let depth = 0; depth <= maxDepth; depth += 1) {
    const layer = layers.get(depth) ?? [];
    const layerHeight = layerHeights[depth];
    let clusterLeft = layer.length === 1
      ? alignedRootX - layer[0].rootX
      : (width - layerWidths[depth]) / 2;
    for (const cluster of layer) {
      const clusterTop = layerTop + layerRootYs[depth] - cluster.rootY;
      for (const node of cluster.nodes) {
        const relative = cluster.positions.get(node.id);
        if (relative) {
          absolutePositionById.set(node.id, {
            x: clusterLeft + relative.x,
            y: clusterTop + relative.y,
          });
        }
      }
      clusterLeft += cluster.width + layerHorizontalGaps[depth];
    }
    layerTop += layerHeight + MAIN_VERTICAL_GAP;
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
  const groups = buildExecutionGraphGroups(
    graphNodes,
    layoutNodeById,
    ownership.childrenByOwnerId,
  );
  const edgePathById = new Map<string, string>();
  for (const edge of graphEdges) {
    if (isExecutionControlEdge(edge.kind)) {
      continue;
    }
    const source = layoutNodeById.get(edge.source_node_id);
    const target = layoutNodeById.get(edge.target_node_id);
    if (!source || !target) {
      continue;
    }
    const path = rootByNodeId.get(source.node.id) === rootByNodeId.get(target.node.id)
      ? buildNestedEdgePath(source, target)
      : buildMainEdgePath(source, target);
    edgePathById.set(edge.id, path);
  }
  for (const edge of graphEdges) {
    if (!isExecutionControlEdge(edge.kind)) {
      continue;
    }
    const source = layoutNodeById.get(edge.source_node_id);
    const target = layoutNodeById.get(edge.target_node_id);
    if (!source || !target) {
      continue;
    }
    const path = buildControlEdgePath(source, target, edge.kind, {
      group: groups.find((group) => (
        group.nodeIds.includes(source.node.id)
          && group.nodeIds.includes(target.node.id)
      )) ?? null,
      nodes,
    });
    edgePathById.set(edge.id, path);
  }

  const edges: ExecutionGraphEdgeLayout[] = [];
  for (const edge of graphEdges) {
    const source = layoutNodeById.get(edge.source_node_id);
    const target = layoutNodeById.get(edge.target_node_id);
    const path = edgePathById.get(edge.id);
    if (!source || !target || !path) {
      continue;
    }
    const isControlEdge = isExecutionControlEdge(edge.kind);
    const hasReverseProcessEdge = isControlEdge && graphEdges.some((candidate) => (
      !isExecutionControlEdge(candidate.kind)
      && candidate.source_node_id === edge.target_node_id
      && candidate.target_node_id === edge.source_node_id
    ));
    const hasReverseControlEdge = !isControlEdge && graphEdges.some((candidate) => (
      isExecutionControlEdge(candidate.kind)
      && candidate.source_node_id === edge.target_node_id
      && candidate.target_node_id === edge.source_node_id
    ));
    edges.push({
      edge,
      id: edge.id,
      kind: edge.kind,
      paired: hasReverseProcessEdge || hasReverseControlEdge,
      path,
      sourceId: edge.source_node_id,
      targetId: edge.target_node_id,
      x: (source.x + target.x) / 2,
      y: (source.y + target.y) / 2,
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
      .filter((edge) => isExecutionOwnershipEdge(edge.kind))
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

function resolveExecutionGraphOwnership(
  nodes: ExecutionGraphNodeView[],
  edges: ExecutionGraphEdgeView[],
): ExecutionGraphOwnership {
  const nodeById = new Map(nodes.map((node) => [node.id, node]));
  const parentById = new Map<string, string>();
  for (const node of nodes) {
    if (
      node.visibility !== "primary"
      && node.parent_node_id
      && node.parent_node_id !== node.id
      && nodeById.has(node.parent_node_id)
    ) {
      parentById.set(node.id, node.parent_node_id);
    }
  }
  for (const edge of edges) {
    const target = nodeById.get(edge.target_node_id);
    if (
      !target
      || target.visibility === "primary"
      || parentById.has(target.id)
      || !isExecutionOwnershipEdge(edge.kind)
      || !nodeById.has(edge.source_node_id)
    ) {
      continue;
    }
    parentById.set(target.id, edge.source_node_id);
  }
  const childrenByOwnerId = new Map<string, ExecutionGraphNodeView[]>();
  for (const node of nodes) {
    const parentId = parentById.get(node.id);
    if (!parentId) {
      continue;
    }
    const children = childrenByOwnerId.get(parentId) ?? [];
    children.push(node);
    childrenByOwnerId.set(parentId, children);
  }
  for (const children of childrenByOwnerId.values()) {
    children.sort(compareExecutionGraphNodeOrder);
  }
  return { childrenByOwnerId, parentByNodeId: parentById };
}

function resolveClusterRoots(
  nodes: ExecutionGraphNodeView[],
  parentById: Map<string, string>,
): Map<string, string> {
  const nodeById = new Map(nodes.map((node) => [node.id, node]));
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
  parentByNodeId: Map<string, string>,
): ExecutionGraphCluster {
  const memberById = new Map(members.map((node) => [node.id, node]));
  const clusterParentByNodeId = new Map<string, string>();
  const childrenByOwnerId = new Map<string, ExecutionGraphNodeView[]>();
  for (const node of members) {
    if (node.id === root.id) {
      continue;
    }
    const declaredParentId = parentByNodeId.get(node.id);
    const parentId = declaredParentId
      && declaredParentId !== node.id
      && memberById.has(declaredParentId)
      ? declaredParentId
      : root.id;
    clusterParentByNodeId.set(node.id, parentId);
    const children = childrenByOwnerId.get(parentId) ?? [];
    children.push(node);
    childrenByOwnerId.set(parentId, children);
  }
  for (const children of childrenByOwnerId.values()) {
    children.sort(compareExecutionGraphNodeOrder);
  }
  const depthById = resolveOwnershipDepths(
    members,
    root.id,
    clusterParentByNodeId,
  );
  let maxDepth = 0;
  for (const node of members) {
    const depth = node.id === root.id ? 0 : Math.max(1, depthById[node.id] ?? 1);
    maxDepth = Math.max(maxDepth, depth);
  }
  const padded = members.length > 1;
  const padding = padded ? GROUP_PADDING : 0;
  const layerHeights = Array.from({ length: maxDepth + 1 }, () => 0);
  for (const node of members) {
    const depth = node.id === root.id ? 0 : Math.max(1, depthById[node.id] ?? 1);
    layerHeights[depth] = Math.max(layerHeights[depth], graphNodeSize(node));
  }
  const layerCenters: number[] = [];
  let layerTop = padding;
  for (let depth = 0; depth <= maxDepth; depth += 1) {
    layerCenters[depth] = layerTop + layerHeights[depth] / 2;
    layerTop += layerHeights[depth] + NESTED_LAYER_VERTICAL_GAP;
  }
  const tree = buildExecutionOwnershipTree(
    root,
    childrenByOwnerId,
    new Set(),
  );
  const contentWidth = tree.width;
  const contentHeight = layerHeights.reduce((total, value) => total + value, 0)
    + maxDepth * NESTED_LAYER_VERTICAL_GAP;
  const width = contentWidth + padding * 2;
  const height = contentHeight + padding * 2;
  const positions = new Map<string, { x: number; y: number }>();
  placeExecutionOwnershipTree(
    tree,
    padding,
    0,
    layerCenters,
    positions,
  );
  const rootPosition = positions.get(root.id);
  return {
    height,
    nodes: members,
    positions,
    root,
    rootX: rootPosition?.x ?? width / 2,
    rootY: rootPosition?.y ?? height / 2,
    width,
  };
}

function buildExecutionOwnershipTree(
  node: ExecutionGraphNodeView,
  childrenByOwnerId: Map<string, ExecutionGraphNodeView[]>,
  ancestors: Set<string>,
): ExecutionOwnershipTree {
  const nextAncestors = new Set(ancestors).add(node.id);
  const children = (childrenByOwnerId.get(node.id) ?? [])
    .filter((child) => !nextAncestors.has(child.id))
    .map((child) => buildExecutionOwnershipTree(
      child,
      childrenByOwnerId,
      nextAncestors,
    ));
  const childrenWidth = executionOwnershipChildrenWidth(children);
  return {
    children,
    node,
    width: Math.max(graphNodeSize(node), childrenWidth),
  };
}

function placeExecutionOwnershipTree(
  tree: ExecutionOwnershipTree,
  left: number,
  depth: number,
  layerCenters: number[],
  positions: Map<string, { x: number; y: number }>,
): void {
  positions.set(tree.node.id, {
    x: left + tree.width / 2,
    y: layerCenters[depth] ?? layerCenters[layerCenters.length - 1],
  });
  const childrenWidth = executionOwnershipChildrenWidth(tree.children);
  let childLeft = left + (tree.width - childrenWidth) / 2;
  for (let index = 0; index < tree.children.length; index += 1) {
    const child = tree.children[index];
    placeExecutionOwnershipTree(
      child,
      childLeft,
      depth + 1,
      layerCenters,
      positions,
    );
    childLeft += child.width;
    if (index < tree.children.length - 1) {
      childLeft += executionOwnershipSubtreeGap(
        child,
        tree.children[index + 1],
      );
    }
  }
}

function executionOwnershipChildrenWidth(
  children: ExecutionOwnershipTree[],
): number {
  return children.reduce((total, child, index) => (
    total
      + child.width
      + (index === 0
        ? 0
        : executionOwnershipSubtreeGap(children[index - 1], child))
  ), 0);
}

function executionOwnershipSubtreeGap(
  left: ExecutionOwnershipTree,
  right: ExecutionOwnershipTree,
): number {
  return left.children.length > 0
    || right.children.length > 0
    || left.node.kind === "subagent"
    || right.node.kind === "subagent"
    ? NESTED_SUBGRAPH_HORIZONTAL_GAP
    : NESTED_NODE_HORIZONTAL_GAP;
}

function buildExecutionGraphGroups(
  nodes: ExecutionGraphNodeView[],
  layoutNodeById: Map<string, ExecutionGraphNodeLayout>,
  childrenByOwnerId: Map<string, ExecutionGraphNodeView[]>,
): ExecutionGraphGroupLayout[] {
  const result: ExecutionGraphGroupLayout[] = [];
  for (const owner of nodes) {
    if (!canFrameExecutionSubgraph(owner)) {
      continue;
    }
    const scope = collectExecutionSubgraphNodes(
      owner,
      childrenByOwnerId,
      new Set(),
    )
      .map((node) => layoutNodeById.get(node.id))
      .filter((child): child is ExecutionGraphNodeLayout => child !== undefined);
    if (scope.length <= 1) {
      continue;
    }
    const left = Math.min(...scope.map((child) => child.x - child.size / 2));
    const right = Math.max(...scope.map((child) => child.x + child.size / 2));
    const top = Math.min(...scope.map((child) => child.y - child.size / 2));
    const bottom = Math.max(...scope.map((child) => child.y + child.size / 2));
    result.push({
      height: bottom - top + GROUP_PADDING * 2,
      id: owner.id,
      nodeIds: scope.map((child) => child.node.id),
      width: right - left + GROUP_PADDING * 2,
      x: left - GROUP_PADDING,
      y: top - GROUP_PADDING,
    });
  }
  return result;
}

function collectExecutionSubgraphNodes(
  owner: ExecutionGraphNodeView,
  childrenByOwnerId: Map<string, ExecutionGraphNodeView[]>,
  ancestors: Set<string>,
): ExecutionGraphNodeView[] {
  if (ancestors.has(owner.id)) {
    return [];
  }
  const nextAncestors = new Set(ancestors).add(owner.id);
  return [
    owner,
    ...(childrenByOwnerId.get(owner.id) ?? []).flatMap((child) => (
      collectExecutionSubgraphNodes(child, childrenByOwnerId, nextAncestors)
    )),
  ];
}

function canFrameExecutionSubgraph(node: ExecutionGraphNodeView): boolean {
  return node.visibility === "primary"
    && (node.kind === "agent" || node.kind === "gate");
}

function resolveOwnershipDepths(
  nodes: ExecutionGraphNodeView[],
  rootId: string,
  parentByNodeId: Map<string, string>,
): Record<string, number> {
  const nodeIds = new Set(nodes.map((node) => node.id));
  const result: Record<string, number> = { [rootId]: 0 };
  const resolveDepth = (nodeId: string, visiting: Set<string>): number => {
    if (result[nodeId] !== undefined) {
      return result[nodeId];
    }
    if (visiting.has(nodeId)) {
      result[nodeId] = 1;
      return 1;
    }
    const parentId = parentByNodeId.get(nodeId);
    const depth = parentId && nodeIds.has(parentId)
      ? resolveDepth(parentId, new Set(visiting).add(nodeId)) + 1
      : 1;
    result[nodeId] = depth;
    return depth;
  };
  for (const node of nodes) {
    resolveDepth(node.id, new Set());
  }
  return result;
}

function compareExecutionGraphNodeOrder(
  left: ExecutionGraphNodeView,
  right: ExecutionGraphNodeView,
): number {
  return left.position - right.position || left.id.localeCompare(right.id);
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
  const downstreamByNodeId = new Map<string, string[]>();
  const validEdges = edges.filter((edge) => (
    nodeIds.has(edge.source_node_id) && nodeIds.has(edge.target_node_id)
  ));
  const addDepthEdge = (edge: ExecutionGraphEdgeView): void => {
    const upstream = upstreamByNodeId.get(edge.target_node_id) ?? [];
    upstream.push(edge.source_node_id);
    upstreamByNodeId.set(edge.target_node_id, upstream);
    const downstream = downstreamByNodeId.get(edge.source_node_id) ?? [];
    downstream.push(edge.target_node_id);
    downstreamByNodeId.set(edge.source_node_id, downstream);
  };
  // Structural edges are the stable skeleton. Build them first so deciding
  // whether a retry/loop-back advances to a new immutable Attempt or closes a
  // cycle never depends on the JSON edge order.
  for (const edge of validEdges) {
    if (!isExecutionControlEdge(edge.kind)) {
      addDepthEdge(edge);
    }
  }
  for (const edge of validEdges) {
    if (!isExecutionControlEdge(edge.kind)) {
      continue;
    }
    if (executionGraphPathExists(
      downstreamByNodeId,
      edge.target_node_id,
      edge.source_node_id,
    )) {
      continue;
    }
    addDepthEdge(edge);
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

function executionGraphPathExists(
  downstreamByNodeId: Map<string, string[]>,
  sourceNodeId: string,
  targetNodeId: string,
): boolean {
  const pending = [sourceNodeId];
  const visited = new Set<string>();
  while (pending.length > 0) {
    const current = pending.pop();
    if (!current || visited.has(current)) {
      continue;
    }
    if (current === targetNodeId) {
      return true;
    }
    visited.add(current);
    pending.push(...(downstreamByNodeId.get(current) ?? []));
  }
  return false;
}

function normalizeAvailableWidth(width: number | undefined): number | null {
  if (width === undefined || !Number.isFinite(width) || width <= 0) {
    return null;
  }
  return Math.floor(width);
}

function resolveMainLayerHorizontalGap(
  clusters: ExecutionGraphCluster[],
  availableWidth: number | null,
): number {
  const gapCount = Math.max(0, clusters.length - 1);
  if (gapCount === 0 || availableWidth === null) {
    return PREFERRED_MAIN_LAYER_HORIZONTAL_GAP;
  }
  const fittingStep = (
    availableWidth
    - HORIZONTAL_PADDING * 2
    - clusters.reduce((total, cluster) => total + cluster.width, 0)
  ) / gapCount;
  return Math.max(
    MIN_MAIN_LAYER_HORIZONTAL_GAP,
    Math.min(PREFERRED_MAIN_LAYER_HORIZONTAL_GAP, fittingStep),
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
    return buildMainEdgePath(source, target);
  }
  return buildOrthogonalEdgePath(source.x, sourceY, target.x, targetY);
}

function buildControlEdgePath(
  source: ExecutionGraphNodeLayout,
  target: ExecutionGraphNodeLayout,
  kind: ExecutionGraphEdgeKind,
  context: ExecutionControlRouteContext,
): string {
  if (Math.abs(source.y - target.y) < 1) {
    return selectExecutionControlRoute(
      Array.from(
        { length: CONTROL_EDGE_ROUTE_LANE_COUNT },
        (_, lane) => buildSiblingControlEdgeCandidate(
          source,
          target,
          lane + (kind === "retry" ? 1 : 0),
        ),
      ),
      source,
      target,
      context,
    ).path;
  }
  return selectExecutionControlRoute(
    buildSideControlEdgeCandidates(source, target, context),
    source,
    target,
    context,
  ).path;
}

// 子节点先沿正常流程方向离开当前节点层，再从层外的水平轨道接入左右
// 侧轨，最后回到目标节点的正常流程中心锚点。子图内的左右侧轨与底部总线固定内缩到
// 圆角框的安全槽中，避免线条贴住边框或圆角。同一子图、目标与侧面的回连
// 允许选择完全相同的 U 形总线，仅保留各源节点自己的接入竖线；路由只把
// 节点碰撞视为硬约束，不再为了躲避其他线条制造大量平行轨道。
function buildSideControlEdgeCandidates(
  source: ExecutionGraphNodeLayout,
  target: ExecutionGraphNodeLayout,
  context: ExecutionControlRouteContext,
): ExecutionControlRouteCandidate[] {
  const returnsUpward = source.y > target.y;
  const sourceY = source.y
    + (returnsUpward ? source.size / 2 : -source.size / 2);
  const targetY = target.y
    + (returnsUpward ? target.size / 2 : -target.size / 2);
  const sourceLayerBoundary = executionControlSourceLayerBoundary(
    source,
    context.nodes,
    returnsUpward,
  );
  const preferredSide: -1 | 1 = source.x < target.x ? -1 : 1;
  const sides: Array<-1 | 1> = [preferredSide, preferredSide === -1 ? 1 : -1];
  const result: ExecutionControlRouteCandidate[] = [];
  for (const side of sides) {
    const targetX = target.x + side * target.size / 2;
    const baseRailX = context.group
      ? side < 0
        ? context.group.x + CONTROL_EDGE_FRAME_SAFE_GAP
        : context.group.x + context.group.width - CONTROL_EDGE_FRAME_SAFE_GAP
      : side < 0
        ? Math.min(source.x - source.size / 2, targetX) - CONTROL_EDGE_GUTTER
        : Math.max(source.x + source.size / 2, targetX) + CONTROL_EDGE_GUTTER;
    for (let lane = 0; lane < CONTROL_EDGE_ROUTE_LANE_COUNT; lane += 1) {
      const railX = context.group
        ? baseRailX - side * lane * CONTROL_EDGE_KIND_LANE_GAP
        : baseRailX + side * lane * CONTROL_EDGE_KIND_LANE_GAP;
      const outerY = context.group
        ? returnsUpward
          ? context.group.y + context.group.height
            - CONTROL_EDGE_FRAME_SAFE_GAP
            - lane * CONTROL_EDGE_KIND_LANE_GAP
          : context.group.y
            + CONTROL_EDGE_FRAME_SAFE_GAP
            + lane * CONTROL_EDGE_KIND_LANE_GAP
        : sourceLayerBoundary
          + (returnsUpward ? 1 : -1)
            * (CONTROL_EDGE_GUTTER + lane * CONTROL_EDGE_KIND_LANE_GAP);
      const approachY = targetY
        + (returnsUpward ? 1 : -1)
          * (CONTROL_EDGE_GUTTER + lane * CONTROL_EDGE_KIND_LANE_GAP / 2);
      const path = [
        `M ${source.x} ${sourceY}`,
        `L ${source.x} ${outerY}`,
        `L ${railX} ${outerY}`,
        `L ${railX} ${approachY}`,
        `L ${target.x} ${approachY}`,
        `L ${target.x} ${targetY}`,
      ].join(" ");
      result.push({
        lane,
        path,
        portOffset: 0,
        segments: executionGraphPathSegments(path),
        side,
      });
    }
  }
  return result;
}

function executionControlSourceLayerBoundary(
  source: ExecutionGraphNodeLayout,
  nodes: ExecutionGraphNodeLayout[],
  returnsUpward: boolean,
): number {
  const sourceTop = source.y - source.size / 2;
  const sourceBottom = source.y + source.size / 2;
  const peers = nodes.filter((node) => (
    node.y - node.size / 2 < sourceBottom + 1
      && node.y + node.size / 2 > sourceTop - 1
  ));
  return returnsUpward
    ? Math.max(...peers.map((node) => node.y + node.size / 2))
    : Math.min(...peers.map((node) => node.y - node.size / 2));
}

function buildSiblingControlEdgeCandidate(
  source: ExecutionGraphNodeLayout,
  target: ExecutionGraphNodeLayout,
  lane: number,
): ExecutionControlRouteCandidate {
  const sourceY = source.y + source.size / 2;
  const targetY = target.y + target.size / 2;
  const railY = Math.max(sourceY, targetY)
    + CONTROL_EDGE_GUTTER
    + lane * CONTROL_EDGE_KIND_LANE_GAP;
  const path = [
    `M ${source.x} ${sourceY}`,
    `L ${source.x} ${railY}`,
    `L ${target.x} ${railY}`,
    `L ${target.x} ${targetY}`,
  ].join(" ");
  return {
    lane,
    path,
    portOffset: 0,
    segments: executionGraphPathSegments(path),
    side: source.x <= target.x ? -1 : 1,
  };
}

function selectExecutionControlRoute(
  candidates: ExecutionControlRouteCandidate[],
  source: ExecutionGraphNodeLayout,
  target: ExecutionGraphNodeLayout,
  context: ExecutionControlRouteContext,
): ExecutionControlRouteCandidate {
  const preferredSide: -1 | 1 = source.x < target.x ? -1 : 1;
  return candidates.reduce((best, candidate) => {
    const score = executionControlRouteScore(
      candidate,
      source,
      target,
      context,
      preferredSide,
    );
    const bestScore = executionControlRouteScore(
      best,
      source,
      target,
      context,
      preferredSide,
    );
    return score < bestScore ? candidate : best;
  });
}

function executionControlRouteScore(
  candidate: ExecutionControlRouteCandidate,
  source: ExecutionGraphNodeLayout,
  target: ExecutionGraphNodeLayout,
  context: ExecutionControlRouteContext,
  preferredSide: -1 | 1,
): number {
  let score = candidate.lane * 2
    + Math.abs(candidate.portOffset) * 0.35
    + (candidate.side === preferredSide ? 0 : 20)
    + executionGraphPathLength(candidate.segments) * 0.02;
  for (const segment of candidate.segments) {
    for (const node of context.nodes) {
      if (node.node.id === source.node.id || node.node.id === target.node.id) {
        continue;
      }
      const collision = executionGraphSegmentNodeOverlap(segment, node);
      if (collision > 0) {
        score += 100_000 + collision * 1_000;
      }
    }
  }
  return score;
}

function executionGraphSegmentNodeOverlap(
  segment: ExecutionGraphPathSegment,
  node: ExecutionGraphNodeLayout,
): number {
  const half = node.size / 2 + CONTROL_EDGE_NODE_CLEARANCE;
  const left = node.x - half;
  const right = node.x + half;
  const top = node.y - half;
  const bottom = node.y + half;
  if (segment.axis === "vertical") {
    return segment.fixed > left && segment.fixed < right
      ? executionGraphRangeOverlap(segment.start, segment.end, top, bottom)
      : 0;
  }
  return segment.fixed > top && segment.fixed < bottom
    ? executionGraphRangeOverlap(segment.start, segment.end, left, right)
    : 0;
}

function executionGraphPathSegments(path: string): ExecutionGraphPathSegment[] {
  const points = Array.from(
    path.matchAll(/[ML]\s+(-?\d+(?:\.\d+)?)\s+(-?\d+(?:\.\d+)?)/g),
    (match) => ({ x: Number(match[1]), y: Number(match[2]) }),
  );
  const result: ExecutionGraphPathSegment[] = [];
  for (let index = 1; index < points.length; index += 1) {
    const start = points[index - 1];
    const end = points[index];
    if (Math.abs(start.x - end.x) < 0.5) {
      if (Math.abs(start.y - end.y) >= 0.5) {
        result.push({
          axis: "vertical",
          fixed: start.x,
          start: Math.min(start.y, end.y),
          end: Math.max(start.y, end.y),
        });
      }
      continue;
    }
    if (Math.abs(start.y - end.y) < 0.5) {
      result.push({
        axis: "horizontal",
        fixed: start.y,
        start: Math.min(start.x, end.x),
        end: Math.max(start.x, end.x),
      });
    }
  }
  return result;
}

function executionGraphPathLength(
  segments: ExecutionGraphPathSegment[],
): number {
  return segments.reduce((total, segment) => total + segment.end - segment.start, 0);
}

function executionGraphRangeOverlap(
  firstStart: number,
  firstEnd: number,
  secondStart: number,
  secondEnd: number,
): number {
  return Math.max(0, Math.min(firstEnd, secondEnd) - Math.max(firstStart, secondStart));
}

function isExecutionControlEdge(kind: ExecutionGraphEdgeKind): boolean {
  return kind === "loop_back" || kind === "retry";
}

function isExecutionOwnershipEdge(kind: ExecutionGraphEdgeKind): boolean {
  return kind === "invoke" || kind === "spawn" || kind === "guard";
}

function buildMainEdgePath(
  source: ExecutionGraphNodeLayout,
  target: ExecutionGraphNodeLayout,
): string {
  const sourceY = source.y + source.size / 2;
  const targetY = target.y - target.size / 2;
  if (targetY <= sourceY) {
    const reverseSourceY = source.y - source.size / 2;
    const reverseTargetY = target.y + target.size / 2;
    const sideDirection = source.x <= target.x ? -1 : 1;
    const railX = sideDirection < 0
      ? Math.min(source.x, target.x) - 38
      : Math.max(source.x, target.x) + 38;
    return [
      `M ${source.x} ${reverseSourceY}`,
      `L ${railX} ${reverseSourceY}`,
      `L ${railX} ${reverseTargetY}`,
      `L ${target.x} ${reverseTargetY}`,
    ].join(" ");
  }
  return buildOrthogonalEdgePath(source.x, sourceY, target.x, targetY);
}

function buildOrthogonalEdgePath(
  sourceX: number,
  sourceY: number,
  targetX: number,
  targetY: number,
): string {
  if (Math.abs(sourceX - targetX) < 1) {
    return `M ${sourceX} ${sourceY} L ${targetX} ${targetY}`;
  }
  const railY = (sourceY + targetY) / 2;
  return [
    `M ${sourceX} ${sourceY}`,
    `L ${sourceX} ${railY}`,
    `L ${targetX} ${railY}`,
    `L ${targetX} ${targetY}`,
  ].join(" ");
}
