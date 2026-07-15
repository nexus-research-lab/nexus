/**
 * INPUT: Room 根轮次内的 user / assistant 消息、slot 与权限状态。
 * OUTPUT: root-global user，以及按精确消费 agent_round_id、终态时序和稳定活动槽排列的 Agent 卡片摘要。
 * POS: Group round feed 的唯一展示归组入口。
 */
import { isAutomationTriggerUserMessage } from "@/types/conversation/automation-message";
import type {
  AssistantMessage,
  Message,
  ResultSummary,
  UserMessage,
} from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

import { ASK_USER_QUESTION_TOOL_NAME } from "@/features/conversation/shared/message/message-tool-names";
import { stripRoomControlMarkers } from "@/features/conversation/shared/message/message-content-model";
import {
  buildRoomAgentRoundEntries,
  extractAgentPreviewText,
  isAgentRoundActive,
  type AgentRoundStatus,
  type RoomAgentRoundEntry,
} from "../../round/round-agent-model";

export interface GroupRoundUserMessageModel {
  message: UserMessage;
  workspaceAgentId: string | null;
}

export interface GroupRoundAgentCardModel extends RoomAgentRoundEntry {
  agentAvatar: string | null;
  agentName: string;
  guidedUserMessages: GroupRoundUserMessageModel[];
  pendingPermissions: PendingPermission[];
  stopMessageId: string | null;
}

export interface GroupRoundCardModel {
  entries: GroupRoundAgentCardModel[];
  userMessages: GroupRoundUserMessageModel[];
}

export type AgentStatusSummaryTone =
  | "default"
  | "error"
  | "stopped"
  | "waiting";

interface GroupAgentStatusLabels {
  failed: string;
  stopped: string;
  waitingPermission: string;
}

export interface GroupAgentStatusModel {
  isActive: boolean;
  isQuestionPending: boolean;
  isWaitingPermission: boolean;
  model: string | null;
  preview: string;
  primaryPendingPermission?: PendingPermission;
  shouldRenderMarkdownSummary: boolean;
  summaryText: string;
  summaryTone: AgentStatusSummaryTone;
  timestamp: number;
}

interface BuildGroupAgentStatusModelOptions {
  labels: GroupAgentStatusLabels;
  messages: AssistantMessage[];
  pendingPermissions: PendingPermission[];
  resultSummary?: ResultSummary;
  status: AgentRoundStatus;
  timestamp: number;
}

type GroupAgentStatusLabelKey = Exclude<
  keyof GroupAgentStatusLabels,
  "waitingPermission"
>;
type AgentSummarySource = "fallback" | "preview" | "result";

interface AgentStatusPresentationRule {
  fallbackLabel: GroupAgentStatusLabelKey | null;
  renderPreview: boolean;
  summaryOrder: AgentSummarySource[];
  tone: AgentStatusSummaryTone;
}

interface AgentStatusSummaryModel {
  shouldRenderMarkdown: boolean;
  text: string;
  tone: AgentStatusSummaryTone;
}

interface BuildGroupRoundCardModelOptions {
  agentAvatarMap: Record<string, string | null>;
  agentNameMap: Record<string, string>;
  messages: Message[];
  pendingPermissions: PendingPermission[];
  pendingSlots: RoomPendingAgentSlotState[];
}

interface PermissionGroups {
  byAgent: Map<string, PendingPermission[]>;
  byAgentRound: Map<string, PendingPermission[]>;
}

const AGENT_STATUS_PRESENTATION: Record<
  AgentRoundStatus,
  AgentStatusPresentationRule
> = {
  cancelled: {
    fallbackLabel: "stopped",
    renderPreview: false,
    summaryOrder: ["result", "fallback"],
    tone: "stopped",
  },
  done: {
    fallbackLabel: null,
    renderPreview: true,
    summaryOrder: ["preview", "fallback"],
    tone: "default",
  },
  error: {
    fallbackLabel: "failed",
    renderPreview: false,
    summaryOrder: ["result", "fallback"],
    tone: "error",
  },
  pending: {
    fallbackLabel: null,
    renderPreview: true,
    summaryOrder: ["preview", "fallback"],
    tone: "default",
  },
  streaming: {
    fallbackLabel: null,
    renderPreview: true,
    summaryOrder: ["preview", "fallback"],
    tone: "default",
  },
};

