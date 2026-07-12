"use client";

import type { MouseEventHandler } from "react";

interface PanelResizeHandleProps {
  ariaLabel: string;
  onResizeStart: MouseEventHandler<HTMLButtonElement>;
}

/** 仅表达横向面板的拖拽起点；尺寸状态与拖拽生命周期归布局所有者。 */
export function PanelResizeHandle({
  ariaLabel,
  onResizeStart,
}: PanelResizeHandleProps) {
  return (
    <button
      aria-label={ariaLabel}
      className="group absolute left-0 top-0 z-20 hidden h-full w-3 cursor-col-resize items-center justify-start lg:flex"
      onMouseDown={onResizeStart}
      type="button"
    >
      <span
        aria-hidden="true"
        className="pointer-events-none h-0 w-0 border-y-[5px] border-y-transparent border-l-[6px] border-l-[color:color-mix(in_srgb,var(--foreground)_34%,transparent)] opacity-0 transition-[opacity,border-color] duration-(--motion-duration-fast) group-hover:opacity-100 group-hover:border-l-[color:color-mix(in_srgb,var(--foreground)_60%,transparent)]"
      />
    </button>
  );
}
