"use client";

import type { MouseEventHandler } from "react";

import { cn } from "@/shared/ui/class-name";

interface PanelResizeHandleProps {
  ariaLabel: string;
  onResizeStart: MouseEventHandler<HTMLButtonElement>;
  variant?: "gutter" | "overlay";
}

/** 仅表达横向面板的拖拽起点；尺寸状态与拖拽生命周期归布局所有者。 */
export function PanelResizeHandle({
  ariaLabel,
  onResizeStart,
  variant = "overlay",
}: PanelResizeHandleProps) {
  return (
    <button
      aria-label={ariaLabel}
      className={cn(
        "z-20 hidden h-full cursor-col-resize border-0 bg-transparent p-0 outline-none lg:block",
        variant === "gutter"
          ? "relative w-2 shrink-0 self-stretch"
          : "absolute left-0 top-0 w-3",
      )}
      onMouseDown={onResizeStart}
      type="button"
    />
  );
}
