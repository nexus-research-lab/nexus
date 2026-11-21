/**
 * INPUT: 当前 Session 身份、驻留消息窗口及其更新入口。
 * OUTPUT: 分页 cursor、加载状态、连续窗口的四种读取命令和按 Session 重置的请求取消所有权。
 * POS: React 层历史分页生命周期协调器。
 */
import {
  useCallback,
  useRef,
  useState,
  type RefObject,
} from "react";

import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";

import type { LoadedMessageWindow } from "../message/message-window-model";
import type { MessageWindowUpdater } from "../message/use-agent-message-collection";
import {
  loadAgentConversationMessagesAroundRound,
  loadNewerAgentConversationMessages,
  loadOlderAgentConversationMessages,
  returnToLatestAgentConversationMessages,
} from "./conversation-history";
import type { AgentConversationHistoryCursor } from "./conversation-history-model";

interface UseAgentConversationHistoryParams {
  activeSessionKeyRef: RefObject<string | null>;
  identity: AgentConversationIdentity | null;
  updateMessageWindow: (updater: MessageWindowUpdater) => void;
  windowRef: RefObject<LoadedMessageWindow>;
}

/**
 * 管理会话历史的分页状态与加载互斥。
 * 主会话 Hook 只负责在身份切换时重置、在快照落地时更新 cursor。
 */
export function useAgentConversationHistory({
  activeSessionKeyRef,
  identity,
  updateMessageWindow,
  windowRef,
}: UseAgentConversationHistoryParams) {
  const [isHistoryLoading, setIsHistoryLoadingState] = useState(false);
  const [hasMoreHistory, setHasMoreHistoryState] = useState(false);
  const [historyStartRoundId, setHistoryStartRoundId] = useState<string | null>(null);
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

  const commitHistoryCursor = useCallback((
    cursor: AgentConversationHistoryCursor,
    hasMore: boolean,
  ) => {
    historyCursorRef.current = cursor;
    setHistoryStartRoundId(cursor.start_round_id ?? null);
    setHasMoreHistory(hasMore);
  }, [setHasMoreHistory]);

  /**
   * 空会话没有可分页的历史，首个驻留的 round 就是会话起点。记下它之后，
   * 即使该 round 日后被窗口淘汰，向前扩展也能重新读回。
   */
  const adoptHistoryStart = useCallback((roundId: string) => {
    const cursor = historyCursorRef.current;
    if (
      !cursor.start_round_id
      && cursor.before_round_id === null
      && !hasMoreHistoryRef.current
    ) {
      commitHistoryCursor({ ...cursor, start_round_id: roundId }, false);
    }
  }, [commitHistoryCursor]);

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
    commitHistoryCursor({
      before_round_id: null,
      before_round_timestamp: null,
    }, false);
    setHistoryLoading(false);
    setResolvedHistoryRoundIds([]);
  }, [commitHistoryCursor, setHistoryLoading]);

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
        commitHistoryCursor,
        setHistoryLoading,
        signal: controller.signal,
        updateMessageWindow,
        windowRef,
      });
    } finally {
      requestControllersRef.current.delete(controller);
    }
  }, [
    activeSessionKeyRef,
    commitHistoryCursor,
    identity,
    setHistoryLoading,
    updateMessageWindow,
    windowRef,
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
        commitHistoryCursor,
        signal: controller.signal,
        updateMessageWindow,
        windowRef,
      });
    } finally {
      requestControllersRef.current.delete(controller);
    }
  }, [
    activeSessionKeyRef,
    identity,
    markHistoryRoundResolved,
    commitHistoryCursor,
    updateMessageWindow,
    windowRef,
  ]);

  const loadNewerMessages = useCallback(async (): Promise<boolean> => {
    const controller = new AbortController();
    requestControllersRef.current.add(controller);
    try {
      return await loadNewerAgentConversationMessages({
        activeSessionKeyRef,
        historyCursorRef,
        identity,
        isRoundWindowLoadingRef,
        commitHistoryCursor,
        signal: controller.signal,
        updateMessageWindow,
        windowRef,
      });
    } finally {
      requestControllersRef.current.delete(controller);
    }
  }, [
    activeSessionKeyRef,
    identity,
    commitHistoryCursor,
    updateMessageWindow,
    windowRef,
  ]);

  const returnToLatestMessages = useCallback(async (): Promise<boolean> => {
    const controller = new AbortController();
    requestControllersRef.current.add(controller);
    try {
      return await returnToLatestAgentConversationMessages({
        activeSessionKeyRef,
        historyCursorRef,
        identity,
        isRoundWindowLoadingRef,
        commitHistoryCursor,
        signal: controller.signal,
        updateMessageWindow,
        windowRef,
      });
    } finally {
      requestControllersRef.current.delete(controller);
    }
  }, [
    activeSessionKeyRef,
    identity,
    commitHistoryCursor,
    updateMessageWindow,
    windowRef,
  ]);

  return {
    adoptHistoryStart,
    commitHistoryCursor,
    hasMoreHistory,
    historyStartRoundId,
    isHistoryLoading,
    loadNewerMessages,
    loadOlderMessages,
    loadRoundWindow,
    resolvedHistoryRoundIds,
    returnToLatestMessages,
    resetHistoryPagination,
  };
}
