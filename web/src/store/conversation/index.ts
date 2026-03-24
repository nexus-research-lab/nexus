import { create } from "zustand";
import { persist } from "zustand/middleware";

import { createBrowserJSONStorage } from "@/lib/browser-storage";
import { ConversationStoreState } from "@/types/conversation";

import * as actions from "./actions";

export const useConversationStore = create<ConversationStoreState>()(
  persist(
    (set, get) => ({
      conversations: [],
      current_conversation_id: null,
      loading: false,
      error: null,

      create_conversation: actions.createConversationAction(set),
      delete_conversation: actions.deleteConversationAction(set),
      update_conversation: actions.updateConversationAction(set),
      set_current_conversation: actions.setCurrentConversationAction(set),
      sync_conversation_snapshot: actions.syncConversationSnapshotAction(set),
      get_conversation: actions.getConversationAction(get),
      load_conversations_from_server: actions.loadConversationsFromServerAction(set),
      clear_all_conversations: actions.clearAllConversationsAction(set),
    }),
    {
      name: "agent-ui-conversations",
      storage: createBrowserJSONStorage(),
      partialize: (state) => ({
        conversations: state.conversations,
        current_conversation_id: state.current_conversation_id,
      }),
    },
  ),
);

export function getConversationStoreSnapshot(): ConversationStoreState {
  return useConversationStore.getState();
}

export { generateConversationKey, createDefaultConversation } from "./utils";
