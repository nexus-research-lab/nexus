import type { RoomContextAggregate } from "@/types/conversation/room";

interface RoomConversationSnapshot {
  conversation_id: string | null;
  room_session_id: string | null;
  session_id?: string | null;
  last_activity_at?: number | string | null;
}

export function applyConversationSnapshotToRoomContexts(
  contexts: RoomContextAggregate[],
  snapshot: RoomConversationSnapshot,
): RoomContextAggregate[] {
  if (!snapshot.conversation_id) {
    return contexts;
  }

  const lastActivityAt = snapshot.last_activity_at
    ? new Date(snapshot.last_activity_at).toISOString()
    : undefined;
  let hasChanged = false;

  const nextContexts = contexts.map((context) => {
    if (context.conversation.id !== snapshot.conversation_id) {
      return context;
    }

    const conversation = {
      ...context.conversation,
      last_activity_at: lastActivityAt ?? context.conversation.last_activity_at,
      updated_at: lastActivityAt ?? context.conversation.updated_at,
    };
    const sessions = context.sessions.map((session) => {
      if (!snapshot.room_session_id || session.id !== snapshot.room_session_id) {
        return session;
      }

      const nextSession = {
        ...session,
        sdk_session_id: snapshot.session_id ?? session.sdk_session_id,
        last_activity_at: lastActivityAt ?? session.last_activity_at,
      };
      if (
        nextSession.sdk_session_id === session.sdk_session_id
        && nextSession.last_activity_at === session.last_activity_at
      ) {
        return session;
      }

      hasChanged = true;
      return nextSession;
    });
    const conversationChanged = (
      conversation.last_activity_at !== context.conversation.last_activity_at
      || conversation.updated_at !== context.conversation.updated_at
    );
    if (!conversationChanged && sessions.every((session, index) => session === context.sessions[index])) {
      return context;
    }

    hasChanged = true;
    return {...context, conversation, sessions};
  });

  return hasChanged ? nextContexts : contexts;
}
