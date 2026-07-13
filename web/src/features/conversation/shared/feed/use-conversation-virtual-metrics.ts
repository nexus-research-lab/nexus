import { useEffect, useState } from "react";
import type { RefObject } from "react";

import { getConversationRoundFocusOffset } from "../timeline/scroll/round-scroll";

interface ConversationVirtualMetrics {
  containerWidth: number;
  scrollPaddingStart: number;
}

const DEFAULT_METRICS: ConversationVirtualMetrics = {
  containerWidth: 680,
  scrollPaddingStart: 180,
};

export function useConversationVirtualMetrics(
  scrollRef: RefObject<HTMLDivElement | null>,
): ConversationVirtualMetrics {
  const [metrics, setMetrics] = useState(DEFAULT_METRICS);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) {
      return;
    }
    const syncMetrics = () => {
      const next = {
        containerWidth: scrollElement.clientWidth || DEFAULT_METRICS.containerWidth,
        scrollPaddingStart: getConversationRoundFocusOffset(scrollElement),
      };
      setMetrics((current) =>
        current.containerWidth === next.containerWidth
          && current.scrollPaddingStart === next.scrollPaddingStart
          ? current
          : next,
      );
    };
    syncMetrics();
    const observer = new ResizeObserver(syncMetrics);
    observer.observe(scrollElement);
    return () => observer.disconnect();
  }, [scrollRef]);

  return metrics;
}
