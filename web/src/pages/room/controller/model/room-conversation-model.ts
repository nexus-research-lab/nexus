import { buildRoomAgentSessionKey, buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import type { RoomConversationView } from "@/types/conversation/conversation";
import type { RoomContextAggregate } from "@/types/conversation/room";

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

function getLatestSession(context: RoomContextAggregate) {
  return [...context.sessions].sort((left, right) => {
    const getLastActivity = (session: typeof left) => (
      toTimestamp(session.last_activity_at)
      || toTimestamp(session.updated_at)
      || toTimestamp(session.created_at)
    );
    return getLastActivity(right) - getLastActivity(left);
  })[0];
}

function getConversationSessionKey(
  context: RoomContextAggregate,
  latestSession: RoomContextAggregate["sessions"][number] | undefined,
): string {
  if (context.room.room_type === "dm" && latestSession?.agent_id) {
    return buildRoomAgentSessionKey(
      context.conversation.id,
      latestSession.agent_id,
      "dm",
    );
  }
  return buildRoomSharedSessionKey(context.conversation.id);
}

export function buildRoomConversationViews(
  roomContexts: RoomContextAggregate[],
): RoomConversationView[] {
  return roomContexts
    .filter((context) => Boolean(context.conversation.id))
    .map((context) => {
      const lastActivityAt = getContextLastActivityTimestamp(context);
      const latestSession = getLatestSession(context);
      return {
        session_key: getConversationSessionKey(context, latestSession),
        room_id: context.room.id,
        conversation_id: context.conversation.id,
        conversation_type: context.conversation.conversation_type,
        session_id: latestSession?.sdk_session_id ?? null,
        agent_id: latestSession?.agent_id,
        title: context.conversation.title?.trim() || context.room.name || "未命名对话",
        options: {},
        created_at: toTimestamp(context.conversation.created_at) || lastActivityAt,
        last_activity_at: lastActivityAt,
        is_active: latestSession?.status === "active",
        message_count: context.conversation.message_count ?? 0,
      } satisfies RoomConversationView;
    })
    .sort((left, right) => right.last_activity_at - left.last_activity_at);
}

export function resolveSelectedConversationId(
  routeConversationId: string | null | undefined,
  roomConversations: RoomConversationView[],
): string | null {
  const routeConversationExists = roomConversations.some(
    (conversation) => conversation.conversation_id === routeConversationId,
  );
  return routeConversationId && routeConversationExists
    ? routeConversationId
    : roomConversations[0]?.conversation_id ?? null;
}

export function resolveCurrentRoomContext(
  roomContexts: RoomContextAggregate[],
  selectedConversationId: string | null,
): RoomContextAggregate | null {
  return roomContexts.find(
    (context) => context.conversation.id === selectedConversationId,
  ) ?? roomContexts[0] ?? null;
}
