/**
 * INPUT: 当前 Room/Conversation 身份、snapshot 与已完成清理的 auth owner generation。
 * OUTPUT: 仅向当前 owner 的 Room context、Conversation Store 和目录失效事件投影快照。
 * POS: Room 页面 snapshot 回写边界；旧 owner 回调不得越过同步身份栅栏。
 */
import { useCallback, useSyncExternalStore, type Dispatch, type SetStateAction } from "react";

import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
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
  const ownerScopeGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  return useCallback((snapshot: ConversationSnapshotPayload) => {
    if (!isAuthOwnerScopeGenerationCurrent(ownerScopeGeneration)) {
      return;
    }
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
    ownerScopeGeneration,
    setRoomContexts,
    syncConversationSnapshot,
  ]);
}
