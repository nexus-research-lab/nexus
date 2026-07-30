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
