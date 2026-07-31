/**
 * INPUT: ExecutionView、当前选中 Work Item 与 Agent 目录。
 * OUTPUT: 状态/类型文案键、摘要段、默认焦点与 terminal 判定。
 * POS: WorkGraph 纯协议到展示语义的无状态投影。
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

export interface ExecutionSummaryPart {
  count: number;
  key: TranslationKey;
}

export function buildExecutionSummaryParts(
  execution: ExecutionView,
): ExecutionSummaryPart[] {
  const { progress } = execution;
  const parts: ExecutionSummaryPart[] = [
    { count: progress.running, key: "execution.summary_running" },
    { count: progress.blocked, key: "execution.summary_blocked" },
    { count: progress.submitted, key: "execution.summary_submitted" },
    { count: progress.ready, key: "execution.summary_ready" },
  ];
  return parts.filter((part) => part.count > 0).slice(0, 2);
}

export function resolveSelectedWorkItem(
  execution: ExecutionView,
  selectedId: string | null,
): ExecutionWorkItemView | null {
  const items = execution.work_items ?? [];
  const selected = items.find((item) => item.id === selectedId);
  if (selected) {
    return selected;
  }
  for (const status of WORK_ITEM_FOCUS_PRIORITY) {
    const item = items.find((candidate) => candidate.status === status);
    if (item) {
      return item;
    }
  }
  return items[0] ?? null;
}

export function resolveWorkItemDepths(
  execution: ExecutionView,
): Record<string, number> {
  const items = execution.work_items ?? [];
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
    result[item.id] = Math.min(depth, 3);
    return result[item.id];
  };
  for (const item of items) {
    resolveDepth(item, new Set());
  }
  return result;
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
