// INPUT: 有限互斥选项、选项图标/可见组名、当前值与变更命令。
// OUTPUT: 按密度统一可换行文字和选项高度、唯一图标提示及 aria-pressed 状态的分段组。
// POS: Segmented control pattern；不解释业务选项或持有选中值。
"use client";

import { type LucideIcon } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { UiField } from "./form-control";

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
  onChange: (value: T) => void;
  options: ReadonlyArray<UiSegmentedControlOption<T>>;
  showLabel?: boolean;
  stretch?: boolean;
  title: string;
  value: T;
}

export function UiSegmentedControl<T extends string>({
  className,
  density = "default",
  disabled = false,
  onChange,
  options,
  showLabel = false,
  stretch = false,
  title,
  value,
}: UiSegmentedControlProps<T>) {
  const control = (
    <div
      aria-label={showLabel ? undefined : title}
      className={cn(
        "segmented-control min-w-0 max-w-full items-stretch surface-radius-md",
        stretch ? "flex w-full" : "inline-flex",
        density === "compact" ? "p-0.5" : "p-1",
        !showLabel && className,
      )}
      role={showLabel ? undefined : "group"}
    >
      {options.map((option) => {
        const OptionIcon = option.icon;
        const iconOnly = Boolean(OptionIcon && option.iconOnly);
        const button = (
          <button
            key={option.value}
            aria-pressed={value === option.value}
            className={cn(
              "segmented-control-option inline-flex min-w-0 items-center justify-center gap-1.5 whitespace-normal text-center radius-control-sm disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
              getUiTypographyClassName({ role: density === "compact" ? "supporting" : "control", weight: "medium" }),
              density === "compact" ? "min-h-7 px-2 py-0.5" : "min-h-8 px-2.5 py-1",
              stretch && "flex-auto px-1.5",
              iconOnly && (density === "compact" ? "h-7 w-7 shrink-0 self-center px-0" : "h-8 w-8 shrink-0 self-center px-0"),
            )}
            data-active={value === option.value}
            disabled={disabled}
            onClick={() => onChange(option.value)}
            type="button"
          >
            {OptionIcon ? (
              <OptionIcon
                aria-hidden="true"
                className={cn("shrink-0", density === "compact" ? "h-3.5 w-3.5" : "h-4 w-4")}
              />
            ) : null}
            <span className={iconOnly ? "sr-only" : "min-w-0 [overflow-wrap:anywhere]"}>{option.label}</span>
          </button>
        );
        return iconOnly ? <UiTooltip key={option.value} label={option.label}>{button}</UiTooltip> : button;
      })}
    </div>
  );
  return showLabel ? <UiField className={className} label={title}>{control}</UiField> : control;
}
