/**
 * INPUT: Automation 表单目标、当前语言、Agent/Room 目录与统一 Session 读模型。
 * OUTPUT: 严格按 DM/Room 身份隔离、无内部 ID 显示兜底的候选、资源状态和 Room 解析。
 * POS: 定时任务弹窗所有目标与 Session 候选的唯一纯投影入口。
 */
import {
  getExternalSessionChannelLabel,
  getExternalSessionDisplayLabel,
} from "@/lib/conversation/external-session";
import {
  buildRoomSharedSessionKey,
  parseSessionKey,
} from "@/lib/conversation/session-key";
import type { Agent, AgentSession } from "@/types/agent/agent";
import { buildAgentSelectionOptions } from "@/lib/agent-selection-options";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  RoomAggregate,
  RoomContextAggregate,
} from "@/types/conversation/room";

import type {
  TargetType,
  TaskDialogLabelOption,
  TaskDialogSessionOption,
  TaskFormDraft,
  TaskDestinationOption,
} from "../scheduled-task-dialog-types";
import { buildRoomSelectionOptions, distinguishSessionOptions } from "./task-dialog-selection-labels";
import type {
  DialogResource,
  DialogResourceStatus,
} from "./use-dialog-resource";

const OPEN_RESOURCE_KEY = "open";

export interface TaskDialogResourceKeys {
  allSessions: string | null;
  agentSessions: string | null;
  agents: string | null;
  roomContexts: string | null;
  rooms: string | null;
}

export interface TaskDialogSessionData {
  options: TaskDialogSessionOption[];
  status: DialogResourceStatus;
}

export interface TaskDialogRoomAgentData {
  defaultAgentId: string;
  options: TaskDialogLabelOption[];
}

interface TaskDialogSessionResources {
  agentSessions: DialogResource<AgentSession>;
  roomContexts: DialogResource<RoomContextAggregate>;
}

const SESSION_REQUEST_KEYS: Record<
  TargetType,
  (form: TaskFormDraft, isOpen: boolean) => Pick<
    TaskDialogResourceKeys,
    "agentSessions" | "roomContexts"
  >
> = {
  agent: (form, isOpen) => ({
    agentSessions: activeResourceKey(
      isOpen
        && form.executionKind === "agent"
        && form.executionMode === "existing",
      form.selectedAgentId,
    ),
    roomContexts: null,
  }),
  room: (form, isOpen) => ({
    agentSessions: null,
    roomContexts: activeResourceKey(
      isOpen && form.executionKind === "agent",
      form.selectedRoomId,
    ),
  }),
};

export function buildTaskDialogResourceKeys(
  form: TaskFormDraft,
  isOpen: boolean,
): TaskDialogResourceKeys {
  return {
    allSessions: isOpen && form.executionKind === "agent" ? OPEN_RESOURCE_KEY : null,
    agents: isOpen ? OPEN_RESOURCE_KEY : null,
    rooms: isOpen && form.executionKind === "agent" ? OPEN_RESOURCE_KEY : null,
    ...SESSION_REQUEST_KEYS[form.targetType](form, isOpen),
  };
}

export function buildRoomOptions(
  rooms: RoomAggregate[],
  t: I18nContextValue["t"],
): TaskDialogLabelOption[] {
  return buildRoomSelectionOptions(rooms.filter(isGroupRoom).map((item) => item.room), t);
}

export function buildExecutionRoomOptions(
  rooms: RoomAggregate[],
  t: I18nContextValue["t"],
): TaskDialogLabelOption[] {
  const eligibleIds = new Set(rooms.filter((room) => (
    isGroupRoom(room)
    && room.members.some((member) => (
      member.member_type === "agent" && !member.participation_paused
    ))
  )).map((item) => item.room.id));
  return buildRoomOptions(rooms, t).filter((option) => eligibleIds.has(option.value));
}

