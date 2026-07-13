import { isMainAgent } from "@/config/runtime-options";
import { buildExternalSessionConversationId } from "@/lib/conversation/external-session";
import type { Agent, AgentSession } from "@/types/agent/agent";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type { RoomConversationView } from "@/types/conversation/conversation";
import type {
  RoomContextAggregate,
  RoomRecord,
  RoomSessionRecord,
} from "@/types/conversation/room";

import {
  buildRoomConversationViews,
  resolveCurrentRoomContext,
  resolveSelectedConversationId,
} from "../room-conversation-model";
import { resolveRoomMemberAgents } from "../room-member-model";
import { resolveCurrentAgentSessionIdentity } from "../room-session-model";

export interface RoomPageBaseModel {
  activeRoomSession: RoomSessionRecord | null;
  availableRoomAgents: Agent[];
  baseRoomConversations: RoomConversationView[];
  currentAgent: Agent | null;
  currentRoom: RoomRecord | null;
  currentRoomContext: RoomContextAggregate | null;
  roomMemberAgents: Agent[];
  selectedBaseConversationId: string | null;
  workspaceAgentIds: string[];
}

export interface RoomPageModel {
  agent: {
    current: Agent | null;
    sessionIdentity: AgentConversationIdentity | null;
    workspaceIds: string[];
  };
  conversation: {
    activeSession: RoomSessionRecord | null;
    current: RoomConversationView | null;
    currentContext: RoomContextAggregate | null;
    items: RoomConversationView[];
    selectedId: string | null;
  };
  room: {
    availableAgents: Agent[];
    current: RoomRecord | null;
    members: Agent[];
    routeId: string | null;
    skillNames: string[];
    title: string;
    type: string;
  };
}

interface BuildRoomPageBaseModelOptions {
  agents: Agent[];
  conversationId?: string | null;
  roomContexts: RoomContextAggregate[];
  roomId?: string | null;
}

interface BuildRoomPageModelOptions {
  base: RoomPageBaseModel;
  externalAgentSessions: AgentSession[];
  externalRoomConversations: RoomConversationView[];
  routeRoomId: string | null;
  routeSessionKey: string | null;
}

function getCurrentRoom(
  roomContexts: RoomContextAggregate[],
): RoomRecord | null {
  return roomContexts[0]?.room ?? null;
}

function getActiveRoomSession(
  currentRoomContext: RoomContextAggregate | null,
): RoomSessionRecord | null {
  return currentRoomContext?.sessions[0] ?? null;
}

function resolveCurrentAgent(
  roomMemberAgents: Agent[],
  activeRoomSession: RoomSessionRecord | null,
): Agent | null {
  const activeAgentId = activeRoomSession?.agent_id;
  return roomMemberAgents.find(
    (agent) => agent.agent_id === activeAgentId,
  ) ?? null;
}

function resolveAvailableRoomAgents(
  agents: Agent[],
  roomMemberAgents: Agent[],
): Agent[] {
  const joinedAgentIds = new Set(
    roomMemberAgents.map((agent) => agent.agent_id),
  );
  return agents.filter(
    (agent) => !joinedAgentIds.has(agent.agent_id) && !isMainAgent(agent.agent_id),
  );
}

export function buildRoomPageBaseModel({
  agents,
  conversationId,
  roomContexts,
  roomId,
}: BuildRoomPageBaseModelOptions): RoomPageBaseModel {
  const scopedRoomContexts = roomContexts.filter(
    (context) => context.room.id === roomId,
  );
  const roomMemberAgents = resolveRoomMemberAgents(scopedRoomContexts);
  const baseRoomConversations = buildRoomConversationViews(scopedRoomContexts);
  const selectedBaseConversationId = resolveSelectedConversationId(
    conversationId,
    baseRoomConversations,
  );
  const currentRoomContext = resolveCurrentRoomContext(
    scopedRoomContexts,
    selectedBaseConversationId,
  );
  const activeRoomSession = getActiveRoomSession(currentRoomContext);
  return {
    activeRoomSession,
    availableRoomAgents: resolveAvailableRoomAgents(agents, roomMemberAgents),
    baseRoomConversations,
    currentAgent: resolveCurrentAgent(roomMemberAgents, activeRoomSession),
    currentRoom: getCurrentRoom(scopedRoomContexts),
    currentRoomContext,
    roomMemberAgents,
    selectedBaseConversationId,
    workspaceAgentIds: roomMemberAgents.map((agent) => agent.agent_id),
  };
}

