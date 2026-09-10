// INPUT: 已归一化的标签滚动尺寸、位置与滚动命令。
// OUTPUT: 原生 range 滚动轨道，按压和捕获丢失只改变临时外观。
// POS: Workspace 标签滚动控制，不拥有滚动位置真相或业务状态。
import { useState, type CSSProperties } from "react";

import type { ConversationTabsScrollMetrics } from "./use-conversation-tabs-scroll";

interface ConversationTabsScrollRailProps {
  ariaLabel: string;
  metrics: ConversationTabsScrollMetrics;
  onChange: (scrollLeft: number) => void;
}

export function ConversationTabsScrollRail({
  ariaLabel,
  metrics,
  onChange,
}: ConversationTabsScrollRailProps) {
  const [isDragging, setIsDragging] = useState(false);
  const thumbWidth = metrics.scrollWidth > 0
    ? Math.max(28, (metrics.clientWidth / metrics.scrollWidth) * metrics.clientWidth)
    : 28;
  const style = {
    "--conversation-tabs-scroll-thumb-width": `${thumbWidth}px`,
  } as CSSProperties;

  return (
    <input
      aria-label={ariaLabel}
      className="workspace-conversation-tabs-scroll-rail"
      data-dragging={isDragging ? "true" : "false"}
      max={metrics.maxScrollLeft}
      min={0}
      onBlur={() => setIsDragging(false)}
      onChange={(event) => onChange(Number(event.currentTarget.value))}
      onLostPointerCapture={() => setIsDragging(false)}
      onPointerCancel={() => setIsDragging(false)}
      onPointerDown={(event) => {
        if (event.button === 0) {
          setIsDragging(true);
        }
      }}
      onPointerUp={() => setIsDragging(false)}
      step={1}
      style={style}
      type="range"
      value={Math.min(metrics.scrollLeft, metrics.maxScrollLeft)}
    />
  );
}