export function buildTaskDialogSessionData(
  targetType: TargetType,
  resources: TaskDialogSessionResources,
  t: I18nContextValue["t"],
): TaskDialogSessionData {
  const options = targetType === "room"
    ? buildRoomSessionOptions(resources.roomContexts.items, t)
    : buildAgentSessionOptions(resources.agentSessions.items, t);
  return {
    options: distinguishSessionOptions(options, t),
    status: targetType === "room" ? resourceStatus(resources.roomContexts) : resourceStatus(resources.agentSessions),
  };
}

export function buildTaskDialogDeliverySessionData(
  form: TaskFormDraft,
  sessions: DialogResource<AgentSession>,
  t: I18nContextValue["t"],
): TaskDialogSessionData {
  if (form.replyMode !== "selected") {
    return { options: [], status: resourceStatus(sessions) };
  }
  const options = form.deliveryTargetType === "room"
    ? buildDeliveryRoomOptions(
        sessions.items,
        form.selectedDeliveryRoomId,
        t,
      )
    : buildDeliveryAgentOptions(
        sessions.items,
        form.selectedDeliveryAgentId,
        t,
      );
  return { options: distinguishSessionOptions(options, t), status: resourceStatus(sessions) };
}

export function buildExecutionRoomAgentData(
  contexts: RoomContextAggregate[],
  selectedSessionKey: string,
  agents: Agent[],
  t: I18nContextValue["t"],
): TaskDialogRoomAgentData {
  const conversationId = roomConversationId(selectedSessionKey);
  const context = contexts.find((item) => (
    item.room.room_type === "room"
    && item.conversation.id === conversationId
  ));
  if (!context) {
    return { defaultAgentId: "", options: [] };
  }
  const availableAgentIds = new Set(context.sessions.map((session) => (
    session.agent_id.trim()
  )).filter(Boolean));
  const eligibleAgents = context.member_agents.filter((agent) => (
    availableAgentIds.has(agent.agent_id)
    && !agent.room_participation_paused
  ));
  const eligibleIds = new Set(eligibleAgents.map((agent) => agent.agent_id));
  const options = buildAgentSelectionOptions(context.member_agents, t, agents).filter((option) => eligibleIds.has(option.value));
  const defaultAgentId = context.room.host_agent_id?.trim() || "";
  return {
    defaultAgentId: options.some((option) => option.value === defaultAgentId)
      ? defaultAgentId
      : "",
    options,
  };
}

export function buildDeliveryRoomAgentData(
  sessions: AgentSession[],
  rooms: RoomAggregate[],
  roomId: string,
  selectedSessionKey: string,
  agents: Agent[],
  t: I18nContextValue["t"],
): TaskDialogRoomAgentData {
  const normalizedRoomId = roomId.trim();
  const conversationId = roomConversationId(selectedSessionKey);
  if (!normalizedRoomId || !conversationId) {
    return { defaultAgentId: "", options: [] };
  }
  const seen = new Set<string>();
  const candidates: Array<{ agent_id: string }> = [];
  sessions.filter((session) => (
    session.room_id === normalizedRoomId
    && session.conversation_id === conversationId
    && isRoomMemberSession(session)
  )).forEach((session) => {
    const agentId = session.agent_id.trim();
    if (!agentId || seen.has(agentId)) {
      return;
    }
    seen.add(agentId);
    candidates.push({
      agent_id: agentId,
    });
  });
  const options = buildAgentSelectionOptions(candidates, t, agents);
  const defaultAgentId = rooms.find((room) => (
    isGroupRoom(room) && room.room.id === normalizedRoomId
  ))?.room.host_agent_id?.trim() || "";
  return {
    defaultAgentId: options.some((option) => option.value === defaultAgentId)
      ? defaultAgentId
      : "",
    options,
  };
}

export function resolveTaskDialogRoomId(
  sessions: AgentSession[],
  sessionKey: string,
): string {
  const parsed = parseSessionKey(sessionKey.split("::executor:", 1)[0]);
  const conversationId = parsed.conversation_id || parsed.ref || "";
  if (!conversationId) {
    return "";
  }
  return sessions.find((session) => (
    isRoomMemberSession(session)
    &&
    session.conversation_id === conversationId
    && Boolean(session.room_id)
  ))?.room_id?.trim() || "";
}

