/**
 * INPUT: 滚动视口与真实内容轨道元素。
 * OUTPUT: 虚拟轮次估高使用的轨道宽度和导航顶部偏移。
 * POS: DM、Room 虚拟 Feed 共用的首帧尺寸测量层。
 */
import { useLayoutEffect, useState } from "react";
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
  feedRef?: RefObject<HTMLDivElement | null>,
): ConversationVirtualMetrics {
  const [metrics, setMetrics] = useState(DEFAULT_METRICS);

  useLayoutEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) {
      return;
    }
    const syncMetrics = () => {
      const feedElement = feedRef?.current;
      const next = {
        containerWidth:
          feedElement?.clientWidth
          || scrollElement.clientWidth
          || DEFAULT_METRICS.containerWidth,
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
    const feedElement = feedRef?.current;
    if (feedElement) {
      observer.observe(feedElement);
    }
    return () => observer.disconnect();
  }, [feedRef, scrollRef]);

  return metrics;
}
