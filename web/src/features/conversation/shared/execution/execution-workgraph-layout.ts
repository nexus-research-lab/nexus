/**
 * INPUT: 权威 Execution Work Item、依赖关系与当前画布可用宽度。
 * OUTPUT: 优先在可用宽度内收敛层间距、空间不足时才横向滚动的 DAG 坐标与边路径。
 * POS: WorkGraph 协议到交互画布之间的无状态布局投影，不持有 React 状态。
 */
import type {
  ExecutionView,
  ExecutionWorkItemView,
} from "@/types/conversation/execution";

import {
  orderedExecutionItems,
  resolveWorkItemDepths,
} from "./execution-process-model";

const NODE_SIZE = 48;
const PREFERRED_HORIZONTAL_STEP = 118;
const MIN_HORIZONTAL_STEP = 82;
const VERTICAL_STEP = 78;
const HORIZONTAL_PADDING = 36;
const VERTICAL_PADDING = 28;
const MIN_CANVAS_WIDTH = 340;
const MIN_CANVAS_HEIGHT = 136;

export interface ExecutionGraphNodeLayout {
  item: ExecutionWorkItemView;
  x: number;
  y: number;
}

export interface ExecutionGraphEdgeLayout {
  id: string;
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
  const items = orderedExecutionItems(execution);
  if (items.length === 0) {
    return {
      edges: [],
      height: MIN_CANVAS_HEIGHT,
      nodes: [],
      width: constrainedWidth === null
        ? MIN_CANVAS_WIDTH
        : Math.min(MIN_CANVAS_WIDTH, constrainedWidth),
    };
  }

  const depthById = resolveWorkItemDepths(execution);
  const layers = new Map<number, ExecutionWorkItemView[]>();
  let maxDepth = 0;
  for (const item of items) {
    const depth = depthById[item.id] ?? 0;
    maxDepth = Math.max(maxDepth, depth);
    const layer = layers.get(depth) ?? [];
    layer.push(item);
    layers.set(depth, layer);
  }

  const maxLayerSize = Math.max(
    1,
    ...Array.from(layers.values(), (layer) => layer.length),
  );
  const horizontalStep = resolveHorizontalStep(maxDepth, constrainedWidth);
  const naturalWidth = HORIZONTAL_PADDING * 2
    + NODE_SIZE
    + maxDepth * horizontalStep;
  const naturalHeight = VERTICAL_PADDING * 2
    + NODE_SIZE
    + (maxLayerSize - 1) * VERTICAL_STEP;
  const minimumWidth = constrainedWidth === null
    ? MIN_CANVAS_WIDTH
    : Math.min(MIN_CANVAS_WIDTH, constrainedWidth);
  const width = Math.max(minimumWidth, naturalWidth);
  const height = Math.max(MIN_CANVAS_HEIGHT, naturalHeight);
  const horizontalOffset = (width - naturalWidth) / 2;
  const nodes: ExecutionGraphNodeLayout[] = [];

  for (let depth = 0; depth <= maxDepth; depth += 1) {
    const layer = layers.get(depth) ?? [];
    const layerHeight = (layer.length - 1) * VERTICAL_STEP;
    layer.forEach((item, index) => {
      nodes.push({
        item,
        x: horizontalOffset
          + HORIZONTAL_PADDING
          + NODE_SIZE / 2
          + depth * horizontalStep,
        y: height / 2 - layerHeight / 2 + index * VERTICAL_STEP,
      });
    });
  }

  const nodeById = new Map(nodes.map((node) => [node.item.id, node]));
  const edges: ExecutionGraphEdgeLayout[] = [];
  for (const target of nodes) {
    const upstreamIds = new Set([
      ...(target.item.dependency_ids ?? []),
      ...(target.item.parent_work_item_id
        ? [target.item.parent_work_item_id]
        : []),
    ]);
    for (const sourceId of upstreamIds) {
      const source = nodeById.get(sourceId);
      if (!source) {
        continue;
      }
      edges.push({
        id: `${sourceId}:${target.item.id}`,
        path: buildEdgePath(source, target),
        sourceId,
        targetId: target.item.id,
      });
    }
  }

  return { edges, height, nodes, width };
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
    availableWidth - HORIZONTAL_PADDING * 2 - NODE_SIZE
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
  const sourceX = source.x + NODE_SIZE / 2;
  const targetX = target.x - NODE_SIZE / 2;
  const curve = Math.max(14, (targetX - sourceX) * 0.42);
  return [
    `M ${sourceX} ${source.y}`,
    `C ${sourceX + curve} ${source.y}`,
    `${targetX - curve} ${target.y}`,
    `${targetX} ${target.y}`,
  ].join(" ");
}
