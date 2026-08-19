/**
 * INPUT: 当前 Session 身份与消息集合 setter。
 * OUTPUT: 分页 cursor、加载状态和按 Session 重置的请求取消所有权。
 * POS: React 层历史分页生命周期协调器。
 */
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
} from "./conversation-history";
import type { AgentConversationHistoryCursor } from "./conversation-history-model";

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
  const [resolvedHistoryRoundIds, setResolvedHistoryRoundIds] = useState<string[]>([]);
  const isHistoryLoadingRef = useRef(false);
  const isRoundWindowLoadingRef = useRef(false);
  const hasMoreHistoryRef = useRef(false);
  const requestControllersRef = useRef<Set<AbortController>>(new Set());
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

  const markHistoryRoundResolved = useCallback((roundId: string) => {
    const normalized = roundId.trim();
    if (!normalized) {
      return;
    }
    setResolvedHistoryRoundIds((currentRoundIds) => (
      currentRoundIds.includes(normalized)
        ? currentRoundIds
        : [...currentRoundIds, normalized].slice(-1_024)
    ));
  }, []);

  const resetHistoryPagination = useCallback(() => {
    requestControllersRef.current.forEach((controller) => controller.abort());
    requestControllersRef.current.clear();
    historyCursorRef.current = {
      before_round_id: null,
      before_round_timestamp: null,
    };
    setHistoryLoading(false);
    setHasMoreHistory(false);
    setHistoryPrependToken(0);
    setResolvedHistoryRoundIds([]);
  }, [setHasMoreHistory, setHistoryLoading]);

  const loadOlderMessages = useCallback(async (): Promise<boolean> => {
    const controller = new AbortController();
    requestControllersRef.current.add(controller);
    try {
      return await loadOlderAgentConversationMessages({
        activeSessionKeyRef,
        hasMoreHistoryRef,
        historyCursorRef,
        identity,
        isHistoryLoadingRef,
        setError,
        setHasMoreHistory,
        setHistoryLoading,
        setHistoryPrependToken,
        setMessages,
        signal: controller.signal,
      });
    } finally {
      requestControllersRef.current.delete(controller);
    }
  }, [
    activeSessionKeyRef,
    identity,
    setError,
    setHasMoreHistory,
    setHistoryLoading,
    setMessages,
  ]);

  const loadRoundWindow = useCallback(async (roundId: string): Promise<boolean> => {
    const controller = new AbortController();
    requestControllersRef.current.add(controller);
    try {
      return await loadAgentConversationMessagesAroundRound({
        activeSessionKeyRef,
        historyCursorRef,
        identity,
        isRoundWindowLoadingRef,
        onRoundResolved: markHistoryRoundResolved,
        roundId,
        setError,
        setHasMoreHistory,
        setMessages,
        signal: controller.signal,
      });
    } finally {
      requestControllersRef.current.delete(controller);
    }
  }, [
    activeSessionKeyRef,
    identity,
    markHistoryRoundResolved,
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
    resolvedHistoryRoundIds,
    resetHistoryPagination,
    setHasMoreHistory,
  };
}
