/**
 * INPUT: ExecutionView、持久 Agent 目录与 runtime Subagent identity。
 * OUTPUT: 状态/类型文案键、当前节点、可展示摘要、稳定 Agent/Subagent 头像身份、一级 Agent 活动态、依赖深度与 WorkGraph 生命周期判定。
 * POS: WorkGraph 纯协议到轻量进程展示语义的无状态投影。
 */
import { stripRoomControlMarkers } from "@/features/conversation/shared/message/message-content-model";
import { getSeededAvatarDataUrl } from "@/lib/seeded-avatar";
import type { TranslationKey } from "@/shared/i18n/messages";
import type {
  ExecutionGraphNodeView,
  ExecutionStatus,
  ExecutionView,
  ExecutionWorkItemStatus,
  ExecutionWorkItemView,
} from "@/types/conversation/execution";

export interface ExecutionAgentIdentity {
  avatar: string | null;
  id: string;
  name: string;
}

export type ExecutionAgentDirectory = Record<string, ExecutionAgentIdentity>;

interface ExecutionAgentDirectorySource {
  agent_id: string;
  avatar?: string | null;
  name: string;
}

const EXECUTION_INTERNAL_SENTINEL_PATTERN = /__nexus_[a-z0-9_]+__/gi;

export function normalizeExecutionNodeDisplayText(value: string): string {
  return stripRoomControlMarkers(
    value.replace(EXECUTION_INTERNAL_SENTINEL_PATTERN, " "),
  ).replace(/\s+/g, " ").trim();
}

export function buildExecutionAgentDirectory(
  agents: readonly ExecutionAgentDirectorySource[],
): ExecutionAgentDirectory {
  return Object.fromEntries(agents.map((agent) => [
    agent.agent_id,
    {
      avatar: agent.avatar ?? null,
      id: agent.agent_id,
      name: agent.name,
    },
  ]));
}

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
  currentNode: ExecutionGraphNodeView | null;
  currentStep: number;
  summary: string;
  totalCount: number;
}

export interface ExecutionWorkGraphHeaderModel {
  currentNodeId: string | null;
  status: ExecutionStatus;
  statusLabelKey: TranslationKey;
  summary: string;
}

/**
 * plan_execution 只有在原子写入 active Plan 与非空 Work Items 后才算
 * 成功创建工作图。普通对话轮次的 runtime 观测节点不能触发任何
 * WorkGraph UI。
 */
export function hasManagedExecutionGraph(
  execution: ExecutionView | null,
): boolean {
  return Boolean(
    execution
    && execution.plan?.status === "active"
    && (execution.work_items?.length ?? 0) > 0,
  );
}

/**
 * Runtime-only Agent Loop 与托管 WorkGraph 共用同一读模型。这个判定
 * 只供图内部节点投影使用，不代表 WorkGraph UI 已可用。
 */
export function hasExecutionGraph(
  execution: ExecutionView | null,
): boolean {
  return Boolean(
    execution
    && ((execution.graph?.nodes?.length ?? 0) > 0
      || (execution.work_items?.length ?? 0) > 0),
  );
}

export function isExecutionActivityVisible(
  execution: ExecutionView | null,
): execution is ExecutionView {
  return Boolean(
    hasManagedExecutionGraph(execution)
    && execution
    && !isTerminalExecutionStatus(execution.status),
  );
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
  const graphNodes = orderedExecutionGraphNodes(execution);
  let currentNode = current
    ? graphNodes.find((node) => (
        node.work_item_id === current?.id
        && node.kind === "subagent"
        && node.run_status === "running"
      ))
      ?? graphNodes.find((node) => (
        node.work_item_id === current?.id && node.kind === "agent"
      ))
      ?? null
    : graphNodes.find((node) => (
      node.kind !== "agent"
      && resolveExecutionGraphNodeStatus(node, null) === "running"
    ))
      ?? graphNodes.find((node) => resolveExecutionGraphNodeStatus(node, null) === "running")
      ?? graphNodes[0]
      ?? null;
  const currentIndex = current
    ? items.findIndex((item) => item.id === current?.id)
    : currentNode
      ? graphNodes.findIndex((node) => node.id === currentNode?.id)
      : -1;
  const totalCount = items.length > 0 ? items.length : graphNodes.length;
  return {
    current,
    currentNode,
    currentStep: currentIndex >= 0 ? currentIndex + 1 : 0,
    summary: current?.subject.trim()
      || currentNode?.description?.trim()
      || currentNode?.name?.trim()
      || execution.objective.trim(),
    totalCount,
  };
}

/**
 * 主视图标题只投影当前摘要与节点定位。生命周期状态留给 waiting、paused
 * 和 terminal 等需要用户注意的顶栏提示，详细进度留在图与详情中。
 */
export function resolveExecutionWorkGraphHeaderModel(
  execution: ExecutionView,
): ExecutionWorkGraphHeaderModel {
  const nodeSummary = resolveExecutionNodeSummary(execution);
  return {
    currentNodeId: nodeSummary.currentNode?.id ?? null,
    status: execution.status,
    statusLabelKey: EXECUTION_STATUS_LABEL_KEY[execution.status],
    summary: nodeSummary.summary,
  };
}

/**
 * Composer 只表达参与当前托管执行的一级 Agent，不重复展示同一 Agent 的
 * Work Item，也不把 Subagent、Tool 或 Gate 混入实时跳转入口。
 */
