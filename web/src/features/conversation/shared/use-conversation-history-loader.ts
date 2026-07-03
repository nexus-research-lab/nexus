import { useCallback, useEffect } from "react";
import type { RefObject } from "react";

const HISTORY_LOAD_THRESHOLD_PX = 120;

interface UseConversationHistoryLoaderOptions {
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
    hasMoreHistory,
    isHistoryLoading,
    isLoading,
    maybeLoadOlderMessages,
    messageCount,
    scrollRef,
  ]);

  return { handleScroll };
}