export function buildGroupRoundCardModel({
  agentAvatarMap,
  agentNameMap,
  messages,
  pendingPermissions,
  pendingSlots,
}: BuildGroupRoundCardModelOptions): GroupRoundCardModel {
  const entries = buildRoomAgentRoundEntries(messages, pendingSlots);
  const permissionGroups = buildPermissionGroups(pendingPermissions);
  const entriesByAgent = groupEntriesByAgent(entries);
  const userMessages: GroupRoundUserMessageModel[] = [];
  const guidedUserMessagesByEntry = new Map<
    string,
    GroupRoundUserMessageModel[]
  >();

  for (const message of messages
    .filter(isVisibleUserMessage)
    .sort((left, right) => left.timestamp - right.timestamp)) {
    const item = {
      message,
      workspaceAgentId: resolveUserWorkspaceAgentId(message),
    };
    const targetAgentId = resolveGuidedTargetAgentId(message);
    const targetEntry = targetAgentId
      ? resolveGuidedTargetEntry(entriesByAgent.get(targetAgentId) ?? [], message)
      : null;
    if (!targetEntry) {
      userMessages.push(item);
      continue;
    }
    const guidedMessages = guidedUserMessagesByEntry.get(targetEntry.entry_id) ?? [];
    guidedMessages.push(item);
    guidedUserMessagesByEntry.set(targetEntry.entry_id, guidedMessages);
  }

  const cards = entries.map((entry) => buildAgentCard(
    entry,
    agentAvatarMap,
    agentNameMap,
    permissionsForEntry(entry, entriesByAgent, permissionGroups),
    guidedUserMessagesByEntry.get(entry.entry_id) ?? [],
  )).sort(compareAgentCards);

  return {
    entries: cards,
    userMessages,
  };
}

export function buildGroupAgentStatusModel({
  labels,
  messages,
  pendingPermissions,
  resultSummary,
  status,
  timestamp,
}: BuildGroupAgentStatusModelOptions): GroupAgentStatusModel {
  const preview = extractAgentPreviewText(messages);
  const isActive = isAgentRoundActive(status);
  const permission = buildAgentPermissionState(pendingPermissions, isActive);
  const presentation = AGENT_STATUS_PRESENTATION[status];
  const summary = buildAgentStatusSummary({
    labels,
    permission,
    presentation,
    preview,
    resultText: resultSummaryText(resultSummary),
  });

  return {
    isActive,
    isQuestionPending: permission.isQuestionPending,
    isWaitingPermission: permission.isWaiting,
    model: lastMessageModel(messages),
    preview,
    primaryPendingPermission: permission.primary,
    shouldRenderMarkdownSummary: summary.shouldRenderMarkdown,
    summaryText: summary.text,
    summaryTone: summary.tone,
    timestamp,
  };
}

interface AgentPermissionState {
  isQuestionPending: boolean;
  isWaiting: boolean;
  primary?: PendingPermission;
}

function buildAgentPermissionState(
  pendingPermissions: PendingPermission[],
  isActive: boolean,
): AgentPermissionState {
  const primary = pendingPermissions[0];
  return {
    isQuestionPending: Boolean(
      primary &&
        (primary.interaction_mode === "question" ||
          primary.tool_name === ASK_USER_QUESTION_TOOL_NAME),
    ),
    isWaiting: primary !== undefined && isActive,
    primary,
  };
}

function buildAgentSummaryText({
  fallbackText,
  labels,
  permission,
  presentation,
  preview,
  resultText,
}: {
  fallbackText: string;
  labels: GroupAgentStatusLabels;
  permission: AgentPermissionState;
  presentation: AgentStatusPresentationRule;
  preview: string;
  resultText?: string;
}): string {
  if (permission.isWaiting) {
    return permission.primary?.summary || labels.waitingPermission;
  }
  const sources: Record<AgentSummarySource, string> = {
    fallback: fallbackText,
    preview,
    result: resultText?.trim() ?? "",
  };
  return (
    presentation.summaryOrder
      .map((source) => sources[source])
      .find(Boolean) ?? ""
  );
}

function buildAgentStatusSummary({
  labels,
  permission,
  presentation,
  preview,
  resultText,
}: {
  labels: GroupAgentStatusLabels;
  permission: AgentPermissionState;
  presentation: AgentStatusPresentationRule;
  preview: string;
  resultText?: string;
}): AgentStatusSummaryModel {
  return {
    shouldRenderMarkdown: Boolean(
      preview && !permission.isWaiting && presentation.renderPreview,
    ),
    text: buildAgentSummaryText({
      fallbackText: statusFallbackText(labels, presentation.fallbackLabel),
      labels,
      permission,
      presentation,
      preview,
      resultText,
    }),
    tone: permission.isWaiting ? "waiting" : presentation.tone,
  };
}

function statusFallbackText(
  labels: GroupAgentStatusLabels,
  labelKey: GroupAgentStatusLabelKey | null,
): string {
  return labelKey ? labels[labelKey] : "";
}

function lastMessageModel(messages: AssistantMessage[]): string | null {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const model = messages[index].model?.trim();
    if (model) {
      return model;
    }
  }
  return null;
}

function resultSummaryText(summary?: ResultSummary): string | undefined {
  const result = stripRoomControlMarkers(summary?.result ?? "");
  return result || undefined;
}

