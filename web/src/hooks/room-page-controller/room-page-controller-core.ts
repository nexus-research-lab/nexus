/**
 * =====================================================
 * @File   ：room-page-controller-core.ts
 * @Date   ：2026-04-08 11:42:07
 * @Author ：leemysw
 * 2026-04-08 11:42:07   Create
 * =====================================================
 */

import { buildRoomAgentSessionKey, buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import { Agent } from "@/types/agent/agent";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import { RoomConversationView } from "@/types/conversation/conversation";
import { RoomContextAggregate } from "@/types/conversation/room";

function toTimestamp(value?: string | null): number {
  if (!value) {
    return 0;
  }

  const parsed = new Date(value).getTime();
  return Number.isFinite(parsed) ? parsed : 0;
}

function getContextLastActivityTimestamp(context: RoomContextAggregate): number {
  const sessionTimestamps = context.sessions.map((session) => (
    Math.max(
      toTimestamp(session.last_activity_at),
      toTimestamp(session.updated_at),
      toTimestamp(session.created_at),
    )
  ));

  return Math.max(
    toTimestamp(context.conversation.last_activity_at),
    toTimestamp(context.conversation.updated_at),
    toTimestamp(context.conversation.created_at),
    0,
    ...sessionTimestamps,
  );
}

function getRoomConversationSessionKey(
  context: RoomContextAggregate,
  fallbackSession: RoomContextAggregate["sessions"][number] | undefined,
): string {
  if (context.room.room_type === "dm") {
    if (fallbackSession?.agent_id) {
      return buildRoomAgentSessionKey(
        context.conversation.id,
        fallbackSession.agent_id,
        "dm",
      );
    }
  }

  return buildRoomSharedSessionKey(context.conversation.id);
}

function buildFallbackRoomMemberAgent(
  agentId: string,
  roomContexts: RoomContextAggregate[],
): Agent {
  const primaryContext = roomContexts[0] ?? null;
  const isDm = primaryContext?.room.room_type === "dm";
  const fallbackName = (
    isDm
      ? primaryContext?.room.name?.trim() ||
        primaryContext?.conversation.title?.trim() ||
        agentId
      : agentId
  );

  return {
    agent_id: agentId,
    name: fallbackName,
    workspace_path: "",
    options: {},
    created_at: 0,
    status: "active",
    avatar: null,
    description: null,
    vibe_tags: [],
    skills_count: null,
  };
}

export function buildRoomConversationViews(
  roomContexts: RoomContextAggregate[],
): RoomConversationView[] {
  return roomContexts
    .filter((context) => Boolean(context.conversation.id))
    .map((context) => {
      const contextLastActivityAt = getContextLastActivityTimestamp(context);
      const fallbackSession = [...context.sessions].sort((left, right) => {
        const leftTimestamp = (
          toTimestamp(left.last_activity_at) ||
          toTimestamp(left.updated_at) ||
          toTimestamp(left.created_at)
        );
        const rightTimestamp = (
          toTimestamp(right.last_activity_at) ||
          toTimestamp(right.updated_at) ||
          toTimestamp(right.created_at)
        );
        return rightTimestamp - leftTimestamp;
      })[0];

      return {
        session_key: getRoomConversationSessionKey(
          context,
          fallbackSession,
        ),
        room_id: context.room.id,
        conversation_id: context.conversation.id,
        conversation_type: context.conversation.conversation_type,
        session_id: fallbackSession?.sdk_session_id ?? null,
        agent_id: fallbackSession?.agent_id,
        title: context.conversation.title?.trim() || context.room.name || "未命名对话",
        options: {},
        created_at: toTimestamp(context.conversation.created_at) || contextLastActivityAt,
        last_activity_at: contextLastActivityAt,
        is_active: fallbackSession?.status === "active",
        message_count: context.conversation.message_count ?? 0,
      } satisfies RoomConversationView;
    })
    .sort((left, right) => right.last_activity_at - left.last_activity_at);
}

export function resolveSelectedConversationId(
  routeConversationId: string | null | undefined,
  roomConversations: RoomConversationView[],
): string | null {
  if (
    routeConversationId &&
    roomConversations.some((conversation) => conversation.conversation_id === routeConversationId)
  ) {
    return routeConversationId;
  }

  return roomConversations[0]?.conversation_id ?? null;
}

export function resolveCurrentRoomContext(
  roomContexts: RoomContextAggregate[],
  selectedConversationId: string | null,
): RoomContextAggregate | null {
  return roomContexts.find((context) => context.conversation.id === selectedConversationId) ??
    roomContexts[0] ??
    null;
}

export function resolveSelectedMemberAgentId(
  currentRoomContext: RoomContextAggregate | null,
  currentSelectedMemberAgentId: string | null,
): string | null {
  const memberAgentIds =
    currentRoomContext?.sessions
      .map((session) => session.agent_id)
      .filter(Boolean) ?? [];

  if (!memberAgentIds.length) {
    return null;
  }

  if (
    currentSelectedMemberAgentId &&
    memberAgentIds.includes(currentSelectedMemberAgentId)
  ) {
    return currentSelectedMemberAgentId;
  }

  return memberAgentIds[0];
}

export function resolveCurrentAgentSessionIdentity(params: {
  currentRoomId: string | null;
  currentConversationId: string | null;
  activeRoomSession: RoomContextAggregate["sessions"][number] | null;
  currentRoomType: string;
}): AgentConversationIdentity | null {
  const {
    currentRoomId: currentRoomId,
    currentConversationId: currentConversationId,
    activeRoomSession: activeRoomSession,
    currentRoomType: currentRoomType,
  } = params;

  const resolvedAgentId = activeRoomSession?.agent_id ?? null;
  const resolvedConversationId = currentConversationId ?? activeRoomSession?.conversation_id ?? null;
  const resolvedRoomId = currentRoomId ?? null;
  const resolvedRoomSessionId = activeRoomSession?.id ?? null;

  let resolvedSessionKey: string | null = null;
  if (!resolvedSessionKey && resolvedConversationId) {
    resolvedSessionKey = (
      currentRoomType === "dm" && resolvedAgentId
        ? buildRoomAgentSessionKey(resolvedConversationId, resolvedAgentId, "dm")
        : buildRoomSharedSessionKey(resolvedConversationId)
    );
  }

  if (!resolvedSessionKey) {
    return null;
  }

  return {
    session_key: resolvedSessionKey,
    agent_id: resolvedAgentId,
    room_id: resolvedRoomId,
    conversation_id: resolvedConversationId,
    room_session_id: resolvedRoomSessionId,
    chat_type: currentRoomType === "dm" ? "dm" : "group",
  };
}

export function resolveRoomMemberAgents(roomContexts: RoomContextAggregate[]): Agent[] {
  const memberAgents = roomContexts[0]?.member_agents ?? [];
  if (memberAgents.length > 0) {
    return memberAgents;
  }

  const memberAgentIds =
    roomContexts[0]?.members
      .filter((member) => member.member_type === "agent")
      .map((member) => member.member_agent_id)
      .filter((memberAgentId): memberAgentId is string => Boolean(memberAgentId)) ?? [];

  return memberAgentIds.map((agentId) => (
    buildFallbackRoomMemberAgent(agentId, roomContexts)
  ));
}

export function applyConversationSnapshotToRoomContexts(
  contexts: RoomContextAggregate[],
  snapshot: {
    conversation_id: string | null;
    room_session_id: string | null;
    session_id?: string | null;
    last_activity_at?: number | string | null;
  },
): RoomContextAggregate[] {
  if (!snapshot.conversation_id) {
    return contexts;
  }

  const nextLastActivityAt = snapshot.last_activity_at
    ? new Date(snapshot.last_activity_at).toISOString()
    : undefined;
  let hasChanged = false;

  const nextContexts = contexts.map((context) => {
    if (context.conversation.id !== snapshot.conversation_id) {
      return context;
    }

    let contextChanged = false;
    const nextConversationLastActivityAt =
      nextLastActivityAt ?? context.conversation.last_activity_at;
    const nextConversationUpdatedAt =
      nextLastActivityAt ?? context.conversation.updated_at;
    const conversationChanged =
      context.conversation.last_activity_at !== nextConversationLastActivityAt ||
      context.conversation.updated_at !== nextConversationUpdatedAt;

    const nextSessions = context.sessions.map((session) => {
      if (!snapshot.room_session_id || session.id !== snapshot.room_session_id) {
        return session;
      }

      const nextSdkSessionId = snapshot.session_id ?? session.sdk_session_id;
      const nextSessionLastActivityAt =
        nextLastActivityAt ?? session.last_activity_at;
      const sessionChanged =
        session.sdk_session_id !== nextSdkSessionId ||
        session.last_activity_at !== nextSessionLastActivityAt;

      if (!sessionChanged) {
        return session;
      }

      hasChanged = true;
      contextChanged = true;
      return {
        ...session,
        sdk_session_id: nextSdkSessionId,
        last_activity_at: nextSessionLastActivityAt,
      };
    });

    if (!contextChanged && !conversationChanged) {
      return context;
    }

    hasChanged = true;
    return {
      ...context,
      conversation: {
        ...context.conversation,
        last_activity_at: nextConversationLastActivityAt,
        updated_at: nextConversationUpdatedAt,
      },
      sessions: nextSessions,
    };
  });

  return hasChanged ? nextContexts : contexts;
}
