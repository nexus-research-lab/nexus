import { useCallback, type Dispatch, type SetStateAction } from "react";

import { isExternalSessionChannel } from "@/features/conversation/external-session-labels";
import { notifyRoomDirectoryUpdated } from "@/lib/api/room-api";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type {
  ConversationSnapshotPayload,
  ConversationStoreState,
} from "@/types/conversation/conversation";
import type { RoomContextAggregate } from "@/types/conversation/room";

import { applyConversationSnapshotToRoomContexts } from "./room-page-controller-core";

interface UseRoomConversationSnapshotOptions {
  activeRoomSessionId: string | null;
  currentConversationId: string | null;
  currentIdentity: AgentConversationIdentity | null;
  setRoomContexts: Dispatch<SetStateAction<RoomContextAggregate[]>>;
  syncConversationSnapshot: ConversationStoreState["sync_conversation_snapshot"];
}

export function useRoomConversationSnapshot({
  activeRoomSessionId,
  currentConversationId,
  currentIdentity,
  setRoomContexts,
  syncConversationSnapshot,
}: UseRoomConversationSnapshotOptions) {
  return useCallback((snapshot: ConversationSnapshotPayload) => {
    const conversationId = "conversation_id" in snapshot
      ? snapshot.conversation_id ?? null
      : currentConversationId;
    const roomSessionId = "room_session_id" in snapshot
      ? snapshot.room_session_id ?? null
      : activeRoomSessionId;

    setRoomContexts((current) => applyConversationSnapshotToRoomContexts(current, {
      conversation_id: conversationId,
      room_session_id: roomSessionId,
      session_id: snapshot.session_id ?? null,
      last_activity_at: snapshot.last_activity_at,
    }));

    const sessionKey = "session_key" in snapshot
      ? snapshot.session_key
      : currentIdentity?.session_key ?? null;
    if (!sessionKey) {
      return;
    }

    syncConversationSnapshot(sessionKey, {
      ...(snapshot.last_activity_at ? {last_activity_at: snapshot.last_activity_at} : {}),
      session_id: snapshot.session_id,
    });
    if (isExternalSessionChannel(null, sessionKey)) {
      notifyRoomDirectoryUpdated();
    }
  }, [
    activeRoomSessionId,
    currentConversationId,
    currentIdentity?.session_key,
    setRoomContexts,
    syncConversationSnapshot,
  ]);
}
