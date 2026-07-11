import { isAutomationTriggerUserMessage } from "@/types/conversation/automation-message";
import type {
  AssistantMessage,
  Message,
  ResultSummary,
  UserMessage,
} from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

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

export interface GroupAgentStatusLabels {
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

interface BuildGroupRoundCardModelOptions {
  agentAvatarMap?: Record<string, string | null>;
  agentNameMap?: Record<string, string>;
  messages: Message[];
  pendingPermissions: PendingPermission[];
  pendingSlots: RoomPendingAgentSlotState[];
}

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
    userWorkspaceAgentId: userMessage?.attachments?.[0]?.workspace_agent_id ?? null,
  };
}

export function buildGroupAgentStatusModel({
  labels,
  messages,
  pendingPermissions,
  pendingSlot,
  resultSummary,
  status,
}: {
  labels: GroupAgentStatusLabels;
  messages: AssistantMessage[];
  pendingPermissions: PendingPermission[];
  pendingSlot?: RoomPendingAgentSlotState;
  resultSummary?: ResultSummary;
  status: AgentRoundStatus;
}): GroupAgentStatusModel {
  const preview = extractAgentPreviewText(messages);
  const primaryPendingPermission = pendingPermissions[0];
  const isActive = isAgentRoundActive(status);
  const isWaitingPermission = pendingPermissions.length > 0 && isActive;
  const isQuestionPending = Boolean(
    primaryPendingPermission
    && (
      primaryPendingPermission.interaction_mode === "question"
      || primaryPendingPermission.tool_name === "AskUserQuestion"
    ),
  );
  const lastMessage = messages[messages.length - 1];
  const statusFallbacks: Partial<Record<AgentRoundStatus, string>> = {
    cancelled: labels.stopped,
    error: labels.failed,
    pending: labels.preparing,
    streaming: labels.replying,
  };

  return {
    isActive,
    isQuestionPending,
    isWaitingPermission,
    model: lastMessage?.model ?? null,
    preview,
    primaryPendingPermission,
    shouldRenderMarkdownSummary: Boolean(
      preview
      && !isWaitingPermission
      && status !== "cancelled"
      && status !== "error",
    ),
    summaryText: resolveSummaryText({
      isWaitingPermission,
      permissionSummary: primaryPendingPermission?.summary,
      preview,
      resultText: resultSummary?.result,
      status,
      statusFallbacks,
      waitingPermissionText: labels.waitingPermission,
    }),
    summaryTone: isWaitingPermission
      ? "waiting"
      : (SUMMARY_TONE_BY_STATUS[status] ?? "default"),
    timestamp:
      lastMessage?.timestamp
      ?? resultSummary?.timestamp
      ?? pendingSlot?.timestamp
      ?? 0,
  };
}

const SUMMARY_TONE_BY_STATUS: Partial<
  Record<AgentRoundStatus, AgentStatusSummaryTone>
> = {
  cancelled: "stopped",
  error: "error",
};

function resolveSummaryText({
  isWaitingPermission,
  permissionSummary,
  preview,
  resultText,
  status,
  statusFallbacks,
  waitingPermissionText,
}: {
  isWaitingPermission: boolean;
  permissionSummary?: string;
  preview: string;
  resultText?: string;
  status: AgentRoundStatus;
  statusFallbacks: Partial<Record<AgentRoundStatus, string>>;
  waitingPermissionText: string;
}): string {
  if (isWaitingPermission) {
    return permissionSummary || waitingPermissionText;
  }
  const normalizedResult = resultText?.trim() ?? "";
  if (status === "cancelled" || status === "error") {
    return normalizedResult || statusFallbacks[status] || "";
  }
  return preview || statusFallbacks[status] || "";
}

function buildAgentCard(
  entry: RoomAgentRoundEntry,
  agentAvatarMap: Record<string, string | null> | undefined,
  agentNameMap: Record<string, string> | undefined,
  permissionGroups: Map<string, PendingPermission[]>,
): GroupRoundAgentCardModel {
  return {
    ...entry,
    agentAvatar: agentAvatarMap?.[entry.agent_id] ?? null,
    agentName: agentNameMap?.[entry.agent_id] ?? entry.agent_id,
    pendingPermissions: permissionGroups.get(entry.agent_id) ?? [],
    stopMessageId:
      entry.pending_slot && isAgentRoundActive(entry.status)
        ? entry.pending_slot.agent_round_id
        : null,
  };
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
