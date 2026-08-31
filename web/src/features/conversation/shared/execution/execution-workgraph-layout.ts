/**
 * INPUT: 权威 Execution Graph 节点/边、当前画布可用宽度、节点展示密度与纯 UI 隐藏节点集合。
 * OUTPUT: 主责任图自上而下展开；节点图标与摘要卡片共享矩形避让和固定端口；稳定交叉最小化只在确实减少交叉时调整主层顺序；子图内部普通边按层间走廊分配轨道，跨子图边改从边框代理端口连接、把所有子图安全外扩区作为硬障碍并保留按需展示的精确节点短引线；真正成环的控制回连继续在所属子图框内避让节点并合流到共享 U 形正交总线。
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
const SUMMARY_NODE_WIDTH = 200;
const SUMMARY_NODE_HEIGHT = 64;
const MAIN_VERTICAL_GAP = 72;
const PREFERRED_MAIN_LAYER_HORIZONTAL_GAP = 36;
const MIN_MAIN_LAYER_HORIZONTAL_GAP = 24;
const NESTED_NODE_HORIZONTAL_GAP = 16;
const NESTED_SUBGRAPH_HORIZONTAL_GAP = 40;
const NESTED_LAYER_VERTICAL_GAP = 46;
const PROCESS_EDGE_LANE_GAP = 7;
const PROCESS_EDGE_CORRIDOR_PADDING = 10;
const BOUNDARY_PORT_SAFE_INSET = 22;
const BOUNDARY_PORT_MIN_GAP = 14;
const BOUNDARY_PORT_TAIL_GUTTER = 10;
const PROCESS_EDGE_GROUP_CLEARANCE = 10;
const PROCESS_EDGE_BEND_COST = 18;
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
  height: number;
  item: ExecutionWorkItemView | null;
  node: ExecutionGraphNodeView;
  size: number;
  width: number;
  x: number;
  y: number;
}

export type ExecutionGraphNodePresentation = "icon" | "summary";

interface ExecutionGraphEdgeLayout {
  edge: ExecutionGraphEdgeView;
  id: string;
  kind: ExecutionGraphEdgeKind;
  paired: boolean;
  path: string;
  sourceTailPath: string | null;
  sourceId: string;
  targetTailPath: string | null;
  targetId: string;
  x: number;
  y: number;
}

interface ExecutionGraphJunctionLayout {
  edgeIds: string[];
  id: string;
  kind: "fan-in" | "fan-out";
  x: number;
  y: number;
}

interface ExecutionGraphBoundaryPortLayout {
  edgeIds: string[];
  groupId: string;
  id: string;
  role: "source" | "target";
  side: "bottom" | "left" | "right" | "top";
  x: number;
  y: number;
}

interface ExecutionGraphBoundaryPortProjection {
  ports: ExecutionGraphBoundaryPortLayout[];
  sourcePortByEdgeId: Map<string, ExecutionGraphBoundaryPortLayout>;
  targetPortByEdgeId: Map<string, ExecutionGraphBoundaryPortLayout>;
}

interface ExecutionGraphBoundaryPortRequest {
  desiredAxis: number;
  edgeId: string;
  group: ExecutionGraphGroupLayout;
  key: string;
  role: "source" | "target";
  side: "bottom" | "left" | "right" | "top";
}

interface ExecutionGraphBoundaryPortBundle {
  desiredAxis: number;
  edgeIds: string[];
  group: ExecutionGraphGroupLayout;
  key: string;
  role: "source" | "target";
  side: "bottom" | "left" | "right" | "top";
}

interface ExecutionGraphPathSegment {
  axis: "horizontal" | "vertical";
  fixed: number;
  start: number;
  end: number;
}

interface ExecutionGraphRoutePoint {
  x: number;
  y: number;
}

interface ExecutionGraphRouteObstacle {
  bottom: number;
  id: string;
  left: number;
  right: number;
  top: number;
}

interface ExecutionGraphRouteState {
  cost: number;
  direction: "horizontal" | "start" | "vertical";
  key: string;
  pointIndex: number;
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
  junctions: ExecutionGraphJunctionLayout[];
  nodes: ExecutionGraphNodeLayout[];
  ports: ExecutionGraphBoundaryPortLayout[];
  width: number;
}

interface ExecutionProcessRouteRequest {
  corridorKey: string;
  edge: ExecutionGraphEdgeView;
  routeClass: string;
  source: ExecutionGraphNodeLayout;
  sourceX: number;
  sourceY: number;
  target: ExecutionGraphNodeLayout;
  targetX: number;
  targetY: number;
}

interface ExecutionProcessRouteBundle {
  id: string;
  junctionKind: "fan-in" | "fan-out" | null;
  requests: ExecutionProcessRouteRequest[];
  sortX: number;
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
  nodePresentation: ExecutionGraphNodePresentation = "icon",
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
      junctions: [],
      nodes: [],
      ports: [],
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
      nodePresentation,
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
  minimizeExecutionGraphLayerCrossings(layers, clusterEdges);

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
    const dimensions = graphNodeDimensions(node, nodePresentation);
    return {
      height: dimensions.height,
      item: itemById.get(node.work_item_id) ?? null,
      node,
      size: dimensions.size,
      width: dimensions.width,
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
  const boundaryPorts = buildExecutionGraphBoundaryPorts(
    graphEdges,
    layoutNodeById,
    rootByNodeId,
    groups,
  );
  const processRoutes = buildProcessEdgeRoutes(
    graphEdges,
    layoutNodeById,
    rootByNodeId,
    boundaryPorts,
    groups,
  );
  const edgePathById = processRoutes.pathById;
  for (const edge of graphEdges) {
    if (!isExecutionControlEdge(edge.kind)) {
      continue;
    }
    const source = layoutNodeById.get(edge.source_node_id);
    const target = layoutNodeById.get(edge.target_node_id);
    if (!source || !target) {
      continue;
    }
    const sharedGroup = groups.find((group) => (
      group.nodeIds.includes(source.node.id)
        && group.nodeIds.includes(target.node.id)
    )) ?? null;
    const sourceRoot = rootByNodeId.get(source.node.id) ?? source.node.id;
    const targetRoot = rootByNodeId.get(target.node.id) ?? target.node.id;
    let path: string;
    if (sourceRoot !== targetRoot) {
      const sourcePort = boundaryPorts.sourcePortByEdgeId.get(edge.id) ?? null;
      const targetPort = boundaryPorts.targetPortByEdgeId.get(edge.id) ?? null;
      const sides = executionGraphBoundaryPortSides(source, target);
      const sourcePoint = sourcePort
        ?? executionGraphNodeSideAnchor(source, sides.source);
      const targetPoint = targetPort
        ?? executionGraphNodeSideAnchor(target, sides.target);
      if (sourcePort) {
        processRoutes.sourceTailPathById.set(
          edge.id,
          buildExecutionGraphBoundaryPortTail(source, sourcePort),
        );
      }
      if (targetPort) {
        processRoutes.targetTailPathById.set(
          edge.id,
          buildExecutionGraphBoundaryPortTail(target, targetPort),
        );
      }
      path = buildObstacleAvoidingGraphEdgePath(
        sourcePoint,
        targetPoint,
        sourcePort,
        targetPort,
        groups,
      );
    } else {
      path = buildControlEdgePath(source, target, edge.kind, {
        group: sharedGroup,
        nodes,
      });
    }
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
    const midpoint = executionGraphPathMidpoint(path);
    edges.push({
      edge,
      id: edge.id,
      kind: edge.kind,
      paired: hasReverseProcessEdge || hasReverseControlEdge,
      path,
      sourceTailPath: processRoutes.sourceTailPathById.get(edge.id) ?? null,
      sourceId: edge.source_node_id,
      targetTailPath: processRoutes.targetTailPathById.get(edge.id) ?? null,
      targetId: edge.target_node_id,
      x: midpoint.x,
      y: midpoint.y,
    });
  }

  return {
    edges,
    groups,
    height,
    junctions: processRoutes.junctions,
    nodes,
    ports: boundaryPorts.ports,
    width,
  };
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
  nodePresentation: ExecutionGraphNodePresentation,
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
    layerHeights[depth] = Math.max(
      layerHeights[depth],
      graphNodeDimensions(node, nodePresentation).height,
    );
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
    nodePresentation,
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
  nodePresentation: ExecutionGraphNodePresentation,
): ExecutionOwnershipTree {
  const nextAncestors = new Set(ancestors).add(node.id);
  const children = (childrenByOwnerId.get(node.id) ?? [])
    .filter((child) => !nextAncestors.has(child.id))
    .map((child) => buildExecutionOwnershipTree(
      child,
      childrenByOwnerId,
      nextAncestors,
      nodePresentation,
    ));
  const childrenWidth = executionOwnershipChildrenWidth(children);
  return {
    children,
    node,
    width: Math.max(
      graphNodeDimensions(node, nodePresentation).width,
      childrenWidth,
    ),
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
    const left = Math.min(...scope.map((child) => child.x - child.width / 2));
    const right = Math.max(...scope.map((child) => child.x + child.width / 2));
    const top = Math.min(...scope.map((child) => child.y - child.height / 2));
    const bottom = Math.max(...scope.map((child) => child.y + child.height / 2));
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

function buildExecutionGraphBoundaryPorts(
  edges: ExecutionGraphEdgeView[],
  nodeById: Map<string, ExecutionGraphNodeLayout>,
  rootByNodeId: Map<string, string>,
  groups: ExecutionGraphGroupLayout[],
): ExecutionGraphBoundaryPortProjection {
  const groupById = new Map(groups.map((group) => [group.id, group]));
  const requests: ExecutionGraphBoundaryPortRequest[] = [];
  for (const edge of edges) {
    const source = nodeById.get(edge.source_node_id);
    const target = nodeById.get(edge.target_node_id);
    if (!source || !target) {
      continue;
    }
    const sourceRoot = rootByNodeId.get(source.node.id) ?? source.node.id;
    const targetRoot = rootByNodeId.get(target.node.id) ?? target.node.id;
    if (sourceRoot === targetRoot) {
      continue;
    }
    const sides = executionGraphBoundaryPortSides(source, target);
    const sourceGroup = groupById.get(sourceRoot);
    if (sourceGroup) {
      requests.push(executionGraphBoundaryPortRequest(
        edge,
        source,
        sourceGroup,
        "source",
        sides.source,
      ));
    }
    const targetGroup = groupById.get(targetRoot);
    if (targetGroup) {
      requests.push(executionGraphBoundaryPortRequest(
        edge,
        target,
        targetGroup,
        "target",
        sides.target,
      ));
    }
  }
  const requestsBySide = new Map<string, ExecutionGraphBoundaryPortRequest[]>();
  for (const request of requests) {
    const sideKey = `${request.group.id}:${request.side}`;
    const sideRequests = requestsBySide.get(sideKey) ?? [];
    sideRequests.push(request);
    requestsBySide.set(sideKey, sideRequests);
  }
  const ports: ExecutionGraphBoundaryPortLayout[] = [];
  const sourcePortByEdgeId = new Map<string, ExecutionGraphBoundaryPortLayout>();
  const targetPortByEdgeId = new Map<string, ExecutionGraphBoundaryPortLayout>();
  for (const sideRequests of requestsBySide.values()) {
    const bundleByKey = new Map<string, ExecutionGraphBoundaryPortBundle>();
    for (const request of sideRequests) {
      const bundle = bundleByKey.get(request.key) ?? {
        desiredAxis: request.desiredAxis,
        edgeIds: [],
        group: request.group,
        key: request.key,
        role: request.role,
        side: request.side,
      };
      bundle.edgeIds.push(request.edgeId);
      bundleByKey.set(request.key, bundle);
    }
    const bundles = [...bundleByKey.values()].sort((left, right) => (
      left.desiredAxis - right.desiredAxis || left.key.localeCompare(right.key)
    ));
    const axes = executionGraphBoundaryPortAxes(bundles);
    for (let index = 0; index < bundles.length; index += 1) {
      const bundle = bundles[index];
      const port = executionGraphBoundaryPortLayout(bundle, axes[index]);
      ports.push(port);
      for (const edgeId of port.edgeIds) {
        if (port.role === "source") {
          sourcePortByEdgeId.set(edgeId, port);
        } else {
          targetPortByEdgeId.set(edgeId, port);
        }
      }
    }
  }
  return { ports, sourcePortByEdgeId, targetPortByEdgeId };
}

function executionGraphBoundaryPortSides(
  source: ExecutionGraphNodeLayout,
  target: ExecutionGraphNodeLayout,
): {
  source: "bottom" | "left" | "right" | "top";
  target: "bottom" | "left" | "right" | "top";
} {
  if (target.y > source.y + 1) {
    return { source: "bottom", target: "top" };
  }
  if (target.y < source.y - 1) {
    return { source: "top", target: "bottom" };
  }
  return source.x <= target.x
    ? { source: "right", target: "left" }
    : { source: "left", target: "right" };
}

function executionGraphBoundaryPortRequest(
  edge: ExecutionGraphEdgeView,
  node: ExecutionGraphNodeLayout,
  group: ExecutionGraphGroupLayout,
  role: "source" | "target",
  side: "bottom" | "left" | "right" | "top",
): ExecutionGraphBoundaryPortRequest {
  return {
    desiredAxis: side === "bottom" || side === "top" ? node.x : node.y,
    edgeId: edge.id,
    group,
    key: [
      group.id,
      side,
      role,
      executionProcessRouteClass(edge.kind),
      node.node.id,
    ].join(":"),
    role,
    side,
  };
}

function executionGraphBoundaryPortAxes(
  bundles: ExecutionGraphBoundaryPortBundle[],
): number[] {
  if (bundles.length === 0) {
    return [];
  }
  const { group, side } = bundles[0];
  const minimum = (side === "bottom" || side === "top" ? group.x : group.y)
    + BOUNDARY_PORT_SAFE_INSET;
  const maximum = minimum
    + (side === "bottom" || side === "top" ? group.width : group.height)
    - BOUNDARY_PORT_SAFE_INSET * 2;
  if (bundles.length === 1 || maximum <= minimum) {
    return [Math.max(minimum, Math.min(maximum, bundles[0].desiredAxis))];
  }
  const gap = Math.min(
    BOUNDARY_PORT_MIN_GAP,
    (maximum - minimum) / (bundles.length - 1),
  );
  const axes = bundles.map((bundle) => (
    Math.max(minimum, Math.min(maximum, bundle.desiredAxis))
  ));
  for (let index = 1; index < axes.length; index += 1) {
    axes[index] = Math.max(axes[index], axes[index - 1] + gap);
  }
  const overflow = Math.max(0, axes[axes.length - 1] - maximum);
  if (overflow > 0) {
    for (let index = 0; index < axes.length; index += 1) {
      axes[index] -= overflow;
    }
  }
  for (let index = axes.length - 2; index >= 0; index -= 1) {
    axes[index] = Math.min(axes[index], axes[index + 1] - gap);
  }
  const underflow = Math.max(0, minimum - axes[0]);
  if (underflow > 0) {
    for (let index = 0; index < axes.length; index += 1) {
      axes[index] += underflow;
    }
  }
  return axes;
}

function executionGraphBoundaryPortLayout(
  bundle: ExecutionGraphBoundaryPortBundle,
  axis: number,
): ExecutionGraphBoundaryPortLayout {
  const { group, side } = bundle;
  return {
    edgeIds: bundle.edgeIds,
    groupId: group.id,
    id: `port:${bundle.key}`,
    role: bundle.role,
    side,
    x: side === "left"
      ? group.x
      : side === "right"
      ? group.x + group.width
      : axis,
    y: side === "top"
      ? group.y
      : side === "bottom"
      ? group.y + group.height
      : axis,
  };
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

function minimizeExecutionGraphLayerCrossings(
  layers: Map<number, ExecutionGraphCluster[]>,
  edges: ExecutionGraphEdgeView[],
): void {
  const structuralEdges = edges.filter((edge) => !isExecutionControlEdge(edge.kind));
  const depths = [...layers.keys()].sort((left, right) => left - right);
  if (depths.length < 2 || structuralEdges.length < 2) {
    return;
  }
  for (let pass = 0; pass < 2; pass += 1) {
    for (let index = 1; index < depths.length; index += 1) {
      minimizeExecutionGraphLayerAgainstReference(
        layers.get(depths[index]) ?? [],
        layers.get(depths[index - 1]) ?? [],
        structuralEdges,
      );
    }
    for (let index = depths.length - 2; index >= 0; index -= 1) {
      minimizeExecutionGraphLayerAgainstReference(
        layers.get(depths[index]) ?? [],
        layers.get(depths[index + 1]) ?? [],
        structuralEdges,
      );
    }
  }
}

function minimizeExecutionGraphLayerAgainstReference(
  layer: ExecutionGraphCluster[],
  reference: ExecutionGraphCluster[],
  edges: ExecutionGraphEdgeView[],
): void {
  if (layer.length < 2 || reference.length < 2) {
    return;
  }
  const layerIds = new Set(layer.map((cluster) => cluster.root.id));
  const referenceIndexById = new Map(
    reference.map((cluster, index) => [cluster.root.id, index]),
  );
  const currentIndexById = new Map(
    layer.map((cluster, index) => [cluster.root.id, index]),
  );
  const relevantEdges = edges.filter((edge) => (
    layerIds.has(edge.source_node_id) && referenceIndexById.has(edge.target_node_id)
  ) || (
    layerIds.has(edge.target_node_id) && referenceIndexById.has(edge.source_node_id)
  ));
  if (relevantEdges.length < 2) {
    return;
  }
  const candidate = [...layer].sort((left, right) => {
    const leftScore = executionGraphLayerBarycenter(
      left.root.id,
      relevantEdges,
      referenceIndexById,
      currentIndexById.get(left.root.id) ?? 0,
    );
    const rightScore = executionGraphLayerBarycenter(
      right.root.id,
      relevantEdges,
      referenceIndexById,
      currentIndexById.get(right.root.id) ?? 0,
    );
    return leftScore - rightScore
      || (currentIndexById.get(left.root.id) ?? 0)
        - (currentIndexById.get(right.root.id) ?? 0);
  });
  if (
    executionGraphLayerCrossingCount(candidate, reference, relevantEdges)
    >= executionGraphLayerCrossingCount(layer, reference, relevantEdges)
  ) {
    return;
  }
  layer.splice(0, layer.length, ...candidate);
}

function executionGraphLayerBarycenter(
  nodeId: string,
  edges: ExecutionGraphEdgeView[],
  referenceIndexById: Map<string, number>,
  fallback: number,
): number {
  const positions: number[] = [];
  for (const edge of edges) {
    const neighborId = edge.source_node_id === nodeId
      ? edge.target_node_id
      : edge.target_node_id === nodeId
      ? edge.source_node_id
      : null;
    const position = neighborId ? referenceIndexById.get(neighborId) : undefined;
    if (position !== undefined) {
      positions.push(position);
    }
  }
  return positions.length > 0
    ? positions.reduce((total, value) => total + value, 0) / positions.length
    : fallback;
}

function executionGraphLayerCrossingCount(
  layer: ExecutionGraphCluster[],
  reference: ExecutionGraphCluster[],
  edges: ExecutionGraphEdgeView[],
): number {
  const layerIndexById = new Map(
    layer.map((cluster, index) => [cluster.root.id, index]),
  );
  const referenceIndexById = new Map(
    reference.map((cluster, index) => [cluster.root.id, index]),
  );
  const positions = edges.flatMap((edge) => {
    const sourceLayer = layerIndexById.get(edge.source_node_id);
    const sourceReference = referenceIndexById.get(edge.source_node_id);
    const targetLayer = layerIndexById.get(edge.target_node_id);
    const targetReference = referenceIndexById.get(edge.target_node_id);
    if (sourceLayer !== undefined && targetReference !== undefined) {
      return [{ layer: sourceLayer, reference: targetReference }];
    }
    if (targetLayer !== undefined && sourceReference !== undefined) {
      return [{ layer: targetLayer, reference: sourceReference }];
    }
    return [];
  });
  let result = 0;
  for (let left = 0; left < positions.length; left += 1) {
    for (let right = left + 1; right < positions.length; right += 1) {
      const layerDelta = positions[left].layer - positions[right].layer;
      const referenceDelta = positions[left].reference - positions[right].reference;
      if (layerDelta * referenceDelta < 0) {
        result += 1;
      }
    }
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

function graphNodeDimensions(
  node: ExecutionGraphNodeView,
  presentation: ExecutionGraphNodePresentation,
): { height: number; size: number; width: number } {
  const size = node.kind === "subagent" || node.kind === "tool"
    ? NESTED_NODE_SIZE
    : AGENT_NODE_SIZE;
  return presentation === "summary"
    ? { height: SUMMARY_NODE_HEIGHT, size, width: SUMMARY_NODE_WIDTH }
    : { height: size, size, width: size };
}

function buildProcessEdgeRoutes(
  edges: ExecutionGraphEdgeView[],
  nodeById: Map<string, ExecutionGraphNodeLayout>,
  rootByNodeId: Map<string, string>,
  boundaryPorts: ExecutionGraphBoundaryPortProjection,
  groups: ExecutionGraphGroupLayout[],
): {
  junctions: ExecutionGraphJunctionLayout[];
  pathById: Map<string, string>;
  sourceTailPathById: Map<string, string>;
  targetTailPathById: Map<string, string>;
} {
  const pathById = new Map<string, string>();
  const sourceTailPathById = new Map<string, string>();
  const targetTailPathById = new Map<string, string>();
  const requests: ExecutionProcessRouteRequest[] = [];
  for (const edge of edges) {
    if (isExecutionControlEdge(edge.kind)) {
      continue;
    }
    const source = nodeById.get(edge.source_node_id);
    const target = nodeById.get(edge.target_node_id);
    if (!source || !target) {
      continue;
    }
    const sourcePort = boundaryPorts.sourcePortByEdgeId.get(edge.id) ?? null;
    const targetPort = boundaryPorts.targetPortByEdgeId.get(edge.id) ?? null;
    if (sourcePort) {
      sourceTailPathById.set(
        edge.id,
        buildExecutionGraphBoundaryPortTail(source, sourcePort),
      );
    }
    if (targetPort) {
      targetTailPathById.set(
        edge.id,
        buildExecutionGraphBoundaryPortTail(target, targetPort),
      );
    }
    const sourceX = sourcePort?.x ?? source.x;
    const sourceY = sourcePort?.y ?? source.y + source.height / 2;
    const targetX = targetPort?.x ?? target.x;
    const targetY = targetPort?.y ?? target.y - target.height / 2;
    const sourceRoot = rootByNodeId.get(source.node.id) ?? source.node.id;
    const targetRoot = rootByNodeId.get(target.node.id) ?? target.node.id;
    if (sourceRoot !== targetRoot) {
      pathById.set(
        edge.id,
        buildObstacleAvoidingGraphEdgePath(
          { x: sourceX, y: sourceY },
          { x: targetX, y: targetY },
          sourcePort,
          targetPort,
          groups,
        ),
      );
      continue;
    }
    if (targetY <= sourceY) {
      pathById.set(edge.id, buildMainEdgePath(source, target));
      continue;
    }
    requests.push({
      corridorKey: `${sourceRoot}:${sourceY.toFixed(2)}:${targetY.toFixed(2)}`,
      edge,
      routeClass: executionProcessRouteClass(edge.kind),
      source,
      sourceX,
      sourceY,
      target,
      targetX,
      targetY,
    });
  }
  const corridors = new Map<string, ExecutionProcessRouteRequest[]>();
  for (const request of requests) {
    const corridor = corridors.get(request.corridorKey) ?? [];
    corridor.push(request);
    corridors.set(request.corridorKey, corridor);
  }
  const junctions: ExecutionGraphJunctionLayout[] = [];
  for (const [corridorKey, corridor] of corridors) {
    const outgoingCount = executionProcessRouteEndpointCounts(
      corridor,
      "source",
    );
    const incomingCount = executionProcessRouteEndpointCounts(
      corridor,
      "target",
    );
    const bundleById = new Map<string, ExecutionProcessRouteBundle>();
    for (const request of corridor) {
      const sourceKey = `${request.routeClass}:${request.edge.source_node_id}`;
      const targetKey = `${request.routeClass}:${request.edge.target_node_id}`;
      const junctionKind = (outgoingCount.get(sourceKey) ?? 0) > 1
        ? "fan-out"
        : (incomingCount.get(targetKey) ?? 0) > 1
        ? "fan-in"
        : null;
      const bundleId = junctionKind === "fan-out"
        ? `out:${sourceKey}`
        : junctionKind === "fan-in"
        ? `in:${targetKey}`
        : `edge:${request.edge.id}`;
      const bundle = bundleById.get(bundleId) ?? {
        id: bundleId,
        junctionKind,
        requests: [],
        sortX: junctionKind === "fan-out"
          ? request.sourceX
          : junctionKind === "fan-in"
          ? request.targetX
          : (request.sourceX + request.targetX) / 2,
      };
      bundle.requests.push(request);
      bundleById.set(bundleId, bundle);
    }
    const bundles = [...bundleById.values()].sort((left, right) => (
      left.sortX - right.sortX || left.id.localeCompare(right.id)
    ));
    const firstRequest = corridor[0];
    const laneYs = executionProcessRouteLaneYs(
      firstRequest.sourceY,
      firstRequest.targetY,
      bundles.length,
    );
    for (let index = 0; index < bundles.length; index += 1) {
      const bundle = bundles[index];
      const laneY = laneYs[index];
      for (const request of bundle.requests) {
        pathById.set(
          request.edge.id,
          buildProcessLaneEdgePath(request, laneY, bundle.requests.length > 1),
        );
      }
      if (bundle.junctionKind && bundle.requests.length > 1) {
        const request = bundle.requests[0];
        junctions.push({
          edgeIds: bundle.requests.map((item) => item.edge.id),
          id: `junction:${corridorKey}:${bundle.id}`,
          kind: bundle.junctionKind,
          x: bundle.junctionKind === "fan-out" ? request.sourceX : request.targetX,
          y: laneY,
        });
      }
    }
  }
  return {
    junctions,
    pathById,
    sourceTailPathById,
    targetTailPathById,
  };
}

function buildExecutionGraphBoundaryPortTail(
  node: ExecutionGraphNodeLayout,
  port: ExecutionGraphBoundaryPortLayout,
): string {
  if (port.side === "bottom" || port.side === "top") {
    const nodeY = node.y + (port.side === "bottom" ? 1 : -1) * node.height / 2;
    const innerY = port.y + (port.side === "bottom" ? -1 : 1)
      * BOUNDARY_PORT_TAIL_GUTTER;
    return [
      `M ${node.x} ${nodeY}`,
      `L ${node.x} ${innerY}`,
      `L ${port.x} ${innerY}`,
      `L ${port.x} ${port.y}`,
    ].join(" ");
  }
  const nodeX = node.x + (port.side === "right" ? 1 : -1) * node.width / 2;
  const innerX = port.x + (port.side === "right" ? -1 : 1)
    * BOUNDARY_PORT_TAIL_GUTTER;
  return [
    `M ${nodeX} ${node.y}`,
    `L ${innerX} ${node.y}`,
    `L ${innerX} ${port.y}`,
    `L ${port.x} ${port.y}`,
  ].join(" ");
}

function buildObstacleAvoidingGraphEdgePath(
  source: ExecutionGraphRoutePoint,
  target: ExecutionGraphRoutePoint,
  sourcePort: ExecutionGraphBoundaryPortLayout | null,
  targetPort: ExecutionGraphBoundaryPortLayout | null,
  groups: ExecutionGraphGroupLayout[],
): string {
  const obstacles = groups.map((group) => ({
    bottom: group.y + group.height + PROCESS_EDGE_GROUP_CLEARANCE,
    id: group.id,
    left: group.x - PROCESS_EDGE_GROUP_CLEARANCE,
    right: group.x + group.width + PROCESS_EDGE_GROUP_CLEARANCE,
    top: group.y - PROCESS_EDGE_GROUP_CLEARANCE,
  }));
  const sourceEscape = sourcePort
    ? executionGraphBoundaryPortEscape(sourcePort)
    : source;
  const targetEscape = targetPort
    ? executionGraphBoundaryPortEscape(targetPort)
    : target;
  const routed = findExecutionGraphOrthogonalRoute(
    sourceEscape,
    targetEscape,
    obstacles,
  );
  const points = compactExecutionGraphRoutePoints([
    source,
    sourceEscape,
    ...routed,
    targetEscape,
    target,
  ]);
  return points.map((point, index) => (
    `${index === 0 ? "M" : "L"} ${point.x} ${point.y}`
  )).join(" ");
}

function executionGraphBoundaryPortEscape(
  port: ExecutionGraphBoundaryPortLayout,
): ExecutionGraphRoutePoint {
  if (port.side === "top") {
    return { x: port.x, y: port.y - PROCESS_EDGE_GROUP_CLEARANCE };
  }
  if (port.side === "bottom") {
    return { x: port.x, y: port.y + PROCESS_EDGE_GROUP_CLEARANCE };
  }
  if (port.side === "left") {
    return { x: port.x - PROCESS_EDGE_GROUP_CLEARANCE, y: port.y };
  }
  return { x: port.x + PROCESS_EDGE_GROUP_CLEARANCE, y: port.y };
}

function executionGraphNodeSideAnchor(
  node: ExecutionGraphNodeLayout,
  side: "bottom" | "left" | "right" | "top",
): ExecutionGraphRoutePoint {
  if (side === "top") {
    return { x: node.x, y: node.y - node.height / 2 };
  }
  if (side === "bottom") {
    return { x: node.x, y: node.y + node.height / 2 };
  }
  if (side === "left") {
    return { x: node.x - node.width / 2, y: node.y };
  }
  return { x: node.x + node.width / 2, y: node.y };
}

function findExecutionGraphOrthogonalRoute(
  source: ExecutionGraphRoutePoint,
  target: ExecutionGraphRoutePoint,
  obstacles: ExecutionGraphRouteObstacle[],
): ExecutionGraphRoutePoint[] {
  const horizontalCoordinates = new Set([source.x, target.x]);
  const verticalCoordinates = new Set([source.y, target.y]);
  for (const obstacle of obstacles) {
    horizontalCoordinates.add(obstacle.left);
    horizontalCoordinates.add(obstacle.right);
    verticalCoordinates.add(obstacle.top);
    verticalCoordinates.add(obstacle.bottom);
  }
  if (obstacles.length > 0) {
    horizontalCoordinates.add(
      Math.min(...obstacles.map((obstacle) => obstacle.left))
        - PROCESS_EDGE_GROUP_CLEARANCE,
    );
    horizontalCoordinates.add(
      Math.max(...obstacles.map((obstacle) => obstacle.right))
        + PROCESS_EDGE_GROUP_CLEARANCE,
    );
    verticalCoordinates.add(
      Math.min(...obstacles.map((obstacle) => obstacle.top))
        - PROCESS_EDGE_GROUP_CLEARANCE,
    );
    verticalCoordinates.add(
      Math.max(...obstacles.map((obstacle) => obstacle.bottom))
        + PROCESS_EDGE_GROUP_CLEARANCE,
    );
  }
  const xs = [...horizontalCoordinates].sort((left, right) => left - right);
  const ys = [...verticalCoordinates].sort((left, right) => left - right);
  const points: ExecutionGraphRoutePoint[] = [];
  const pointIndexByCoordinate = new Map<string, number>();
  for (const y of ys) {
    for (const x of xs) {
      const point = { x, y };
      if (obstacles.some((obstacle) => executionGraphPointInsideObstacle(
        point,
        obstacle,
      ))) {
        continue;
      }
      pointIndexByCoordinate.set(executionGraphRoutePointKey(point), points.length);
      points.push(point);
    }
  }
  const sourceIndex = pointIndexByCoordinate.get(executionGraphRoutePointKey(source));
  const targetIndex = pointIndexByCoordinate.get(executionGraphRoutePointKey(target));
  if (sourceIndex === undefined || targetIndex === undefined) {
    return [source, target];
  }
  const neighbors = new Map<number, Array<{
    direction: "horizontal" | "vertical";
    distance: number;
    pointIndex: number;
  }>>();
  const addNeighbor = (
    from: number,
    to: number,
    direction: "horizontal" | "vertical",
  ) => {
    const fromPoint = points[from];
    const toPoint = points[to];
    if (!executionGraphRouteSegmentClear(fromPoint, toPoint, obstacles)) {
      return;
    }
    const distance = Math.abs(fromPoint.x - toPoint.x)
      + Math.abs(fromPoint.y - toPoint.y);
    neighbors.set(from, [
      ...(neighbors.get(from) ?? []),
      { direction, distance, pointIndex: to },
    ]);
    neighbors.set(to, [
      ...(neighbors.get(to) ?? []),
      { direction, distance, pointIndex: from },
    ]);
  };
  for (const y of ys) {
    const row = xs.flatMap((x) => {
      const index = pointIndexByCoordinate.get(executionGraphRoutePointKey({ x, y }));
      return index === undefined ? [] : [index];
    });
    for (let index = 1; index < row.length; index += 1) {
      addNeighbor(row[index - 1], row[index], "horizontal");
    }
  }
  for (const x of xs) {
    const column = ys.flatMap((y) => {
      const index = pointIndexByCoordinate.get(executionGraphRoutePointKey({ x, y }));
      return index === undefined ? [] : [index];
    });
    for (let index = 1; index < column.length; index += 1) {
      addNeighbor(column[index - 1], column[index], "vertical");
    }
  }
  return resolveExecutionGraphShortestRoute(
    points,
    neighbors,
    sourceIndex,
    targetIndex,
  );
}

function resolveExecutionGraphShortestRoute(
  points: ExecutionGraphRoutePoint[],
  neighbors: Map<number, Array<{
    direction: "horizontal" | "vertical";
    distance: number;
    pointIndex: number;
  }>>,
  sourceIndex: number,
  targetIndex: number,
): ExecutionGraphRoutePoint[] {
  const startKey = `${sourceIndex}:start`;
  const distanceByKey = new Map([[startKey, 0]]);
  const previousByKey = new Map<string, string>();
  const stateByKey = new Map<string, ExecutionGraphRouteState>();
  const queue: ExecutionGraphRouteState[] = [{
    cost: 0,
    direction: "start",
    key: startKey,
    pointIndex: sourceIndex,
  }];
  stateByKey.set(startKey, queue[0]);
  let targetState: ExecutionGraphRouteState | null = null;
  while (queue.length > 0) {
    const current = executionGraphRouteQueuePop(queue);
    if (!current || current.cost !== distanceByKey.get(current.key)) {
      continue;
    }
    if (current.pointIndex === targetIndex) {
      targetState = current;
      break;
    }
    for (const neighbor of neighbors.get(current.pointIndex) ?? []) {
      const bendCost = current.direction !== "start"
        && current.direction !== neighbor.direction
        ? PROCESS_EDGE_BEND_COST
        : 0;
      const nextCost = current.cost + neighbor.distance + bendCost;
      const nextKey = `${neighbor.pointIndex}:${neighbor.direction}`;
      if (nextCost >= (distanceByKey.get(nextKey) ?? Number.POSITIVE_INFINITY)) {
        continue;
      }
      const nextState: ExecutionGraphRouteState = {
        cost: nextCost,
        direction: neighbor.direction,
        key: nextKey,
        pointIndex: neighbor.pointIndex,
      };
      distanceByKey.set(nextKey, nextCost);
      previousByKey.set(nextKey, current.key);
      stateByKey.set(nextKey, nextState);
      executionGraphRouteQueuePush(queue, nextState);
    }
  }
  if (!targetState) {
    return [points[sourceIndex], points[targetIndex]];
  }
  const route: ExecutionGraphRoutePoint[] = [];
  let currentKey: string | undefined = targetState.key;
  while (currentKey) {
    const state = stateByKey.get(currentKey);
    if (!state) {
      break;
    }
    route.push(points[state.pointIndex]);
    currentKey = previousByKey.get(currentKey);
  }
  return compactExecutionGraphRoutePoints(route.reverse());
}

function executionGraphRouteQueuePush(
  queue: ExecutionGraphRouteState[],
  state: ExecutionGraphRouteState,
): void {
  queue.push(state);
  let index = queue.length - 1;
  while (index > 0) {
    const parent = Math.floor((index - 1) / 2);
    if (!executionGraphRouteStateBefore(queue[index], queue[parent])) {
      break;
    }
    [queue[index], queue[parent]] = [queue[parent], queue[index]];
    index = parent;
  }
}

function executionGraphRouteQueuePop(
  queue: ExecutionGraphRouteState[],
): ExecutionGraphRouteState | undefined {
  const first = queue[0];
  const last = queue.pop();
  if (!first || !last || queue.length === 0) {
    return first;
  }
  queue[0] = last;
  let index = 0;
  while (true) {
    const left = index * 2 + 1;
    const right = left + 1;
    let next = index;
    if (left < queue.length && executionGraphRouteStateBefore(queue[left], queue[next])) {
      next = left;
    }
    if (right < queue.length && executionGraphRouteStateBefore(queue[right], queue[next])) {
      next = right;
    }
    if (next === index) {
      return first;
    }
    [queue[index], queue[next]] = [queue[next], queue[index]];
    index = next;
  }
}

function executionGraphRouteStateBefore(
  left: ExecutionGraphRouteState,
  right: ExecutionGraphRouteState,
): boolean {
  return left.cost < right.cost
    || (left.cost === right.cost && left.key.localeCompare(right.key) < 0);
}

function executionGraphPointInsideObstacle(
  point: ExecutionGraphRoutePoint,
  obstacle: ExecutionGraphRouteObstacle,
): boolean {
  return point.x > obstacle.left
    && point.x < obstacle.right
    && point.y > obstacle.top
    && point.y < obstacle.bottom;
}

function executionGraphRouteSegmentClear(
  source: ExecutionGraphRoutePoint,
  target: ExecutionGraphRoutePoint,
  obstacles: ExecutionGraphRouteObstacle[],
): boolean {
  const segment: ExecutionGraphPathSegment = Math.abs(source.x - target.x) < 0.5
    ? {
        axis: "vertical",
        fixed: source.x,
        start: Math.min(source.y, target.y),
        end: Math.max(source.y, target.y),
      }
    : {
        axis: "horizontal",
        fixed: source.y,
        start: Math.min(source.x, target.x),
        end: Math.max(source.x, target.x),
      };
  return obstacles.every((obstacle) => {
    if (segment.axis === "vertical") {
      return segment.fixed <= obstacle.left
        || segment.fixed >= obstacle.right
        || executionGraphRangeOverlap(
          segment.start,
          segment.end,
          obstacle.top,
          obstacle.bottom,
        ) <= 0;
    }
    return segment.fixed <= obstacle.top
      || segment.fixed >= obstacle.bottom
      || executionGraphRangeOverlap(
        segment.start,
        segment.end,
        obstacle.left,
        obstacle.right,
      ) <= 0;
  });
}

function executionGraphRoutePointKey(point: ExecutionGraphRoutePoint): string {
  return `${point.x}:${point.y}`;
}

function compactExecutionGraphRoutePoints(
  points: ExecutionGraphRoutePoint[],
): ExecutionGraphRoutePoint[] {
  const result: ExecutionGraphRoutePoint[] = [];
  for (const point of points) {
    const previous = result[result.length - 1];
    if (previous && previous.x === point.x && previous.y === point.y) {
      continue;
    }
    const beforePrevious = result[result.length - 2];
    if (
      beforePrevious
      && (
        beforePrevious.x === previous.x && previous.x === point.x
        || beforePrevious.y === previous.y && previous.y === point.y
      )
    ) {
      result[result.length - 1] = point;
      continue;
    }
    result.push(point);
  }
  return result;
}

function executionProcessRouteClass(kind: ExecutionGraphEdgeKind): string {
  if (isExecutionControlEdge(kind)) {
    return "control";
  }
  if (kind === "invoke" || kind === "spawn" || kind === "guard") {
    return "ownership";
  }
  if (kind === "review") {
    return "review";
  }
  return "workflow";
}

function executionProcessRouteEndpointCounts(
  requests: ExecutionProcessRouteRequest[],
  endpoint: "source" | "target",
): Map<string, number> {
  const result = new Map<string, number>();
  for (const request of requests) {
    const nodeId = endpoint === "source"
      ? request.edge.source_node_id
      : request.edge.target_node_id;
    const key = `${request.routeClass}:${nodeId}`;
    result.set(key, (result.get(key) ?? 0) + 1);
  }
  return result;
}

function executionProcessRouteLaneYs(
  sourceY: number,
  targetY: number,
  count: number,
): number[] {
  if (count <= 1) {
    return [(sourceY + targetY) / 2];
  }
  const minimum = sourceY + PROCESS_EDGE_CORRIDOR_PADDING;
  const maximum = targetY - PROCESS_EDGE_CORRIDOR_PADDING;
  if (maximum <= minimum) {
    return Array.from({ length: count }, () => (sourceY + targetY) / 2);
  }
  const gap = Math.min(
    PROCESS_EDGE_LANE_GAP,
    (maximum - minimum) / (count - 1),
  );
  const span = gap * (count - 1);
  const start = (sourceY + targetY - span) / 2;
  return Array.from({ length: count }, (_, index) => start + gap * index);
}

function buildProcessLaneEdgePath(
  request: ExecutionProcessRouteRequest,
  laneY: number,
  bundled: boolean,
): string {
  if (!bundled && Math.abs(request.sourceX - request.targetX) < 1) {
    return `M ${request.sourceX} ${request.sourceY} L ${request.targetX} ${request.targetY}`;
  }
  return [
    `M ${request.sourceX} ${request.sourceY}`,
    `L ${request.sourceX} ${laneY}`,
    `L ${request.targetX} ${laneY}`,
    `L ${request.targetX} ${request.targetY}`,
  ].join(" ");
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
    + (returnsUpward ? source.height / 2 : -source.height / 2);
  const targetY = target.y
    + (returnsUpward ? target.height / 2 : -target.height / 2);
  const sourceLayerBoundary = executionControlSourceLayerBoundary(
    source,
    context.nodes,
    returnsUpward,
  );
  const preferredSide: -1 | 1 = source.x < target.x ? -1 : 1;
  const sides: Array<-1 | 1> = [preferredSide, preferredSide === -1 ? 1 : -1];
  const result: ExecutionControlRouteCandidate[] = [];
  for (const side of sides) {
    const targetX = target.x + side * target.width / 2;
    const baseRailX = context.group
      ? side < 0
        ? context.group.x + CONTROL_EDGE_FRAME_SAFE_GAP
        : context.group.x + context.group.width - CONTROL_EDGE_FRAME_SAFE_GAP
      : side < 0
        ? Math.min(source.x - source.width / 2, targetX) - CONTROL_EDGE_GUTTER
        : Math.max(source.x + source.width / 2, targetX) + CONTROL_EDGE_GUTTER;
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
  const sourceTop = source.y - source.height / 2;
  const sourceBottom = source.y + source.height / 2;
  const peers = nodes.filter((node) => (
    node.y - node.height / 2 < sourceBottom + 1
      && node.y + node.height / 2 > sourceTop - 1
  ));
  return returnsUpward
    ? Math.max(...peers.map((node) => node.y + node.height / 2))
    : Math.min(...peers.map((node) => node.y - node.height / 2));
}

function buildSiblingControlEdgeCandidate(
  source: ExecutionGraphNodeLayout,
  target: ExecutionGraphNodeLayout,
  lane: number,
): ExecutionControlRouteCandidate {
  const sourceY = source.y + source.height / 2;
  const targetY = target.y + target.height / 2;
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
  const halfWidth = node.width / 2 + CONTROL_EDGE_NODE_CLEARANCE;
  const halfHeight = node.height / 2 + CONTROL_EDGE_NODE_CLEARANCE;
  const left = node.x - halfWidth;
  const right = node.x + halfWidth;
  const top = node.y - halfHeight;
  const bottom = node.y + halfHeight;
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

function executionGraphPathMidpoint(path: string): { x: number; y: number } {
  const points = Array.from(
    path.matchAll(/[ML]\s+(-?\d+(?:\.\d+)?)\s+(-?\d+(?:\.\d+)?)/g),
    (match) => ({ x: Number(match[1]), y: Number(match[2]) }),
  );
  if (points.length === 0) {
    return { x: 0, y: 0 };
  }
  const lengths = points.slice(1).map((point, index) => (
    Math.abs(point.x - points[index].x) + Math.abs(point.y - points[index].y)
  ));
  const halfway = lengths.reduce((total, length) => total + length, 0) / 2;
  let traversed = 0;
  for (let index = 0; index < lengths.length; index += 1) {
    const length = lengths[index];
    if (traversed + length < halfway || length === 0) {
      traversed += length;
      continue;
    }
    const start = points[index];
    const end = points[index + 1];
    const ratio = (halfway - traversed) / length;
    return {
      x: start.x + (end.x - start.x) * ratio,
      y: start.y + (end.y - start.y) * ratio,
    };
  }
  return points[points.length - 1];
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
  const sourceY = source.y + source.height / 2;
  const targetY = target.y - target.height / 2;
  if (targetY <= sourceY) {
    const reverseSourceY = source.y - source.height / 2;
    const reverseTargetY = target.y + target.height / 2;
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
