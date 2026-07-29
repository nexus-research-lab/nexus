/**
 * INPUT: static/virtual Feed 轮次身份、DOM/索引定位能力与共享导航 ref。
 * OUTPUT: 在父级 layout 阶段可用、可定位已挂载或虚拟轮次的导航句柄。
 * POS: shared Feed 到 timeline round-scroll 协议的注册边界。
 */
import { useLayoutEffect } from "react";
import type { RefObject } from "react";

import {
  findConversationRoundElement,
  scrollToConversationRoundElement,
  type ConversationRoundScrollHandle,
  type ConversationRoundScrollHandleRef,
  type ConversationRoundScrollOptions,
} from "../timeline/scroll/round-scroll";

interface UseConversationRoundNavigationOptions {
  fallbackScrollToIndex?: (
    index: number,
    options?: ConversationRoundScrollOptions,
  ) => void;
  roundIds: string[];
  roundIdAliases?: ReadonlyMap<string, string>;
  roundScrollRef?: ConversationRoundScrollHandleRef;
  scrollRef: RefObject<HTMLDivElement | null>;
}

interface CreateConversationRoundScrollHandleOptions {
  fallbackScrollToIndex?: (
    index: number,
    options?: ConversationRoundScrollOptions,
  ) => void;
  getScrollElement: () => HTMLDivElement | null;
  roundIds: readonly string[];
  roundIdAliases?: ReadonlyMap<string, string>;
}

export function createConversationRoundScrollHandle({
  fallbackScrollToIndex,
  getScrollElement,
  roundIds,
  roundIdAliases,
}: CreateConversationRoundScrollHandleOptions): ConversationRoundScrollHandle {
  return {
    scrollToRoundId: (
      roundId: string,
      options?: ConversationRoundScrollOptions,
    ) => {
      const timelineRoundId = roundIdAliases?.get(roundId) ?? roundId;
      const scrollElement = getScrollElement();
      const target = scrollElement
        ? findConversationRoundElement(scrollElement, timelineRoundId)
        : null;
      if (scrollElement && target) {
        scrollToConversationRoundElement(scrollElement, target, options);
        return true;
      }
      const targetIndex = roundIds.indexOf(timelineRoundId);
      if (targetIndex < 0 || !fallbackScrollToIndex) {
        return false;
      }
      fallbackScrollToIndex(targetIndex, options);
      return true;
    },
  };
}

export function useConversationRoundNavigation({
  fallbackScrollToIndex,
  roundIds,
  roundIdAliases,
  roundScrollRef,
  scrollRef,
}: UseConversationRoundNavigationOptions): void {
  useLayoutEffect(() => {
    if (!roundScrollRef) {
      return;
    }
    const handle = createConversationRoundScrollHandle({
      fallbackScrollToIndex,
      getScrollElement: () => scrollRef.current,
      roundIds,
      roundIdAliases,
    });
    roundScrollRef.current = handle;
    return () => {
      if (roundScrollRef.current === handle) {
        roundScrollRef.current = null;
      }
    };
  }, [
    fallbackScrollToIndex,
    roundIdAliases,
    roundIds,
    roundScrollRef,
    scrollRef,
  ]);
}
