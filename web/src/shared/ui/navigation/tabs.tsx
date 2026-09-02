// INPUT: 视图/筛选选项、当前值与切换/关闭命令。
// OUTPUT: 以 button group 语义呈现的可横向滚动选择条。
// POS: 选择条 pattern；不是站点导航，也不拥有 tabpanel 或路由生命周期。
"use client";

import { type ReactNode } from "react";
import { type LucideIcon, X } from "lucide-react";

import {
  getUiTabDismissClassName,
  getUiTabClassName,
  getUiTabsNavClassName,
  type UiTabsDensity,
} from "@/shared/ui/navigation/tabs-styles";

interface UiTabOption<TValue extends string> {
  anchor?: string;
  className?: string;
  icon?: LucideIcon;
  label: ReactNode;
  title?: string;
  value: TValue;
}

interface UiTabsProps<TValue extends string> {
  activeValue?: TValue;
  ariaLabel: string;
  className?: string;
  density?: UiTabsDensity;
  itemClassName?: string;
  navAnchor?: string;
  onChange?: (value: TValue) => void;
  onDismissActive?: (value: TValue) => void;
  dismissActiveLabel?: string;
  options: Array<UiTabOption<TValue>>;
}

export function UiTabs<TValue extends string>({
  activeValue: activeValue,
  ariaLabel: ariaLabel,
  className: className,
  density,
  itemClassName: itemClassName,
  navAnchor: navAnchor,
  onChange: onChange,
  onDismissActive: onDismissActive,
  dismissActiveLabel: dismissActiveLabel = "关闭",
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
        const wrapperClassName = [
          "ui-navigation-tab-item",
          onDismissActive ? "ui-navigation-tab-item-dismissible" : "",
          option.className ?? "",
        ].filter(Boolean).join(" ");
        const tabButton = (
          <button
            aria-pressed={isActive}
            className={getUiTabClassName(
              { active: isActive, density },
              onDismissActive
                ? `${itemClassName ?? ""} pr-8`
                : itemClassName,
            )}
            data-tour-anchor={option.anchor}
            onClick={() => onChange?.(option.value)}
            title={option.title}
            type="button"
          >
            {Icon ? <Icon className="h-3.5 w-3.5" /> : null}
            {option.label}
          </button>
        );

        if (!isActive || !onDismissActive) {
          return (
            <span
              className={`${wrapperClassName} inline-flex h-full shrink-0 items-center`}
              key={option.value}
            >
              {tabButton}
            </span>
          );
        }

        return (
          <span
            className={`${wrapperClassName} relative inline-flex h-full shrink-0 items-center`}
            key={option.value}
          >
            {tabButton}
            <button
              aria-label={dismissActiveLabel}
              className={getUiTabDismissClassName(
                "ui-navigation-tab-dismiss absolute right-1 top-1/2 -translate-y-1/2",
              )}
              onClick={(event) => {
                event.stopPropagation();
                onDismissActive(option.value);
              }}
              title={dismissActiveLabel}
              type="button"
            >
              <X className="h-3 w-3" />
            </button>
          </span>
        );
      })}
    </div>
  );
}
