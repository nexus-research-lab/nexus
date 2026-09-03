// INPUT: 有限互斥选项、可选图标/图标模式、当前值与变更命令。
// OUTPUT: 以 aria-pressed 暴露状态、且标签或图标选项保持紧凑的分段按钮组。
// POS: Segmented control pattern；不解释业务选项或持有选中值。
"use client";

import { type LucideIcon } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface UiSegmentedControlOption<T extends string> {
  icon?: LucideIcon;
  iconOnly?: boolean;
  label: string;
  value: T;
}

interface UiSegmentedControlProps<T extends string> {
  className?: string;
  density?: "default" | "compact";
  disabled?: boolean;
  icon?: LucideIcon;
  onChange: (value: T) => void;
  options: ReadonlyArray<UiSegmentedControlOption<T>>;
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
        "segmented-control items-center gap-px surface-radius-md",
        stretch ? "flex w-full" : "inline-flex",
        density === "compact" ? "p-0.5" : "p-1",
        !Icon && "gap-0",
        className,
      )}
      role="group"
      title={title}
    >
      {Icon ? (
        <span
          className={cn(
            "segmented-control-icon flex items-center justify-center radius-control-sm",
            density === "compact" ? "h-5 w-5" : "h-7 w-7",
          )}
        >
          <Icon className={cn(density === "compact" ? "h-3 w-3" : "h-3.5 w-3.5")} />
        </span>
      ) : null}

      {options.map((option) => {
        const OptionIcon = option.icon;
        const iconOnly = Boolean(OptionIcon && option.iconOnly);
        return (
          <button
            key={option.value}
            aria-pressed={value === option.value}
            className={cn(
              "segmented-control-option whitespace-nowrap radius-control-sm",
              getUiTypographyClassName({ role: "caption", weight: "semibold" }),
              density === "compact" ? "px-2 py-1" : "px-2.5 py-1.5",
              iconOnly && (density === "compact" ? "h-7 w-7 px-0" : "h-8 w-8 px-0"),
              stretch && "flex-1 px-1.5 text-center",
            )}
            data-active={value === option.value}
            disabled={disabled}
            onClick={() => onChange(option.value)}
            title={iconOnly ? option.label : undefined}
            type="button"
          >
            {OptionIcon ? (
              <OptionIcon
                aria-hidden="true"
                className={density === "compact" ? "h-3.5 w-3.5" : "h-4 w-4"}
              />
            ) : null}
            <span className={iconOnly ? "sr-only" : undefined}>{option.label}</span>
          </button>
        );
      })}
    </div>
  );
}
