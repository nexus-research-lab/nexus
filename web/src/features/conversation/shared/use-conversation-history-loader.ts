import { useCallback, useEffect } from "react";
import type { RefObject } from "react";

const HISTORY_LOAD_THRESHOLD_PX = 120;

interface UseConversationHistoryLoaderOptions {
  scroll_ref: RefObject<HTMLDivElement | null>;
  message_count: number;
  has_more_history: boolean;
  is_history_loading: boolean;
  is_loading: boolean;
  load_older_messages: () => Promise<boolean>;
  prepare_history_prepend_restore: () => void;
  cancel_history_prepend_restore: () => void;
  on_scroll: () => void;
}

export function useConversationHistoryLoader({
  scroll_ref: scrollRef,
  message_count: messageCount,
  has_more_history: hasMoreHistory,
  is_history_loading: isHistoryLoading,
  is_loading: isLoading,
  load_older_messages: loadOlderMessages,
  prepare_history_prepend_restore: prepareHistoryPrependRestore,
  cancel_history_prepend_restore: cancelHistoryPrependRestore,
  on_scroll: onScroll,
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

  return { handle_scroll: handleScroll };
}
