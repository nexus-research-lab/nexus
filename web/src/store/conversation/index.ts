import { useMemo } from "react";

import { useSessionStore } from "@/store/session";
import { Conversation } from "@/types/conversation";

export interface ConversationStoreState {
  conversations: Conversation[];
  current_conversation_id: string | null;
  loading: boolean;
  error: string | null;
  createConversation: ReturnType<typeof useSessionStore.getState>["createSession"];
  deleteConversation: ReturnType<typeof useSessionStore.getState>["deleteSession"];
  updateConversation: ReturnType<typeof useSessionStore.getState>["updateSession"];
  setCurrentConversation: ReturnType<typeof useSessionStore.getState>["setCurrentSession"];
  syncConversationSnapshot: ReturnType<typeof useSessionStore.getState>["syncSessionSnapshot"];
  getConversation: ReturnType<typeof useSessionStore.getState>["getSession"];
  loadConversationsFromServer: ReturnType<typeof useSessionStore.getState>["loadSessionsFromServer"];
  clearAllConversations: ReturnType<typeof useSessionStore.getState>["clearAllSessions"];
}

function mapConversationStoreState(
  state: ReturnType<typeof useSessionStore.getState>,
): ConversationStoreState {
  return {
    conversations: state.sessions,
    current_conversation_id: state.current_session_key,
    loading: state.loading,
    error: state.error,
    createConversation: state.createSession,
    deleteConversation: state.deleteSession,
    updateConversation: state.updateSession,
    setCurrentConversation: state.setCurrentSession,
    syncConversationSnapshot: state.syncSessionSnapshot,
    getConversation: state.getSession,
    loadConversationsFromServer: state.loadSessionsFromServer,
    clearAllConversations: state.clearAllSessions,
  };
}

export function useConversationStore(): ConversationStoreState {
  const sessionStore = useSessionStore();
  return useMemo(() => mapConversationStoreState(sessionStore), [sessionStore]);
}

export function getConversationStoreSnapshot(): ConversationStoreState {
  return mapConversationStoreState(useSessionStore.getState());
}
