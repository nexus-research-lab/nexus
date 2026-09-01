/**
 * INPUT: Conversation 元数据动作、owner scope reset 与浏览器持久化。
 * OUTPUT: 当前 owner 的会话目录和可废弃旧读取的 scope revision。
 * POS: Conversation 元数据 Store；不持有聊天正文或服务端身份真相。
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";

import { createBrowserJsonStorage } from "@/lib/storage/browser-storage";
import { ConversationStoreState } from "@/types/conversation/conversation";

import * as actions from "./actions";

interface PersistedConversationStoreState {
  conversations?: ConversationStoreState["conversations"];
}

let ownerScopeRevision = 0;

export const useConversationStore = create<ConversationStoreState>()(
  persist(
    (set) => ({
      conversations: [],
      loading: false,
      error: null,

      sync_conversation_snapshot: actions.syncConversationSnapshotAction(set),
      load_conversations_from_server: actions.loadConversationsFromServerAction(
        set,
        () => ownerScopeRevision,
      ),
      clear_all_conversations: actions.clearAllConversationsAction(set),
    }),
    {
      name: "agent-ui-conversations",
      storage: createBrowserJsonStorage(),
      version: 4,
      migrate: (persistedState: unknown): PersistedConversationStoreState => {
        const state = (persistedState ?? {}) as PersistedConversationStoreState;
        return {
          conversations: Array.isArray(state.conversations) ? state.conversations : [],
        };
      },
      partialize: (state) => ({
        conversations: state.conversations,
      }),
    },
  ),
);

/** 身份边界推进时清空本地目录，并让旧 owner 的迟到读取失去提交资格。 */
export function resetConversationOwnerScope(): void {
  ownerScopeRevision += 1;
  useConversationStore.setState({
    conversations: [],
    error: null,
    loading: false,
  });
}