function roomConversationId(sessionKey: string): string {
  const parsed = parseSessionKey(sessionKey);
  if (!parsed.is_structured || parsed.kind !== "room") {
    return "";
  }
  return parsed.conversation_id || parsed.ref || "";
}

export function resourceStatus<T>(
  resource: DialogResource<T>,
): DialogResourceStatus {
  return {
    error: resource.error,
    loading: resource.loading,
    retry: resource.retry,
  };
}

function activeResourceKey(
  active: boolean,
  selectedId: string,
): string | null {
  return active && selectedId ? selectedId : null;
}

function isGroupRoom(room: RoomAggregate): boolean {
  return room.room.room_type.trim().toLowerCase() === "room";
}

function normalizeSessionChatType(value: string | null): string {
  switch (value?.trim().toLowerCase()) {
    case "":
    case undefined:
    case "dm":
      return "dm";
    case "group":
    case "room":
      return "group";
    default:
      return value?.trim().toLowerCase() || "";
  }
}

function isAgentSessionOfChatType(
  session: AgentSession,
  expectedChatType: "dm" | "group",
): boolean {
  const parsed = parseSessionKey(session.session_key);
  if (!parsed.is_structured
    || parsed.kind !== "agent"
    || normalizeSessionChatType(parsed.chat_type) !== expectedChatType) {
    return false;
  }
  const storedChatType = session.chat_type.trim();
  return !storedChatType
    || normalizeSessionChatType(storedChatType) === expectedChatType;
}

function isAgentDMSession(session: AgentSession): boolean {
  return isAgentSessionOfChatType(session, "dm");
}

function isRoomMemberSession(session: AgentSession): boolean {
  return isAgentSessionOfChatType(session, "group");
}

function buildAgentSessionOptions(
  sessions: AgentSession[],
  t: I18nContextValue["t"],
): TaskDialogSessionOption[] {
  return sessions.filter((session) => {
    if (!isAgentDMSession(session)) {
      return false;
    }
    const externalChannel = getExternalSessionChannelLabel(
      session.channel_type,
      session.session_key,
    );
    return !externalChannel || session.external_identity?.current_pairing === true;
  }).map((session) => buildAgentSessionOption(session, t));
}

function buildAgentSessionOption(session: AgentSession, t: I18nContextValue["t"]): TaskDialogSessionOption {
  const channelLabel = getExternalSessionDisplayLabel(session.channel_type, session.session_key, session.external_identity);
  const title = session.title?.trim() || t("capability.scheduled_dialog_unnamed_session");
  return {
    badge: channelLabel ? `IM · ${channelLabel}` : null,
    label: title,
    sessionKey: session.session_key,
    value: session.session_key,
  };
}

function isAvailableDeliverySession(session: AgentSession): boolean {
  const externalChannel = getExternalSessionChannelLabel(
    session.channel_type,
    session.session_key,
  );
  return !externalChannel || session.external_identity?.current_pairing === true;
}

function isUserVisibleDeliverySession(session: AgentSession): boolean {
  const parsed = parseSessionKey(session.session_key);
  return parsed.channel !== "automation"
    && !(parsed.channel === "internal" && parsed.ref === "automation-inbox")
    && session.options.created_by !== "automation_delivery";
}

function buildDeliveryAgentOptions(
  sessions: AgentSession[],
  agentId: string,
  t: I18nContextValue["t"],
): TaskDialogSessionOption[] {
  const normalizedAgentId = agentId.trim();
  if (!normalizedAgentId) {
    return [];
  }
  const options: TaskDialogSessionOption[] = [];
  const seen = new Set<string>();
  sessions.filter((session) => (
    session.agent_id === normalizedAgentId
    && isAgentDMSession(session)
    && isUserVisibleDeliverySession(session)
    && isAvailableDeliverySession(session)
  )).forEach((session) => {
    if (seen.has(session.session_key)) {
      return;
    }
    seen.add(session.session_key);
    options.push(buildAgentSessionOption(session, t));
  });
  return options;
}

