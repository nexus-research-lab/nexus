import { useEffect } from "react";
import type { RefObject } from "react";

import {
  findConversationRoundElement,
  scrollToConversationRoundElement,
  type ConversationRoundScrollHandleRef,
  type ConversationRoundScrollOptions,
} from "../timeline/scroll/round-scroll";

interface UseConversationRoundNavigationOptions {
  fallbackScrollToIndex?: (
    index: number,
    options?: ConversationRoundScrollOptions,
  ) => void;
  roundIds: string[];
  roundScrollRef?: ConversationRoundScrollHandleRef;
  scrollRef: RefObject<HTMLDivElement | null>;
}

export function useConversationRoundNavigation({
  fallbackScrollToIndex,
  roundIds,
  roundScrollRef,
  scrollRef,
}: UseConversationRoundNavigationOptions): void {
  useEffect(() => {
    if (!roundScrollRef) {
      return;
    }
    const handle = {
      scrollToRoundId: (
        roundId: string,
        options?: ConversationRoundScrollOptions,
      ) => {
        const scrollElement = scrollRef.current;
        const target = scrollElement
          ? findConversationRoundElement(scrollElement, roundId)
          : null;
        if (scrollElement && target) {
          scrollToConversationRoundElement(scrollElement, target, options);
          return true;
        }
        const targetIndex = roundIds.indexOf(roundId);
        if (targetIndex < 0 || !fallbackScrollToIndex) {
          return false;
        }
        fallbackScrollToIndex(targetIndex, options);
        return true;
      },
    };
    roundScrollRef.current = handle;
    return () => {
      if (roundScrollRef.current === handle) {
        roundScrollRef.current = null;
      }
    };
  }, [fallbackScrollToIndex, roundIds, roundScrollRef, scrollRef]);
}
