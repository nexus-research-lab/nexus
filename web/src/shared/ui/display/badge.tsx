// INPUT: Badge 内容、可选图标/状态点与有限的 size/tone/shape 语义。
// OUTPUT: 统一外形和状态颜色的只读 Badge，以及正数 Counter Badge。
// POS: Badge DOM 原语；不解释业务状态或计数来源。

"use client";

import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import {
  getUiBadgeClassName,
  type UiBadgeShape,
  type UiBadgeSize,
  type UiBadgeTone,
} from "@/shared/ui/display/badge-styles";

interface UiBadgeProps extends HTMLAttributes<HTMLSpanElement> {
  children: ReactNode;
  className?: string;
  icon?: ReactNode;
  showDot?: boolean;
  shape?: UiBadgeShape;
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
  shape,
  size,
  tone,
  ...props
}: UiBadgeProps) {
  return (
    <span
      className={getUiBadgeClassName({ shape, size, tone }, cn(className))}
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
        "inline-flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-(--destructive) px-1.5 text-xs font-semibold leading-none text-white",
        className,
      )}
      {...props}
    >
      {count > max ? `${max}+` : count}
    </span>
  );
}