export function resolveExecutionPrimaryAgentNodes(
  execution: ExecutionView,
  limit = 5,
): ExecutionGraphNodeView[] {
  const selectedByAgent = new Map<string, ExecutionGraphNodeView>();
  for (const node of orderedExecutionGraphNodes(execution)) {
    if (node.kind !== "agent" || node.visibility !== "primary") {
      continue;
    }
    const item = resolveExecutionGraphNodeItem(execution, node);
    const agentKey = node.agent_id?.trim()
      || item?.owner_agent_id?.trim();
    if (!agentKey) {
      continue;
    }
    const selected = selectedByAgent.get(agentKey);
    if (!selected) {
      selectedByAgent.set(agentKey, node);
      continue;
    }
    const selectedItem = resolveExecutionGraphNodeItem(execution, selected);
    const selectedPriority = WORK_ITEM_FOCUS_PRIORITY.indexOf(
      resolveExecutionGraphNodeStatus(selected, selectedItem),
    );
    const candidatePriority = WORK_ITEM_FOCUS_PRIORITY.indexOf(
      resolveExecutionGraphNodeStatus(node, item),
    );
    if (candidatePriority < selectedPriority) {
      selectedByAgent.set(agentKey, node);
    }
  }
  return [...selectedByAgent.values()]
    .sort((left, right) => (
      primaryAgentNodeRank(execution, left)
      - primaryAgentNodeRank(execution, right)
      || left.position - right.position
      || left.id.localeCompare(right.id)
    ))
    .slice(0, Math.max(0, limit));
}

function primaryAgentNodeRank(
  execution: ExecutionView,
  node: ExecutionGraphNodeView,
): number {
  const coordinatorAgentId = execution.coordinator_agent_id?.trim();
  if (coordinatorAgentId && node.agent_id?.trim() === coordinatorAgentId) {
    return 0;
  }
  if (execution.graph?.edges?.some((edge) => (
    (edge.kind === "coordination" || edge.kind === "dispatch")
    && edge.source_node_id === node.id
  ))) {
    return 0;
  }
  return 1;
}

export function orderedExecutionGraphNodes(
  execution: ExecutionView,
): ExecutionGraphNodeView[] {
  const projected = (execution.graph?.nodes ?? []).filter(
    (node) => node.visibility !== "detail",
  );
  if (projected.length > 0) {
    return [...projected].sort((left, right) => (
      left.position - right.position
      || graphNodeKindOrder(left) - graphNodeKindOrder(right)
      || left.id.localeCompare(right.id)
    ));
  }
  return orderedExecutionItems(execution).map((item) => ({
    agent_id: item.owner_agent_id,
    id: item.id,
    kind: "agent",
    position: item.position,
    responsibility_status: item.status,
    visibility: "primary",
    work_item_id: item.id,
  }));
}

export function resolveExecutionGraphNodeItem(
  execution: ExecutionView,
  node: ExecutionGraphNodeView,
): ExecutionWorkItemView | null {
  return execution.work_items?.find((item) => item.id === node.work_item_id)
    ?? null;
}

export function resolveExecutionGraphNodeStatus(
  node: ExecutionGraphNodeView,
  item: ExecutionWorkItemView | null,
): ExecutionWorkItemStatus {
  if (node.kind === "agent" && (node.responsibility_status || item?.status)) {
    return node.responsibility_status ?? item?.status ?? "waiting";
  }
  switch (node.lifecycle_status) {
    case "aligned":
    case "accepted":
    case "succeeded":
      return "accepted";
    case "running":
    case "claimed":
      return "running";
    case "pending":
    case "planned":
      return "assigned";
    case "delivered":
      return "submitted";
    case "changes_requested":
    case "not_aligned":
      return "changes_requested";
    case "inconclusive":
      return "blocked";
    case "failed":
    case "rejected":
    case "interrupted":
      return "failed";
    case "cancelled":
      return "cancelled";
  }
  switch (node.run_status) {
    case "pending":
      return "assigned";
    case "running":
      return "running";
    case "succeeded":
      return "accepted";
    case "failed":
    case "interrupted":
    case "timed_out":
      return "failed";
    case "cancelled":
      return "cancelled";
    default:
      return item?.status ?? "waiting";
  }
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

export function resolveExecutionGraphNodeAgent(
  directory: ExecutionAgentDirectory,
  node: ExecutionGraphNodeView,
  item: ExecutionWorkItemView | null,
): ExecutionAgentIdentity | null {
  if (node.kind === "tool") {
    return null;
  }
  if (node.kind === "subagent") {
    const identity = node.subject_id?.trim()
      || node.attempt_id?.trim()
      || node.agent_id?.trim()
      || node.id;
    return {
      avatar: getSeededAvatarDataUrl(identity),
      id: `subagent:${identity}`,
      name: node.name?.trim() || node.agent_id?.trim() || "Subagent",
    };
  }
  return resolveExecutionAgent(
    directory,
    node.agent_id ?? item?.owner_agent_id,
  );
}

export function compactExecutionNodeObjective(
  objective: string,
  ownerName: string | undefined,
): string {
  const value = objective.trim();
  const owner = ownerName?.trim() ?? "";
  if (!owner || value.length <= owner.length) {
    return value;
  }
  const prefix = value.slice(0, owner.length);
  const remainder = value.slice(owner.length);
  const separator = remainder.match(/^(?:\s*-\s+|[\s:：·,，—–]+)/u)?.[0];
  if (prefix.toLocaleLowerCase() !== owner.toLocaleLowerCase() || !separator) {
    return value;
  }
  return remainder.slice(separator.length).trim() || value;
}

function graphNodeKindOrder(node: ExecutionGraphNodeView): number {
  switch (node.kind) {
    case "agent":
      return 0;
    case "subagent":
      return 1;
    case "tool":
      return 2;
    case "gate":
      return 3;
  }
}
