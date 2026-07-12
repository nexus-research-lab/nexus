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
import {
  buildRoomAgentRoundEntries,
  extractAgentPreviewText,
  isAgentRoundActive,
  type AgentRoundStatus,
  type RoomAgentRoundEntry,
} from "../round/round-agent-model";

export interface GroupRoundAgentCardModel extends RoomAgentRoundEntry {
  agentAvatar: string | null;
  agentName: string;
  pendingPermissions: PendingPermission[];
  stopMessageId: string | null;
}

export interface GroupRoundCardModel {
  completedEntries: GroupRoundAgentCardModel[];
  pendingEntries: GroupRoundAgentCardModel[];
  userMessage: UserMessage | null;
  userWorkspaceAgentId: string | null;
}

export type AgentStatusSummaryTone =
  | "default"
  | "error"
  | "stopped"
  | "waiting";

interface GroupAgentStatusLabels {
  failed: string;
  preparing: string;
  replying: string;
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
  pendingSlot?: RoomPendingAgentSlotState;
  resultSummary?: ResultSummary;
  status: AgentRoundStatus;
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
  agentAvatarMap?: Record<string, string | null>;
  agentNameMap?: Record<string, string>;
  messages: Message[];
  pendingPermissions: PendingPermission[];
  pendingSlots: RoomPendingAgentSlotState[];
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
    fallbackLabel: "preparing",
    renderPreview: true,
    summaryOrder: ["preview", "fallback"],
    tone: "default",
  },
  streaming: {
    fallbackLabel: "replying",
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
  const permissionGroups = buildPermissionGroups(pendingPermissions);
  const completedEntries: GroupRoundAgentCardModel[] = [];
  const pendingEntries: GroupRoundAgentCardModel[] = [];

  for (const entry of buildRoomAgentRoundEntries(messages, pendingSlots)) {
    const card = buildAgentCard(
      entry,
      agentAvatarMap,
      agentNameMap,
      permissionGroups,
    );
    (entry.status === "done" ? completedEntries : pendingEntries).push(card);
  }
  completedEntries.sort((left, right) => left.timestamp - right.timestamp);

  const userMessage = messages.find(isVisibleUserMessage) ?? null;
  return {
    completedEntries,
    pendingEntries,
    userMessage,
    userWorkspaceAgentId: resolveUserWorkspaceAgentId(userMessage),
  };
}

export function buildGroupAgentStatusModel({
  labels,
  messages,
  pendingPermissions,
  pendingSlot,
  resultSummary,
  status,
}: BuildGroupAgentStatusModelOptions): GroupAgentStatusModel {
  const preview = extractAgentPreviewText(messages);
  const isActive = isAgentRoundActive(status);
  const permission = buildAgentPermissionState(pendingPermissions, isActive);
  const lastMessage = messages[messages.length - 1];
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
    model: lastMessageModel(lastMessage),
    preview,
    primaryPendingPermission: permission.primary,
    shouldRenderMarkdownSummary: summary.shouldRenderMarkdown,
    summaryText: summary.text,
    summaryTone: summary.tone,
    timestamp: resolveAgentTimestamp(lastMessage, resultSummary, pendingSlot),
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

function lastMessageModel(message?: AssistantMessage): string | null {
  return message?.model ?? null;
}

function resultSummaryText(summary?: ResultSummary): string | undefined {
  return summary?.result;
}

function resolveAgentTimestamp(
  lastMessage?: AssistantMessage,
  resultSummary?: ResultSummary,
  pendingSlot?: RoomPendingAgentSlotState,
): number {
  return firstDefinedNumber([
    lastMessage?.timestamp,
    resultSummary?.timestamp,
    pendingSlot?.timestamp,
  ]);
}

function firstDefinedNumber(values: Array<number | undefined>): number {
  return values.find((value) => value !== undefined) ?? 0;
}

function buildAgentCard(
  entry: RoomAgentRoundEntry,
  agentAvatarMap: Record<string, string | null> | undefined,
  agentNameMap: Record<string, string> | undefined,
  permissionGroups: Map<string, PendingPermission[]>,
): GroupRoundAgentCardModel {
  return {
    ...entry,
    agentAvatar: resolveAgentAvatar(agentAvatarMap, entry.agent_id),
    agentName: resolveAgentName(agentNameMap, entry.agent_id),
    pendingPermissions: permissionGroups.get(entry.agent_id) ?? [],
    stopMessageId: resolveStopMessageId(entry),
  };
}

function resolveAgentAvatar(
  avatarMap: Record<string, string | null> | undefined,
  agentId: string,
): string | null {
  return avatarMap?.[agentId] ?? null;
}

function resolveAgentName(
  nameMap: Record<string, string> | undefined,
  agentId: string,
): string {
  return nameMap?.[agentId] ?? agentId;
}

function resolveStopMessageId(entry: RoomAgentRoundEntry): string | null {
  if (!entry.pending_slot || !isAgentRoundActive(entry.status)) {
    return null;
  }
  return entry.pending_slot.agent_round_id;
}

function resolveUserWorkspaceAgentId(
  userMessage: UserMessage | null,
): string | null {
  return userMessage?.attachments?.[0]?.workspace_agent_id ?? null;
}

function buildPermissionGroups(
  permissions: PendingPermission[],
): Map<string, PendingPermission[]> {
  const groups = new Map<string, PendingPermission[]>();
  for (const permission of permissions) {
    if (!permission.agent_id) {
      continue;
    }
    const group = groups.get(permission.agent_id) ?? [];
    group.push(permission);
    groups.set(permission.agent_id, group);
  }
  return groups;
}

function isVisibleUserMessage(message: Message): message is UserMessage {
  return message.role === "user" && !isAutomationTriggerUserMessage(message);
}
