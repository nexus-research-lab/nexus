"use client";

import { LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

interface UiSegmentedControlOption<T extends string> {
  label: string;
  value: T;
}

interface UiSegmentedControlProps<T extends string> {
  className?: string;
  density?: "default" | "compact";
  disabled?: boolean;
  icon?: LucideIcon;
  onChange: (value: T) => void;
  options: UiSegmentedControlOption<T>[];
  stretch?: boolean;
  title: string;
  value: T;
}

export function UiSegmentedControl<T extends string>({
  className: className,
  density = "default",
  disabled = false,
  icon: Icon,
  onChange: onChange,
  options,
  stretch = false,
  title,
  value,
}: UiSegmentedControlProps<T>) {
  return (
    <div
      aria-label={title}
      className={cn(
        "segmented-control items-center gap-px rounded-full",
        stretch ? "flex w-full" : "inline-flex",
        density === "compact" ? "p-[1.5px]" : "p-[3px]",
        !Icon && "gap-0",
        className,
      )}
      role="group"
      title={title}
    >
      {Icon ? (
        <span
          className={cn(
            "segmented-control-icon flex items-center justify-center rounded-full",
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
            "segmented-control-option rounded-full font-semibold tracking-[0.02em]",
            density === "compact" ? "px-[0.7rem] py-[3.5px] text-[9.5px]" : "px-1.5 py-[5px] text-[10px]",
            stretch && "min-w-0 flex-1 px-1.5 text-center",
          )}
          data-active={value === option.value}
          disabled={disabled}
          onClick={() => onChange(option.value)}
          type="button"
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
