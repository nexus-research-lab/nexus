import {
  useCallback,
  useRef,
  useState,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from "react";

import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type { Message } from "@/types/conversation/message/entity";

import {
  loadAgentConversationMessagesAroundRound,
  loadOlderAgentConversationMessages,
  type AgentConversationHistoryCursor,
} from "./conversation-history";

interface UseAgentConversationHistoryParams {
  activeSessionKeyRef: RefObject<string | null>;
  identity: AgentConversationIdentity | null;
  setError: Dispatch<SetStateAction<string | null>>;
  setMessages: Dispatch<SetStateAction<Message[]>>;
}

/**
 * 管理会话历史的分页状态与加载互斥。
 * 主会话 Hook 只负责在身份切换时重置、在快照落地时更新 cursor。
 */
export function useAgentConversationHistory({
  activeSessionKeyRef,
  identity,
  setError,
  setMessages,
}: UseAgentConversationHistoryParams) {
  const [isHistoryLoading, setIsHistoryLoadingState] = useState(false);
  const [hasMoreHistory, setHasMoreHistoryState] = useState(false);
  const [historyPrependToken, setHistoryPrependToken] = useState(0);
  const isHistoryLoadingRef = useRef(false);
  const isRoundWindowLoadingRef = useRef(false);
  const hasMoreHistoryRef = useRef(false);
  const historyCursorRef = useRef<AgentConversationHistoryCursor>({
    before_round_id: null,
    before_round_timestamp: null,
  });

  const setHistoryLoading = useCallback((nextValue: boolean) => {
    isHistoryLoadingRef.current = nextValue;
    setIsHistoryLoadingState((currentValue) =>
      currentValue === nextValue ? currentValue : nextValue,
    );
  }, []);

  const setHasMoreHistory = useCallback((nextValue: boolean) => {
    hasMoreHistoryRef.current = nextValue;
    setHasMoreHistoryState((currentValue) =>
      currentValue === nextValue ? currentValue : nextValue,
    );
  }, []);

  const resetHistoryPagination = useCallback(() => {
    historyCursorRef.current = {
      before_round_id: null,
      before_round_timestamp: null,
    };
    setHistoryLoading(false);
    setHasMoreHistory(false);
    setHistoryPrependToken(0);
  }, [setHasMoreHistory, setHistoryLoading]);

  const loadOlderMessages = useCallback(async (): Promise<boolean> => {
    return loadOlderAgentConversationMessages({
      active_session_key_ref: activeSessionKeyRef,
      identity,
      history_cursor_ref: historyCursorRef,
      has_more_history_ref: hasMoreHistoryRef,
      is_history_loading_ref: isHistoryLoadingRef,
      set_history_loading: setHistoryLoading,
      set_has_more_history: setHasMoreHistory,
      set_history_prepend_token: setHistoryPrependToken,
      set_messages: setMessages,
      set_error: setError,
    });
  }, [
    activeSessionKeyRef,
    identity,
    setError,
    setHasMoreHistory,
    setHistoryLoading,
    setMessages,
  ]);

  const loadRoundWindow = useCallback(async (roundId: string): Promise<boolean> => {
    return loadAgentConversationMessagesAroundRound({
      active_session_key_ref: activeSessionKeyRef,
      identity,
      history_cursor_ref: historyCursorRef,
      is_round_window_loading_ref: isRoundWindowLoadingRef,
      round_id: roundId,
      set_has_more_history: setHasMoreHistory,
      set_messages: setMessages,
      set_error: setError,
    });
  }, [
    activeSessionKeyRef,
    identity,
    setError,
    setHasMoreHistory,
    setMessages,
  ]);

  return {
    hasMoreHistory,
    historyCursorRef,
    historyPrependToken,
    isHistoryLoading,
    loadOlderMessages,
    loadRoundWindow,
    resetHistoryPagination,
    setHasMoreHistory,
  };
}
