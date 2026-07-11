/**
 * =====================================================
 * @File   : message-rail.tsx
 * @Date   : 2026-04-05 15:08
 * @Author : leemysw
 * 2026-04-05 15:08   Create
 * =====================================================
 */

"use client";

import { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

export function MessageRail({
  children,
  className: className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "min-w-0 max-w-full overflow-hidden border-l-2 pl-4",
        className,
      )}
      style={{ borderColor: "color-mix(in srgb, var(--foreground) 18%, transparent)" }}
    >
      {children}
    </div>
  );
}

export function MessageRailLabel({
  children,
  active = false,
  className: className,
}: {
  children: ReactNode;
  active?: boolean;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex min-w-0 items-center gap-2 text-[11px] font-medium text-(--text-muted)",
        active && "text-primary",
        className,
      )}
    >
      {children}
    </div>
  );
}

export function MessageRailBody({
  children,
  className: className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("message-cjk-font min-w-0 max-w-full overflow-hidden break-words text-[11px] leading-[1.45] text-(--text-default)", className)}>
      {children}
    </div>
  );
}
