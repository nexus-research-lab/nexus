import { useCallback, useEffect } from "react";
import type { RefObject } from "react";

import { getConversationRoundNavigationTarget } from "./scroll/round-scroll";

const HISTORY_LOAD_THRESHOLD_PX = 120;

interface UseConversationHistoryLoaderOptions {
  autoFillViewport?: boolean;
  scrollRef: RefObject<HTMLDivElement | null>;
  messageCount: number;
  hasMoreHistory: boolean;
  isHistoryLoading: boolean;
  isLoading: boolean;
  loadOlderMessages: () => Promise<boolean>;
  prepareHistoryPrependRestore: () => void;
  cancelHistoryPrependRestore: () => void;
  onScroll: () => void;
}

export function useConversationHistoryLoader({
  autoFillViewport = true,
  scrollRef,
  messageCount,
  hasMoreHistory,
  isHistoryLoading,
  isLoading,
  loadOlderMessages,
  prepareHistoryPrependRestore,
  cancelHistoryPrependRestore,
  onScroll,
}: UseConversationHistoryLoaderOptions) {
  const maybeLoadOlderMessages = useCallback(async () => {
    const container = scrollRef.current;
    if (
      !container ||
      !hasMoreHistory ||
      isHistoryLoading ||
      getConversationRoundNavigationTarget(container) ||
      container.scrollTop > HISTORY_LOAD_THRESHOLD_PX
    ) {
      return;
    }

    prepareHistoryPrependRestore();
    const didPrepend = await loadOlderMessages();
    if (!didPrepend) {
      cancelHistoryPrependRestore();
    }
  }, [
    cancelHistoryPrependRestore,
    hasMoreHistory,
    isHistoryLoading,
    loadOlderMessages,
    prepareHistoryPrependRestore,
    scrollRef,
  ]);

  const handleScroll = useCallback(() => {
    onScroll();
    void maybeLoadOlderMessages();
  }, [maybeLoadOlderMessages, onScroll]);

  useEffect(() => {
    const container = scrollRef.current;
    if (
      !autoFillViewport ||
      !container ||
      !hasMoreHistory ||
      isHistoryLoading ||
      isLoading ||
      container.scrollHeight > container.clientHeight + 24
    ) {
      return;
    }
    void maybeLoadOlderMessages();
  }, [
    autoFillViewport,
    hasMoreHistory,
    isHistoryLoading,
    isLoading,
    maybeLoadOlderMessages,
    messageCount,
    scrollRef,
  ]);

  return { handleScroll };
}
