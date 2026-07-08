"use client";

import { type ReactNode } from "react";
import { type LucideIcon } from "lucide-react";

import {
  getUiUnderlineTabClassName,
  getUiUnderlineTabsNavClassName,
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
  activeValue?: TValue;
  ariaLabel: string;
  className?: string;
  density?: UiTabsDensity;
  itemClassName?: string;
  navAnchor?: string;
  onChange?: (value: TValue) => void;
  options: Array<UiUnderlineTabOption<TValue>>;
}

export function UiUnderlineTabs<TValue extends string>({
  activeValue: activeValue,
  ariaLabel: ariaLabel,
  className: className,
  density,
  itemClassName: itemClassName,
  navAnchor: navAnchor,
  onChange: onChange,
  options,
}: UiUnderlineTabsProps<TValue>) {
  return (
    <nav
      aria-label={ariaLabel}
      className={getUiUnderlineTabsNavClassName(className)}
      data-tour-anchor={navAnchor}
    >
      {options.map((option) => {
        const Icon = option.icon;
        const isActive = activeValue === option.value;
        return (
          <button
            aria-current={isActive ? "page" : undefined}
            aria-pressed={isActive}
            className={getUiUnderlineTabClassName(
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
