import { useLayoutEffect, useRef, type ReactNode } from "react";

import { cn } from "@/lib/utils";

import {
  DEFAULT_TIMELINE_DOT_TOP,
  getTimelineAnchorElement,
  getTimelineAnchorTop,
} from "../../message-item-support";

export function TimelineBlock({
  active = false,
  children,
}: {
  active?: boolean;
  children: ReactNode;
}) {
  const contentRef = useRef<HTMLDivElement | null>(null);
  const dotRef = useRef<HTMLSpanElement | null>(null);
  const dotTopRef = useRef(DEFAULT_TIMELINE_DOT_TOP);

  useLayoutEffect(() => {
    const contentElement = contentRef.current;
    const dotElement = dotRef.current;
    if (!contentElement || !dotElement) {
      return;
    }

    // 圆点位置只写入 DOM，避免测量值进入 React 状态后形成渲染循环。
    const updateDotTop = () => {
      const anchorElement = getTimelineAnchorElement(contentElement);
      const nextDotTop = getTimelineAnchorTop(contentElement, anchorElement);
      if (Math.abs(dotTopRef.current - nextDotTop) < 0.5) {
        return;
      }
      dotTopRef.current = nextDotTop;
      dotElement.style.top = `${nextDotTop}px`;
    };

    updateDotTop();
    const frameId = window.requestAnimationFrame(updateDotTop);
    return () => window.cancelAnimationFrame(frameId);
  });

  return (
    <div className="nexus-chat-timeline-block relative grid min-w-0 grid-cols-[12px_minmax(0,1fr)] items-start gap-3">
      <div className="relative">
        <span
          className={cn(
            "absolute left-1/2 block h-1.5 w-1.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-(--divider-subtle-color)",
            active && "bg-primary/70",
          )}
          ref={dotRef}
          style={{ top: `${DEFAULT_TIMELINE_DOT_TOP}px` }}
        />
      </div>
      <div className="min-w-0" ref={contentRef}>
        {children}
      </div>
    </div>
  );
}
