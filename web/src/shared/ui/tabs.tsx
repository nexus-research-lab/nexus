"use client";

import { type ReactNode } from "react";
import { type LucideIcon } from "lucide-react";

import {
  get_ui_underline_tab_class_name,
  get_ui_underline_tabs_nav_class_name,
  type UiTabsDensity,
} from "@/shared/ui/tabs-styles";

interface UiUnderlineTabOption<TValue extends string> {
  anchor?: string;
  icon?: LucideIcon;
  label: ReactNode;
  title?: string;
  value: TValue;
}

interface UiUnderlineTabsProps<TValue extends string> {
  active_value?: TValue;
  aria_label: string;
  class_name?: string;
  density?: UiTabsDensity;
  item_class_name?: string;
  nav_anchor?: string;
  on_change?: (value: TValue) => void;
  options: Array<UiUnderlineTabOption<TValue>>;
}

export function UiUnderlineTabs<TValue extends string>({
  active_value: activeValue,
  aria_label: ariaLabel,
  class_name: className,
  density,
  item_class_name: itemClassName,
  nav_anchor: navAnchor,
  on_change: onChange,
  options,
}: UiUnderlineTabsProps<TValue>) {
  return (
    <nav
      aria-label={ariaLabel}
      className={get_ui_underline_tabs_nav_class_name(className)}
      data-tour-anchor={navAnchor}
    >
      {options.map((option) => {
        const Icon = option.icon;
        const isActive = activeValue === option.value;
        return (
          <button
            aria-current={isActive ? "page" : undefined}
            aria-pressed={isActive}
            className={get_ui_underline_tab_class_name(
              { active: isActive, density },
              itemClassName,
            )}
            data-tour-anchor={option.anchor}
            key={option.value}
            onClick={() => onChange?.(option.value)}
            title={option.title}
            type="button"
          >
            {Icon ? <Icon className="h-3.5 w-3.5" /> : null}
            {option.label}
          </button>
        );
      })}
    </nav>
  );
}
