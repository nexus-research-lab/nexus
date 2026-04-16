/**
 * =====================================================
 * @File   : segmented-pill.tsx
 * @Date   : 2026-04-04 18:45
 * @Author : leemysw
 * 2026-04-04 18:45   Create
 * =====================================================
 */

"use client";

import { LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

interface SegmentedPillOption<T extends string> {
  label: string;
  value: T;
}

interface SegmentedPillProps<T extends string> {
  class_name?: string;
  density?: "default" | "compact";
  icon?: LucideIcon;
  on_change: (value: T) => void;
  options: SegmentedPillOption<T>[];
  stretch?: boolean;
  title: string;
  value: T;
}

export function SegmentedPill<T extends string>({
  class_name,
  density = "default",
  icon: Icon,
  on_change,
  options,
  stretch = false,
  title,
  value,
}: SegmentedPillProps<T>) {
  return (
    <div
      aria-label={title}
      className={cn(
        "segmented-pill items-center gap-px rounded-full",
        stretch ? "flex w-full" : "inline-flex",
        density === "compact" ? "p-[1.5px]" : "p-[3px]",
        !Icon && "gap-0",
        class_name,
      )}
      role="group"
      title={title}
    >
      {Icon ? (
        <span
          className={cn(
            "segmented-pill-icon flex items-center justify-center rounded-full",
            density === "compact" ? "h-[21px] w-[21px]" : "h-[26px] w-[26px]",
          )}
        >
          <Icon className={cn(density === "compact" ? "h-3 w-3" : "h-3.5 w-3.5")} />
        </span>
      ) : null}

      {options.map((option) => (
        <button
          key={option.value}
          className={cn(
            "segmented-pill-option rounded-full font-semibold tracking-[0.02em]",
            density === "compact" ? "px-[0.7rem] py-[3.5px] text-[9.5px]" : "px-1.5 py-[5px] text-[10px]",
            stretch && "min-w-0 flex-1 px-1.5 text-center",
          )}
          data-active={value === option.value}
          onClick={() => on_change(option.value)}
          type="button"
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
