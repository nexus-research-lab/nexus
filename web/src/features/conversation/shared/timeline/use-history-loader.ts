import { useCallback, useEffect } from "react";
import type { RefObject } from "react";

import { getConversationRoundNavigationTarget } from "./scroll/round-scroll";

const HISTORY_LOAD_THRESHOLD_PX = 120;

interface ConversationHistoryLoadDecision {
  allowWhileFollowing: boolean;
  enabled: boolean;
  followingLatest: boolean;
  hasMoreHistory: boolean;
  hasNavigationTarget: boolean;
  isHistoryLoading: boolean;
  isLoading: boolean;
  scrollTop: number;
}

export function shouldLoadOlderConversationMessages({
  allowWhileFollowing,
  enabled,
  followingLatest,
  hasMoreHistory,
  hasNavigationTarget,
  isHistoryLoading,
  isLoading,
  scrollTop,
}: ConversationHistoryLoadDecision): boolean {
  return (
    enabled
    && hasMoreHistory
    && !isHistoryLoading
    && !isLoading
    && !hasNavigationTarget
    && (allowWhileFollowing || !followingLatest)
    && scrollTop <= HISTORY_LOAD_THRESHOLD_PX
  );
}

interface UseConversationHistoryLoaderOptions {
  enabled?: boolean;
  scrollRef: RefObject<HTMLDivElement | null>;
  messageCount: number;
  hasMoreHistory: boolean;
  isHistoryLoading: boolean;
  isFollowingLatest: () => boolean;
  isLoading: boolean;
  loadOlderMessages: () => Promise<boolean>;
  prepareHistoryPrependRestore: () => void;
  cancelHistoryPrependRestore: () => void;
  onScroll: () => void;
}

export function useConversationHistoryLoader({
  enabled = true,
  scrollRef,
  messageCount,
  hasMoreHistory,
  isHistoryLoading,
  isFollowingLatest,
  isLoading,
  loadOlderMessages,
  prepareHistoryPrependRestore,
  cancelHistoryPrependRestore,
  onScroll,
}: UseConversationHistoryLoaderOptions) {
  const maybeLoadOlderMessages = useCallback(async (
    allowWhileFollowing = false,
  ) => {
    const container = scrollRef.current;
    if (
      !container
      || !shouldLoadOlderConversationMessages({
        allowWhileFollowing,
        enabled,
        followingLatest: isFollowingLatest(),
        hasMoreHistory,
        hasNavigationTarget: Boolean(
          getConversationRoundNavigationTarget(container),
        ),
        isHistoryLoading,
        isLoading,
        scrollTop: container.scrollTop,
      })
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
    enabled,
    hasMoreHistory,
    isHistoryLoading,
    isFollowingLatest,
    isLoading,
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
      container.scrollHeight > container.clientHeight + 24
    ) {
      return;
    }
    void maybeLoadOlderMessages(true);
  }, [
    maybeLoadOlderMessages,
    messageCount,
    scrollRef,
  ]);

  return { handleScroll };
}
