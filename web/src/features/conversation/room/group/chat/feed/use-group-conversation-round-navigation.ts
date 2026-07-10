import { useEffect } from "react";

import {
  findConversationRoundElement,
  scrollToConversationRoundElement,
  type ConversationRoundScrollHandleRef,
  type ConversationRoundScrollOptions,
} from "@/features/conversation/shared/conversation-round-scroll";
import type { RefObject } from "react";

interface UseGroupConversationRoundNavigationOptions {
  fallbackScrollToIndex?: (
    index: number,
    options?: ConversationRoundScrollOptions,
  ) => void;
  roundIds: string[];
  roundScrollRef?: ConversationRoundScrollHandleRef;
  scrollRef: RefObject<HTMLDivElement | null>;
}

export function useGroupConversationRoundNavigation({
  fallbackScrollToIndex,
  roundIds,
  roundScrollRef,
  scrollRef,
}: UseGroupConversationRoundNavigationOptions): void {
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
