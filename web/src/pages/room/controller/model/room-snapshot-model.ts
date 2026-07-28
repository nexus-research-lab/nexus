/**
 * [INPUT]: 当前会话身份与消息观察器提交的活动快照。
 * [OUTPUT]: Room 上下文、Session Store 与目录通知的最小增量。
 * [POS]: Room 页面控制器的纯快照投影，不读取路由或执行网络请求。
 */

import { isExternalSessionChannel } from "@/lib/conversation/external-session";
import type {
  ConversationSnapshotPayload,
  ConversationStoreState,
} from "@/types/conversation/conversation";
import type { RoomContextAggregate } from "@/types/conversation/room";

interface RoomConversationSnapshot {
  conversation_id: string | null;
  room_session_id: string | null;
  session_id?: string | null;
  has_user_input?: boolean;
  last_activity_at?: number | string | null;
  message_count?: number;
}

interface RoomConversationSnapshotProjectionContext {
  activeRoomSessionId: string | null;
  currentConversationId: string | null;
  currentSessionKey: string | null;
}

interface ConversationStoreSnapshotUpdate {
  patch: Parameters<
    ConversationStoreState["sync_conversation_snapshot"]
  >[1];
  sessionKey: string;
}

export interface ProjectedRoomConversationSnapshot {
  roomContextSnapshot: RoomConversationSnapshot;
  shouldNotifyRoomDirectory: boolean;
  storeUpdate: ConversationStoreSnapshotUpdate | null;
}

export function projectRoomConversationSnapshot(
  snapshot: ConversationSnapshotPayload,
  context: RoomConversationSnapshotProjectionContext,
): ProjectedRoomConversationSnapshot {
  const conversationId = resolveSnapshotConversationId(
    snapshot,
    context.currentConversationId,
  );
  const roomSessionId = resolveSnapshotRoomSessionId(
    snapshot,
    context.activeRoomSessionId,
  );
  const sessionKey = resolveSnapshotSessionKey(
    snapshot,
    context.currentSessionKey,
  );
  const storeUpdate = buildConversationStoreSnapshotUpdate(
    snapshot,
    sessionKey,
  );

  return {
    roomContextSnapshot: {
      conversation_id: conversationId,
      has_user_input: snapshot.has_user_input,
      last_activity_at: snapshot.last_activity_at,
      message_count: snapshot.message_count,
      room_session_id: roomSessionId,
      session_id: snapshot.session_id ?? null,
    },
    shouldNotifyRoomDirectory: storeUpdate
      ? isExternalSessionChannel(null, storeUpdate.sessionKey)
      : false,
    storeUpdate,
  };
}

function resolveSnapshotConversationId(
  snapshot: ConversationSnapshotPayload,
  fallback: string | null,
): string | null {
  return "conversation_id" in snapshot
    ? snapshot.conversation_id ?? null
    : fallback;
}

function resolveSnapshotRoomSessionId(
  snapshot: ConversationSnapshotPayload,
  fallback: string | null,
): string | null {
  return "room_session_id" in snapshot
    ? snapshot.room_session_id ?? null
    : fallback;
}

function resolveSnapshotSessionKey(
  snapshot: ConversationSnapshotPayload,
  fallback: string | null,
): string | null {
  return "session_key" in snapshot ? snapshot.session_key : fallback;
}

function buildConversationStoreSnapshotUpdate(
  snapshot: ConversationSnapshotPayload,
  sessionKey: string | null,
): ConversationStoreSnapshotUpdate | null {
  if (!sessionKey) {
    return null;
  }
  return {
    patch: {
      ...(snapshot.last_activity_at
        ? {last_activity_at: snapshot.last_activity_at}
        : {}),
      session_id: snapshot.session_id,
    },
    sessionKey,
  };
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
      is_draft: snapshot.has_user_input === true
        ? false
        : context.conversation.is_draft,
      last_activity_at: lastActivityAt ?? context.conversation.last_activity_at,
      message_count: snapshot.message_count === undefined
        ? context.conversation.message_count
        : Math.max(context.conversation.message_count ?? 0, snapshot.message_count),
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
      conversation.is_draft !== context.conversation.is_draft
      || conversation.last_activity_at !== context.conversation.last_activity_at
      || conversation.message_count !== context.conversation.message_count
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
