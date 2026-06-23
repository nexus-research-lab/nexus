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
  scroll_ref,
  message_count,
  has_more_history,
  is_history_loading,
  is_loading,
  load_older_messages,
  prepare_history_prepend_restore,
  cancel_history_prepend_restore,
  on_scroll,
}: UseConversationHistoryLoaderOptions) {
  const maybe_load_older_messages = useCallback(async () => {
    const container = scroll_ref.current;
    if (
      !container ||
      !has_more_history ||
      is_history_loading ||
      container.scrollTop > HISTORY_LOAD_THRESHOLD_PX
    ) {
      return;
    }

    prepare_history_prepend_restore();
    const did_prepend = await load_older_messages();
    if (!did_prepend) {
      cancel_history_prepend_restore();
    }
  }, [
    cancel_history_prepend_restore,
    has_more_history,
    is_history_loading,
    load_older_messages,
    prepare_history_prepend_restore,
    scroll_ref,
  ]);

  const handle_scroll = useCallback(() => {
    on_scroll();
    void maybe_load_older_messages();
  }, [maybe_load_older_messages, on_scroll]);

  useEffect(() => {
    const container = scroll_ref.current;
    if (
      !container ||
      !has_more_history ||
      is_history_loading ||
      is_loading ||
      container.scrollHeight > container.clientHeight + 24
    ) {
      return;
    }
    void maybe_load_older_messages();
  }, [
    has_more_history,
    is_history_loading,
    is_loading,
    maybe_load_older_messages,
    message_count,
    scroll_ref,
  ]);

  return { handle_scroll };
}
