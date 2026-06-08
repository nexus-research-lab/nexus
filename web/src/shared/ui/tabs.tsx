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
  active_value,
  aria_label,
  class_name,
  density,
  item_class_name,
  nav_anchor,
  on_change,
  options,
}: UiUnderlineTabsProps<TValue>) {
  return (
    <nav
      aria-label={aria_label}
      className={get_ui_underline_tabs_nav_class_name(class_name)}
      data-tour-anchor={nav_anchor}
    >
      {options.map((option) => {
        const Icon = option.icon;
        const is_active = active_value === option.value;
        return (
          <button
            aria-current={is_active ? "page" : undefined}
            aria-pressed={is_active}
            className={get_ui_underline_tab_class_name(
              { active: is_active, density },
              item_class_name,
            )}
            data-tour-anchor={option.anchor}
            key={option.value}
            onClick={() => on_change?.(option.value)}
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
