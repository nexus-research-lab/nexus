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
  icon?: LucideIcon;
  on_change: (value: T) => void;
  options: SegmentedPillOption<T>[];
  title: string;
  value: T;
}

export function SegmentedPill<T extends string>({
  class_name,
  icon: Icon,
  on_change,
  options,
  title,
  value,
}: SegmentedPillProps<T>) {
  return (
    <div
      aria-label={title}
      className={cn(
        "segmented-pill inline-flex items-center gap-0.5 rounded-full p-[3px]",
        !Icon && "gap-0 p-[2px]",
        class_name,
      )}
      role="group"
      title={title}
    >
      {Icon ? (
        <span className="segmented-pill-icon flex h-[26px] w-[26px] items-center justify-center rounded-full">
          <Icon className="h-3.5 w-3.5" />
        </span>
      ) : null}

      {options.map((option) => (
        <button
          key={option.value}
          className="segmented-pill-option rounded-full px-2.5 py-[5px] text-[10px] font-semibold tracking-[0.02em]"
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
