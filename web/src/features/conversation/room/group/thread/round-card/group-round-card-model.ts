/**
 * INPUT: Room 根轮次内的 user / assistant 消息、slot、权限与 execution 首见状态。
 * OUTPUT: root-global user 与按精确 agent_round_id/稳定展示槽排列的唯一 entries，保留人工介入和停止目标。
 * POS: Group round feed 的唯一结构归组入口；姓名/头像/语言由显示层绑定。
 */
import { isAutomationTriggerUserMessage } from "@/types/conversation/automation-message";
import type {
  Message,
  UserMessage,
} from "@/types/conversation/message/entity";
import type {
  RoomAgentExecutionState,
  RoomPendingAgentSlotState,
} from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import {
  filterPendingPermissionsForTerminalRoomExecutions,
} from "@/lib/conversation/pending-permission-match";

import {
  buildRoomAgentRoundEntries,
  isAgentRoundActive,
  type RoomAgentRoundEntry,
} from "../../round/round-agent-model";

export interface GroupRoundUserMessageModel {
  message: UserMessage;
  workspaceAgentId: string | null;
}

export interface GroupRoundAgentCardModel extends RoomAgentRoundEntry {
  guidedUserMessages: GroupRoundUserMessageModel[];
  pendingPermissions: PendingPermission[];
  stopAgentRoundId: string | null;
}

export interface GroupRoundCardModel {
  entries: GroupRoundAgentCardModel[];
  userMessages: GroupRoundUserMessageModel[];
}

interface BuildGroupRoundCardModelOptions {
  executionStates?: RoomAgentExecutionState[];
  messages: Message[];
  pendingPermissions: PendingPermission[];
  pendingSlots: RoomPendingAgentSlotState[];
}

interface PermissionGroups {
  byAgent: Map<string, PendingPermission[]>;
  byAgentRound: Map<string, PendingPermission[]>;
}

export function buildGroupRoundCardModel({
  executionStates = [],
  messages,
  pendingPermissions,
  pendingSlots,
}: BuildGroupRoundCardModelOptions): GroupRoundCardModel {
  const visiblePendingPermissions =
    filterPendingPermissionsForTerminalRoomExecutions(
      pendingPermissions,
      executionStates,
    );
  const entries = buildRoomAgentRoundEntries(
    messages,
    pendingSlots,
    visiblePendingPermissions,
    executionStates,
  );
  const permissionGroups = buildPermissionGroups(visiblePendingPermissions);
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
    const targetEntry = resolveGuidedTargetEntryForMessage(
      entriesByAgent,
      entries,
      message,
    );
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
    permissionsForEntry(entry, entriesByAgent, permissionGroups),
    guidedUserMessagesByEntry.get(entry.entry_id) ?? [],
  )).sort(compareAgentCards);

  return {
    entries: cards,
    userMessages,
  };
}

function buildAgentCard(
  entry: RoomAgentRoundEntry,
  pendingPermissions: PendingPermission[],
  guidedUserMessages: GroupRoundUserMessageModel[],
): GroupRoundAgentCardModel {
  return {
    ...entry,
    guidedUserMessages,
    pendingPermissions,
    stopAgentRoundId: resolveStopAgentRoundId(entry),
  };
}

function resolveStopAgentRoundId(entry: RoomAgentRoundEntry): string | null {
  if (!isAgentRoundActive(entry.status)) {
    return null;
  }
  return entry.agent_round_id?.trim() || null;
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
  const targets = normalizedTargetAgentIds(message);
  return targets.length === 1 ? targets[0] : null;
}

function normalizedTargetAgentIds(message: UserMessage): string[] {
  if (!Array.isArray(message.target_agent_ids)) {
    return [];
  }
  return Array.from(new Set(
    message.target_agent_ids
      .filter((value): value is string => typeof value === "string")
      .map((value) => value.trim())
      .filter(Boolean),
  ));
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

function resolveGuidedTargetEntryForMessage(
  entriesByAgent: Map<string, RoomAgentRoundEntry[]>,
  entries: RoomAgentRoundEntry[],
  message: UserMessage,
): RoomAgentRoundEntry | null {
  const targetAgentId = resolveGuidedTargetAgentId(message);
  if (targetAgentId) {
    return resolveGuidedTargetEntry(
      entriesByAgent.get(targetAgentId) ?? [],
      message,
    );
  }

  // 旧协议的重挂引导没有 target_agent_ids；只有当前根轮次唯一对应一个执行卡片时才能安全归组。
  if (
    message.delivery_policy === "guide"
    && message.source_round_id?.trim()
    && entries.length === 1
  ) {
    return resolveGuidedTargetEntry(entries, message);
  }
  return null;
}

function compareAgentCards(
  left: GroupRoundAgentCardModel,
  right: GroupRoundAgentCardModel,
): number {
  return left.display_order - right.display_order
    || (left.pending_slot?.index ?? Number.MAX_SAFE_INTEGER)
      - (right.pending_slot?.index ?? Number.MAX_SAFE_INTEGER)
    || left.timestamp - right.timestamp
    || left.entry_id.localeCompare(right.entry_id);
}

function isVisibleUserMessage(message: Message): message is UserMessage {
  return message.role === "user" && !isAutomationTriggerUserMessage(message);
}
