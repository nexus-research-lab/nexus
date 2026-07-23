import type { CSSProperties } from "react";

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
      max={metrics.maxScrollLeft}
      min={0}
      onChange={(event) => onChange(Number(event.currentTarget.value))}
      step={1}
      style={style}
      type="range"
      value={Math.min(metrics.scrollLeft, metrics.maxScrollLeft)}
    />
  );
}
