import { create } from "zustand";
import { persist } from "zustand/middleware";

import { create_browser_json_storage } from "@/lib/browser-storage";
import { ConversationStoreState } from "@/types/conversation";

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

      sync_conversation_snapshot: actions.sync_conversation_snapshot_action(set),
      load_conversations_from_server: actions.load_conversations_from_server_action(set),
      clear_all_conversations: actions.clear_all_conversations_action(set),
    }),
    {
      name: "agent-ui-conversations",
      storage: create_browser_json_storage(),
      version: 4,
      migrate: (persisted_state: unknown): PersistedConversationStoreState => {
        const state = (persisted_state ?? {}) as PersistedConversationStoreState;
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

export function get_conversation_store_snapshot(): ConversationStoreState {
  return useConversationStore.getState();
}
