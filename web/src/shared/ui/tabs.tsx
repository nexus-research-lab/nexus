"use client";

import { type ReactNode } from "react";
import { type LucideIcon, X } from "lucide-react";

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
  onDismissActive?: (value: TValue) => void;
  dismissActiveLabel?: string;
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
  onDismissActive: onDismissActive,
  dismissActiveLabel: dismissActiveLabel = "关闭",
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
        const tabButton = (
          <button
            aria-current={isActive ? "page" : undefined}
            aria-pressed={isActive}
            className={getUiUnderlineTabClassName(
              { active: isActive, density },
              isActive && onDismissActive
                ? `${itemClassName ?? ""} pr-5`
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
          return <span className="inline-flex h-full shrink-0 items-center" key={option.value}>{tabButton}</span>;
        }

        return (
          <span className="relative inline-flex h-full shrink-0 items-center" key={option.value}>
            {tabButton}
            <button
              aria-label={dismissActiveLabel}
              className="absolute right-0 top-1/2 flex h-5 w-5 -translate-y-1/2 items-center justify-center rounded-full text-(--icon-muted) transition-colors duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
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
    </nav>
  );
}
