import { useCallback, type Dispatch, type SetStateAction } from "react";

import { notifyRoomDirectoryUpdated } from "@/lib/conversation/room-directory-events";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type {
  ConversationSnapshotPayload,
  ConversationStoreState,
} from "@/types/conversation/conversation";
import type { RoomContextAggregate } from "@/types/conversation/room";

import {
  applyConversationSnapshotToRoomContexts,
  projectRoomConversationSnapshot,
} from "./room-snapshot-model";

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
    const projection = projectRoomConversationSnapshot(snapshot, {
      activeRoomSessionId,
      currentConversationId,
      currentSessionKey: currentIdentity?.session_key ?? null,
    });

    setRoomContexts((current) => applyConversationSnapshotToRoomContexts(
      current,
      projection.roomContextSnapshot,
    ));

    if (!projection.storeUpdate) {
      return;
    }

    syncConversationSnapshot(
      projection.storeUpdate.sessionKey,
      projection.storeUpdate.patch,
    );
    if (projection.shouldNotifyRoomDirectory) {
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
