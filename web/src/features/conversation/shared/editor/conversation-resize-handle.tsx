"use client";

import { MouseEventHandler } from "react";

import { cn } from "@/lib/utils";

interface ConversationResizeHandleProps {
  aria_label: string;
  class_name?: string;
  on_mouse_down: MouseEventHandler<HTMLButtonElement>;
}

export function ConversationResizeHandle({
  aria_label: ariaLabel,
  class_name: className,
  on_mouse_down: onMouseDown,
}: ConversationResizeHandleProps) {
  return (
    <button
      aria-label={ariaLabel}
      className={cn(
        "group absolute left-0 top-0 z-20 hidden h-full w-3 cursor-col-resize items-center justify-start lg:flex",
        className,
      )}
      onMouseDown={onMouseDown}
      type="button"
    >
      <span
        aria-hidden="true"
        className="pointer-events-none h-0 w-0 border-y-[5px] border-y-transparent border-l-[6px] border-l-[color:color-mix(in_srgb,var(--foreground)_34%,transparent)] opacity-0 transition-[opacity,border-color] duration-(--motion-duration-fast) group-hover:opacity-100 group-hover:border-l-[color:color-mix(in_srgb,var(--foreground)_60%,transparent)]"
      />
    </button>
  );
}
