import { useLayoutEffect, useRef, type ReactNode } from "react";
import { CornerDownRight, Info, LoaderCircle, RotateCcw } from "lucide-react";

import { cn } from "@/lib/utils";
import type { SystemEventContent } from "@/types/conversation/message";

import {
  DEFAULT_TIMELINE_DOT_TOP,
  getSystemMessageIconClassName,
  getTimelineAnchorElement,
  getTimelineAnchorTop,
} from "./message-item-support";

export function SystemEventIcon({
  icon,
  className: className,
}: {
  icon: SystemEventContent["icon"];
  className?: string;
}) {
  if (icon === "retry") {
    return <RotateCcw className={className} />;
  }
  if (icon === "progress") {
    return <LoaderCircle className={className} />;
  }
  if (icon === "guide") {
    return <CornerDownRight className={className} />;
  }
  return <Info className={className} />;
}

export function TimelineBlock({
  children,
  active = false,
}: {
  children: ReactNode;
  active?: boolean;
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

    // 圆点位置是纯 DOM 对齐值，避免用 state 回写触发渲染递归。
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
          ref={dotRef}
          className={cn(
            "absolute left-1/2 block h-1.5 w-1.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-(--divider-subtle-color)",
            active ? "bg-primary/70" : null,
          )}
          style={{ top: `${DEFAULT_TIMELINE_DOT_TOP}px` }}
        />
      </div>
      <div ref={contentRef} className="min-w-0">
        {children}
      </div>
    </div>
  );
}