function buildDeliveryRoomOptions(
  sessions: AgentSession[],
  roomId: string,
  t: I18nContextValue["t"],
): TaskDialogSessionOption[] {
  const normalizedRoomId = roomId.trim();
  if (!normalizedRoomId) {
    return [];
  }
  const seen = new Set<string>();
  const options: TaskDialogSessionOption[] = [];
  sessions.filter((session) => (
    session.room_id === normalizedRoomId
    && isRoomMemberSession(session)
    && Boolean(session.conversation_id)
  )).forEach((session) => {
    const conversationId = session.conversation_id?.trim() || "";
    if (!conversationId || seen.has(conversationId)) {
      return;
    }
    seen.add(conversationId);
    const sharedSessionKey = buildRoomSharedSessionKey(conversationId);
    options.push({
      badge: "Room",
      label: session.title?.trim() || t("capability.scheduled_dialog_unnamed_session"),
      sessionKey: sharedSessionKey,
      value: sharedSessionKey,
    });
  });
  return options;
}

function buildRoomSessionOptions(
  contexts: RoomContextAggregate[],
  t: I18nContextValue["t"],
): TaskDialogSessionOption[] {
  return contexts.filter((context) => (
    context.room.room_type === "room"
    && context.conversation.id.trim()
    && context.sessions.length > 0
  )).map((context) => {
    const sessionKey = buildRoomSharedSessionKey(context.conversation.id);
    const conversationName = context.conversation.title?.trim()
      || t("capability.scheduled_dialog_unnamed_session");
    return {
      badge: "Room",
      label: conversationName,
      sessionKey,
      value: sessionKey,
    };
  });
}

// 复用既有可投递会话资格，按父对象分组；Room 仍使用共享会话身份。
export function buildTaskDestinations(
  sessions: AgentSession[],
  agents: TaskDialogLabelOption[],
  rooms: TaskDialogLabelOption[],
  t: I18nContextValue["t"],
): TaskDestinationOption[] {
  return [
    ...agents.flatMap((agent) => distinguishSessionOptions(buildDeliveryAgentOptions(sessions, agent.value, t), t)
      .map((option) => ({ ...option, group: agent.label, targetType: "agent" as const, agentId: agent.value, roomId: "" }))),
    ...rooms.flatMap((room) => distinguishSessionOptions(buildDeliveryRoomOptions(sessions, room.value, t), t)
      .map((option) => ({ ...option, group: room.label, targetType: "room" as const, agentId: "", roomId: room.value }))),
  ];
}

// 与创建时的权限快照保持相同优先级；资源未就绪时不猜测权限。
export function resolveTaskInheritedPermission(
  form: TaskFormDraft,
  agents: DialogResource<Agent>,
  agentSessions: DialogResource<AgentSession>,
  roomContexts: DialogResource<RoomContextAggregate>,
  defaultAgentId: string,
): string | null {
  if (agents.loading || agents.error) return null;
  const agent = agents.items.find((item) => item.agent_id === (form.selectedAgentId || defaultAgentId));
  if (!agent) return null;
  let sessionMode: unknown;
  if (form.targetType === "room") {
    if (roomContexts.loading || roomContexts.error) return null;
    const context = roomContexts.items.find((item) => item.conversation.id === roomConversationId(form.selectedSessionKey));
    if (!context) return null;
    const sessions = context.sessions.filter((item) => item.agent_id === agent.agent_id);
    sessionMode = (sessions.find((item) => item.is_primary) ?? sessions[0])?.options.permission_mode;
  } else if (form.executionMode === "existing") {
    if (agentSessions.loading || agentSessions.error) return null;
    const session = agentSessions.items.find((item) => item.session_key === form.selectedSessionKey);
    if (!session) return null;
    sessionMode = session.options.permission_mode;
  }
  return (typeof sessionMode === "string" ? sessionMode.trim() : "") || agent.options.permission_mode?.trim() || "default";
}