function resolveExternalSessionIdentity(
  routeSessionKey: string,
  currentAgent: Agent,
  externalAgentSessions: AgentSession[],
): AgentConversationIdentity {
  const externalSession = externalAgentSessions.find(
    (session) => session.session_key === routeSessionKey,
  );
  return {
    session_key: routeSessionKey,
    agent_id: externalSession?.agent_id ?? currentAgent.agent_id,
    chat_type: externalSession?.chat_type === "group" ? "group" : "dm",
  };
}

function getBaseRoomId(base: RoomPageBaseModel): string | null {
  return base.currentRoom?.id ?? null;
}

function getBaseConversationId(base: RoomPageBaseModel): string | null {
  return base.currentRoomContext?.conversation.id ?? null;
}

function getBaseRoomType(base: RoomPageBaseModel): string {
  return base.currentRoom?.room_type ?? "dm";
}

function resolveRoomPageSessionIdentity(
  base: RoomPageBaseModel,
  routeSessionKey: string | null,
  externalAgentSessions: AgentSession[],
): AgentConversationIdentity | null {
  if (routeSessionKey && base.currentAgent) {
    return resolveExternalSessionIdentity(
      routeSessionKey,
      base.currentAgent,
      externalAgentSessions,
    );
  }
  return resolveCurrentAgentSessionIdentity({
    currentRoomId: getBaseRoomId(base),
    currentConversationId: getBaseConversationId(base),
    activeRoomSession: base.activeRoomSession,
    currentRoomType: getBaseRoomType(base),
  });
}

function mergeRoomConversations(
  baseConversations: RoomConversationView[],
  externalConversations: RoomConversationView[],
): RoomConversationView[] {
  return [...baseConversations, ...externalConversations]
    .sort((left, right) => right.last_activity_at - left.last_activity_at);
}

function resolveRouteConversationId(
  routeSessionKey: string | null,
  selectedBaseConversationId: string | null,
): string | null {
  return routeSessionKey
    ? buildExternalSessionConversationId(routeSessionKey)
    : selectedBaseConversationId;
}

function findCurrentConversation(
  conversations: RoomConversationView[],
  selectedConversationId: string | null,
): RoomConversationView | null {
  return conversations.find(
    (conversation) => conversation.conversation_id === selectedConversationId,
  ) ?? null;
}

function resolveRoomPageTitle(
  currentRoom: RoomRecord | null,
  currentAgent: Agent | null,
): string {
  const candidates = [currentRoom?.name, currentAgent?.name];
  for (const candidate of candidates) {
    const title = candidate?.trim();
    if (title) {
      return title;
    }
  }
  return "未命名 room";
}

export function buildRoomPageModel({
  base,
  externalAgentSessions,
  externalRoomConversations,
  routeRoomId,
  routeSessionKey,
}: BuildRoomPageModelOptions): RoomPageModel {
  const conversations = mergeRoomConversations(
    base.baseRoomConversations,
    externalRoomConversations,
  );
  const selectedConversationId = resolveRouteConversationId(
    routeSessionKey,
    base.selectedBaseConversationId,
  );
  return {
    agent: {
      current: base.currentAgent,
      sessionIdentity: resolveRoomPageSessionIdentity(
        base,
        routeSessionKey,
        externalAgentSessions,
      ),
      workspaceIds: base.workspaceAgentIds,
    },
    conversation: {
      activeSession: base.activeRoomSession,
      current: findCurrentConversation(conversations, selectedConversationId),
      currentContext: base.currentRoomContext,
      items: conversations,
      selectedId: selectedConversationId,
    },
    room: {
      availableAgents: base.availableRoomAgents,
      current: base.currentRoom,
      members: base.roomMemberAgents,
      routeId: routeRoomId,
      skillNames: base.currentRoom?.skill_names ?? [],
      title: resolveRoomPageTitle(base.currentRoom, base.currentAgent),
      type: base.currentRoom?.room_type ?? "room",
    },
  };
}
