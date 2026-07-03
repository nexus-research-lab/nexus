import { create } from "zustand";
import { persist } from "zustand/middleware";

import { createBrowserJsonStorage } from "@/lib/storage/browser-storage";
import { ConversationStoreState } from "@/types/conversation/conversation";

import * as actions from "./actions";

interface PersistedConversationStoreState {
  conversations?: ConversationStoreState["conversations"];
}

export const useConversationStore = create<ConversationStoreState>()(
  persist(
    (set) => ({
      conversations: [],
      loading: false,
      error: null,

      sync_conversation_snapshot: actions.syncConversationSnapshotAction(set),
      load_conversations_from_server: actions.loadConversationsFromServerAction(set),
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
