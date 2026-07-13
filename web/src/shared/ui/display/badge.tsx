"use client";

import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import {
  getUiBadgeClassName,
  type UiBadgeSize,
  type UiBadgeTone,
} from "@/shared/ui/display/badge-styles";

interface UiBadgeProps extends HTMLAttributes<HTMLSpanElement> {
  children: ReactNode;
  className?: string;
  icon?: ReactNode;
  showDot?: boolean;
  size?: UiBadgeSize;
  tone?: UiBadgeTone;
}

interface UiCounterBadgeProps extends HTMLAttributes<HTMLSpanElement> {
  className?: string;
  count: number;
  max?: number;
}

export function UiBadge({
  children,
  className,
  icon,
  showDot: showDot = false,
  size,
  tone,
  ...props
}: UiBadgeProps) {
  return (
    <span
      className={getUiBadgeClassName({ size, tone }, cn(className))}
      {...props}
    >
      {icon ?? (showDot ? <span className="h-1.5 w-1.5 rounded-full bg-current" /> : null)}
      {children}
    </span>
  );
}

export function UiCounterBadge({
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
      )}
      {...props}
    >
      {count > max ? `${max}+` : count}
    </span>
  );
}
