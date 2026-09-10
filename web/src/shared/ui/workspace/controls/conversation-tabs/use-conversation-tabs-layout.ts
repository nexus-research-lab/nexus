// INPUT: 已打开标签、活动身份与左右固定动作的存在性。
// OUTPUT: 容器测量、标签宽度、溢出和原有滚动/拖拽控制。
// POS: 共享标签视图几何 owner；不读取 Room Store 或决定打开/关闭事务。

import { useLayoutEffect, useMemo, useRef, useState } from "react";

import { calculateConversationTabWidths, hasConversationTabsOverflow } from "./conversation-tabs-model";
import { useConversationTabsScroll } from "./use-conversation-tabs-scroll";

export function useConversationTabsLayout({
  activeConversationId,
  hasCreateButton,
  hasLeadingControl,
  tabs,
}: {
  activeConversationId: string | null;
  hasCreateButton: boolean;
  hasLeadingControl: boolean;
  tabs: readonly { id: string }[];
}) {
  const trackRef = useRef<HTMLElement | null>(null);
  const [trackWidth, setTrackWidth] = useState(0);
  const hasTabsOverflow = useMemo(() => hasConversationTabsOverflow({
    conversationCount: tabs.length,
    hasCreateButton,
    hasLeadingControl,
    trackWidth,
  }), [hasCreateButton, hasLeadingControl, tabs.length, trackWidth]);
  const tabsScroll = useConversationTabsScroll({
    activeConversationId,
    contentKey: tabs.map((tab) => tab.id).join(":"),
  });
  const tabWidths = useMemo(() => calculateConversationTabWidths({
    activeConversationId,
    hasCreateButton,
    hasLeadingControl,
    hasTabsOverflow,
    tabs,
    trackWidth,
  }), [activeConversationId, hasCreateButton, hasLeadingControl, hasTabsOverflow, tabs, trackWidth]);

  useLayoutEffect(() => {
    const trackElement = trackRef.current;
    if (!trackElement) return undefined;
    const updateTrackWidth = () => {
      setTrackWidth((currentWidth) => {
        const nextWidth = trackElement.clientWidth;
        return currentWidth === nextWidth ? currentWidth : nextWidth;
      });
    };
    updateTrackWidth();
    const resizeObserver = new ResizeObserver(updateTrackWidth);
    resizeObserver.observe(trackElement);
    return () => resizeObserver.disconnect();
  }, []);

  return { hasTabsOverflow, tabsScroll, tabWidths, trackRef };
}
