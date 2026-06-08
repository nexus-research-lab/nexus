"use client";

import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/lib/utils";
import {
  get_ui_badge_class_name,
  type UiBadgeSize,
  type UiBadgeTone,
} from "@/shared/ui/badge-styles";

interface UiBadgeProps extends HTMLAttributes<HTMLSpanElement> {
  children: ReactNode;
  class_name?: string;
  icon?: ReactNode;
  show_dot?: boolean;
  size?: UiBadgeSize;
  tone?: UiBadgeTone;
}

interface UiCounterBadgeProps extends HTMLAttributes<HTMLSpanElement> {
  class_name?: string;
  count: number;
  max?: number;
}

export function UiBadge({
  children,
  class_name,
  className,
  icon,
  show_dot = false,
  size,
  tone,
  ...props
}: UiBadgeProps) {
  return (
    <span
      className={get_ui_badge_class_name({ size, tone }, cn(className, class_name))}
      {...props}
    >
      {icon ?? (show_dot ? <span className="h-1.5 w-1.5 rounded-full bg-current" /> : null)}
      {children}
    </span>
  );
}

export function UiCounterBadge({
  class_name,
  className,
  count,
  max = 99,
  ...props
}: UiCounterBadgeProps) {
  if (count <= 0) {
    return null;
  }

  return (
    <span
      className={cn(
        "inline-flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-(--destructive) px-1.5 text-[11px] font-semibold leading-none text-white",
        className,
        class_name,
      )}
      {...props}
    >
      {count > max ? `${max}+` : count}
    </span>
  );
}
