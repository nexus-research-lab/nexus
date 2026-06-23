import { useLayoutEffect, useRef, type ReactNode } from "react";
import { CornerDownRight, Info, LoaderCircle, RotateCcw } from "lucide-react";

import { cn } from "@/lib/utils";
import type { SystemEventContent } from "@/types/conversation/message";

import {
  DEFAULT_TIMELINE_DOT_TOP,
  get_system_message_icon_class_name,
  get_timeline_anchor_element,
  get_timeline_anchor_top,
} from "./message-item-support";

export function SystemEventIcon({
  icon,
  class_name,
}: {
  icon: SystemEventContent["icon"];
  class_name?: string;
}) {
  if (icon === "retry") {
    return <RotateCcw className={class_name} />;
  }
  if (icon === "progress") {
    return <LoaderCircle className={class_name} />;
  }
  if (icon === "guide") {
    return <CornerDownRight className={class_name} />;
  }
  return <Info className={class_name} />;
}

export function TimelineBlock({
  children,
  active = false,
}: {
  children: ReactNode;
  active?: boolean;
}) {
  const content_ref = useRef<HTMLDivElement | null>(null);
  const dot_ref = useRef<HTMLSpanElement | null>(null);
  const dot_top_ref = useRef(DEFAULT_TIMELINE_DOT_TOP);

  useLayoutEffect(() => {
    const content_element = content_ref.current;
    const dot_element = dot_ref.current;
    if (!content_element || !dot_element) {
      return;
    }

    // 圆点位置是纯 DOM 对齐值，避免用 state 回写触发渲染递归。
    const update_dot_top = () => {
      const anchor_element = get_timeline_anchor_element(content_element);
      const next_dot_top = get_timeline_anchor_top(content_element, anchor_element);
      if (Math.abs(dot_top_ref.current - next_dot_top) < 0.5) {
        return;
      }
      dot_top_ref.current = next_dot_top;
      dot_element.style.top = `${next_dot_top}px`;
    };

    update_dot_top();

    const frame_id = window.requestAnimationFrame(update_dot_top);
    return () => window.cancelAnimationFrame(frame_id);
  });

  return (
    <div className="nexus-chat-timeline-block relative grid min-w-0 grid-cols-[12px_minmax(0,1fr)] items-start gap-3">
      <div className="relative">
        <span
          ref={dot_ref}
          className={cn(
            "absolute left-1/2 block h-1.5 w-1.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-(--divider-subtle-color)",
            active ? "bg-primary/70" : null,
          )}
          style={{ top: `${DEFAULT_TIMELINE_DOT_TOP}px` }}
        />
      </div>
      <div ref={content_ref} className="min-w-0">
        {children}
      </div>
    </div>
  );
}
