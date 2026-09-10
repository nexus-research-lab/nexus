// INPUT: 视图/筛选选项、当前值与切换命令。
// OUTPUT: 以 button group 语义呈现的可横向滚动选择条，每项保留稳定布局包装。
// POS: 选择条 pattern；不是站点导航，也不拥有 tabpanel 或路由生命周期。
"use client";

import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { type ReactNode } from "react";
import { type LucideIcon } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import {
  getUiTabClassName,
  getUiTabsNavClassName,
  type UiTabsDensity,
} from "@/shared/ui/navigation/tabs-styles";

export interface UiTabOption<TValue extends string> {
  anchor?: string;
  className?: string;
  icon?: LucideIcon;
  label: ReactNode;
  title?: string;
  value: TValue;
}

export interface UiTabsProps<TValue extends string> {
  activeValue?: TValue;
  ariaLabel: string;
  className?: string;
  density?: UiTabsDensity;
  itemClassName?: string;
  navAnchor?: string;
  onChange?: (value: TValue) => void;
  options: Array<UiTabOption<TValue>>;
}

export function UiTabs<TValue extends string>({
  activeValue,
  ariaLabel,
  className,
  density,
  itemClassName,
  navAnchor,
  onChange,
  options,
}: UiTabsProps<TValue>) {
  return (
    <div
      aria-label={ariaLabel}
      className={getUiTabsNavClassName(className)}
      data-tour-anchor={navAnchor}
      role="group"
    >
      {options.map((option) => {
        const Icon = option.icon;
        const isActive = activeValue === option.value;
        return (
          <span
            className={cn("ui-navigation-tab-item inline-flex h-full shrink-0 items-center", option.className)}
            key={option.value}
          >
            <UiTooltip label={option.title}><button
              aria-pressed={isActive}
              className={getUiTabClassName({ active: isActive, density }, itemClassName)}
              data-tour-anchor={option.anchor}
              onClick={() => onChange?.(option.value)}

              type="button"
            >
              {Icon ? <Icon aria-hidden="true" className="h-3.5 w-3.5" /> : null}
              {option.label}
            </button></UiTooltip>
          </span>
        );
      })}
    </div>
  );
}
