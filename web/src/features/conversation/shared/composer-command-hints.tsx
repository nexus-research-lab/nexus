"use client";

import { Command } from "lucide-react";

import { cn } from "@/lib/utils";
import type { ComposerCommandHintItem } from "./composer-command-hint-model";

interface ComposerCommandHintPopoverProps {
  items: ComposerCommandHintItem[];
  on_select: (item: ComposerCommandHintItem) => void;
}

export function ComposerCommandHintPopover({
  items,
  on_select,
}: ComposerCommandHintPopoverProps) {
  if (items.length === 0) {
    return null;
  }

  return (
    <div className="absolute bottom-full left-0 z-20 mb-2 w-[min(420px,calc(100vw-48px))] overflow-hidden rounded-lg border border-(--surface-canvas-border) bg-(--surface-elevated-background) p-1 shadow-lg">
      {items.map((item) => (
        <button
          key={item.command}
          className={cn(
            "flex w-full min-w-0 items-center gap-2 rounded-md px-2 py-1.5 text-left",
            "transition-colors hover:bg-(--surface-interactive-hover-background)",
          )}
          type="button"
          onClick={() => on_select(item)}
        >
          <span className="grid h-6 w-6 shrink-0 place-items-center rounded-md border border-(--surface-canvas-border) text-(--text-soft)">
            <Command className="h-3.5 w-3.5" />
          </span>
          <span className="min-w-0 flex-1">
            <span className="block truncate text-[12px] font-semibold text-(--text-strong)">
              {item.command}
            </span>
            <span className="block truncate text-[11px] text-(--text-soft)">
              {item.detail}
            </span>
          </span>
        </button>
      ))}
    </div>
  );
}
