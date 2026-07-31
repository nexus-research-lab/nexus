/**
 * INPUT: ExecutionView。
 * OUTPUT: 状态/类型文案键、当前节点、紧凑节点窗口、依赖深度与 terminal 判定。
 * POS: WorkGraph 纯协议到轻量进程展示语义的无状态投影。
 */
import type { TranslationKey } from "@/shared/i18n/messages";
import type {
  ExecutionStatus,
  ExecutionView,
  ExecutionWorkItemKind,
  ExecutionWorkItemStatus,
  ExecutionWorkItemView,
} from "@/types/conversation/execution";

export interface ExecutionAgentIdentity {
  avatar: string | null;
  id: string;
  name: string;
}

export type ExecutionAgentDirectory = Record<string, ExecutionAgentIdentity>;

export const EXECUTION_STATUS_LABEL_KEY: Record<
  ExecutionStatus,
  TranslationKey
> = {
  active: "execution.status_active",
  waiting: "execution.status_waiting",
  paused: "execution.status_paused",
  completed: "execution.status_completed",
  failed: "execution.status_failed",
  cancelled: "execution.status_cancelled",
  superseded: "execution.status_superseded",
};

export const WORK_ITEM_STATUS_LABEL_KEY: Record<
  ExecutionWorkItemStatus,
  TranslationKey
> = {
  waiting: "execution.work_status_waiting",
  ready: "execution.work_status_ready",
  assigned: "execution.work_status_assigned",
  running: "execution.work_status_running",
  blocked: "execution.work_status_blocked",
  submitted: "execution.work_status_submitted",
  changes_requested: "execution.work_status_changes_requested",
  accepted: "execution.work_status_accepted",
  failed: "execution.work_status_failed",
  cancelled: "execution.work_status_cancelled",
};

export const WORK_ITEM_KIND_LABEL_KEY: Record<
  ExecutionWorkItemKind,
  TranslationKey
> = {
  produce: "execution.kind_produce",
  review: "execution.kind_review",
  verify: "execution.kind_verify",
  integrate: "execution.kind_integrate",
};

const WORK_ITEM_FOCUS_PRIORITY: ExecutionWorkItemStatus[] = [
  "running",
  "blocked",
  "submitted",
  "changes_requested",
  "ready",
  "assigned",
  "waiting",
  "failed",
  "accepted",
  "cancelled",
];

export interface ExecutionNodeSummary {
  current: ExecutionWorkItemView | null;
  currentStep: number;
  summary: string;
  totalCount: number;
}

export interface ExecutionNodeWindow {
  hiddenAfter: number;
  hiddenBefore: number;
  items: ExecutionWorkItemView[];
}

export function resolveExecutionNodeSummary(
  execution: ExecutionView,
): ExecutionNodeSummary {
  const items = orderedExecutionItems(execution);
  let current: ExecutionWorkItemView | null = null;
  for (const status of WORK_ITEM_FOCUS_PRIORITY) {
    const item = items.find((candidate) => candidate.status === status);
    if (item) {
      current = item;
      break;
    }
  }
  current ??= items[0] ?? null;
  const currentIndex = current
    ? items.findIndex((item) => item.id === current?.id)
    : -1;
  return {
    current,
    currentStep: currentIndex >= 0 ? currentIndex + 1 : 0,
    summary: current?.subject.trim() || execution.objective.trim(),
    totalCount: items.length,
  };
}

export function resolveExecutionNodeWindow(
  execution: ExecutionView,
  focusId: string | null,
  limit = 7,
): ExecutionNodeWindow {
  const items = orderedExecutionItems(execution);
  if (items.length <= limit || limit <= 0) {
    return {
      hiddenAfter: 0,
      hiddenBefore: 0,
      items,
    };
  }
  const focusIndex = Math.max(
    0,
    items.findIndex((item) => item.id === focusId),
  );
  const halfWindow = Math.floor(limit / 2);
  const start = Math.min(
    Math.max(0, focusIndex - halfWindow),
    items.length - limit,
  );
  return {
    hiddenAfter: items.length - start - limit,
    hiddenBefore: start,
    items: items.slice(start, start + limit),
  };
}

export function resolveWorkItemDepths(
  execution: ExecutionView,
): Record<string, number> {
  const items = orderedExecutionItems(execution);
  const itemById = new Map(items.map((item) => [item.id, item]));
  const result: Record<string, number> = {};
  const resolveDepth = (
    item: ExecutionWorkItemView,
    visiting: Set<string>,
  ): number => {
    if (result[item.id] !== undefined) {
      return result[item.id];
    }
    if (visiting.has(item.id)) {
      return 0;
    }
    const nextVisiting = new Set(visiting).add(item.id);
    const upstreamIds = [
      ...(item.dependency_ids ?? []),
      ...(item.parent_work_item_id ? [item.parent_work_item_id] : []),
    ];
    let depth = 0;
    for (const upstreamId of upstreamIds) {
      const upstream = itemById.get(upstreamId);
      if (upstream) {
        depth = Math.max(depth, resolveDepth(upstream, nextVisiting) + 1);
      }
    }
    result[item.id] = depth;
    return result[item.id];
  };
  for (const item of items) {
    resolveDepth(item, new Set());
  }
  return result;
}

export function orderedExecutionItems(
  execution: ExecutionView,
): ExecutionWorkItemView[] {
  return [...(execution.work_items ?? [])].sort(
    (left, right) => left.position - right.position,
  );
}

export function isTerminalExecutionStatus(status: ExecutionStatus): boolean {
  return status === "completed"
    || status === "failed"
    || status === "cancelled"
    || status === "superseded";
}

export function resolveExecutionAgent(
  directory: ExecutionAgentDirectory,
  agentId: string | undefined,
): ExecutionAgentIdentity | null {
  const normalized = agentId?.trim() ?? "";
  if (!normalized) {
    return null;
  }
  return directory[normalized] ?? {
    avatar: null,
    id: normalized,
    name: normalized,
  };
}