function buildAgentCard(
  entry: RoomAgentRoundEntry,
  agentAvatarMap: Record<string, string | null>,
  agentNameMap: Record<string, string>,
  pendingPermissions: PendingPermission[],
  guidedUserMessages: GroupRoundUserMessageModel[],
): GroupRoundAgentCardModel {
  return {
    ...entry,
    agentAvatar: resolveAgentAvatar(agentAvatarMap, entry.agent_id),
    agentName: resolveAgentName(agentNameMap, entry.agent_id),
    guidedUserMessages,
    pendingPermissions,
    stopMessageId: resolveStopMessageId(entry),
  };
}

function resolveAgentAvatar(
  avatarMap: Record<string, string | null>,
  agentId: string,
): string | null {
  return avatarMap[agentId] ?? null;
}

function resolveAgentName(
  nameMap: Record<string, string>,
  agentId: string,
): string {
  return nameMap[agentId] ?? agentId;
}

function resolveStopMessageId(entry: RoomAgentRoundEntry): string | null {
  if (!entry.pending_slot || !isAgentRoundActive(entry.status)) {
    return null;
  }
  return entry.pending_slot.agent_round_id;
}

function resolveUserWorkspaceAgentId(
  userMessage: UserMessage,
): string | null {
  return userMessage?.attachments?.[0]?.workspace_agent_id ?? null;
}

function resolveGuidedTargetAgentId(message: UserMessage): string | null {
  if (
    message.delivery_policy !== "guide"
    || !message.source_round_id?.trim()
    || !Array.isArray(message.target_agent_ids)
  ) {
    return null;
  }
  const targets = Array.from(new Set(
    message.target_agent_ids
      .filter((value): value is string => typeof value === "string")
      .map((value) => value.trim())
      .filter(Boolean),
  ));
  return targets.length === 1 ? targets[0] : null;
}

function buildPermissionGroups(permissions: PendingPermission[]): PermissionGroups {
  const byAgent = new Map<string, PendingPermission[]>();
  const byAgentRound = new Map<string, PendingPermission[]>();
  for (const permission of permissions) {
    if (!permission.agent_id) {
      continue;
    }
    const agentRoundId = permission.agent_round_id?.trim();
    const groups = agentRoundId ? byAgentRound : byAgent;
    const key = agentRoundId || permission.agent_id;
    const group = groups.get(key) ?? [];
    group.push(permission);
    groups.set(key, group);
  }
  return { byAgent, byAgentRound };
}

function permissionsForEntry(
  entry: RoomAgentRoundEntry,
  entriesByAgent: Map<string, RoomAgentRoundEntry[]>,
  groups: PermissionGroups,
): PendingPermission[] {
  const exact = entry.agent_round_id
    ? groups.byAgentRound.get(entry.agent_round_id) ?? []
    : [];
  const agentEntries = entriesByAgent.get(entry.agent_id) ?? [];
  const legacyTarget = agentEntries
    .filter((candidate) => isAgentRoundActive(candidate.status))
    .at(-1) ?? agentEntries.at(-1);
  const legacy = legacyTarget?.entry_id === entry.entry_id
    ? groups.byAgent.get(entry.agent_id) ?? []
    : [];
  return [...exact, ...legacy];
}

function groupEntriesByAgent(
  entries: RoomAgentRoundEntry[],
): Map<string, RoomAgentRoundEntry[]> {
  const groups = new Map<string, RoomAgentRoundEntry[]>();
  for (const entry of entries) {
    const group = groups.get(entry.agent_id) ?? [];
    group.push(entry);
    groups.set(entry.agent_id, group);
  }
  return groups;
}

function resolveGuidedTargetEntry(
  entries: RoomAgentRoundEntry[],
  message: UserMessage,
): RoomAgentRoundEntry | null {
  const agentRoundId = message.agent_round_id?.trim();
  if (agentRoundId) {
    return entries.find(
      (entry) => entry.agent_round_id === agentRoundId,
    ) ?? null;
  }
  const active = entries.filter((entry) => isAgentRoundActive(entry.status));
  return active.at(-1)
    ?? entries.find((entry) => entry.timestamp >= message.timestamp)
    ?? entries.at(-1)
    ?? null;
}

function compareAgentCards(
  left: GroupRoundAgentCardModel,
  right: GroupRoundAgentCardModel,
): number {
  const leftActive = isAgentRoundActive(left.status);
  const rightActive = isAgentRoundActive(right.status);
  if (leftActive !== rightActive) {
    return leftActive ? 1 : -1;
  }
  if (!leftActive) {
    return left.timestamp - right.timestamp
      || left.display_order - right.display_order
      || left.entry_id.localeCompare(right.entry_id);
  }
  return left.timestamp - right.timestamp
    || (left.pending_slot?.index ?? Number.MAX_SAFE_INTEGER)
      - (right.pending_slot?.index ?? Number.MAX_SAFE_INTEGER)
    || left.entry_id.localeCompare(right.entry_id);
}

function isVisibleUserMessage(message: Message): message is UserMessage {
  return message.role === "user" && !isAutomationTriggerUserMessage(message);
}
