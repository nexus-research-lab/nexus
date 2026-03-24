import { useMemo } from "react";

import { useSessionStore } from "@/store/session";
import { ConversationStoreState } from "@/types/conversation";

function mapConversationStoreState(
  state: ReturnType<typeof useSessionStore.getState>,
): ConversationStoreState {
  return {
    conversations: state.sessions,
    current_conversation_id: state.current_session_key,
    loading: state.loading,
    error: state.error,
    create_conversation: state.create_session,
    delete_conversation: state.delete_session,
    update_conversation: state.update_session,
    set_current_conversation: state.set_current_session,
    sync_conversation_snapshot: state.sync_session_snapshot,
    get_conversation: state.get_session,
    load_conversations_from_server: state.load_sessions_from_server,
    clear_all_conversations: state.clear_all_sessions,
  };
}

export function useConversationStore(): ConversationStoreState {
  const sessionStore = useSessionStore();
  return useMemo(() => mapConversationStoreState(sessionStore), [sessionStore]);
}

export function getConversationStoreSnapshot(): ConversationStoreState {
  return mapConversationStoreState(useSessionStore.getState());
}
