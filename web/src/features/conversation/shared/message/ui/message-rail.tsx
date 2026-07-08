/**
 * =====================================================
 * @File   : message-rail.tsx
 * @Date   : 2026-04-05 15:08
 * @Author : leemysw
 * 2026-04-05 15:08   Create
 * =====================================================
 */

"use client";

import { HTMLAttributes, ReactNode } from "react";

import { cn } from "@/lib/utils";

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

function MessageCallout({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn("message-cjk-font rounded-[10px] border px-3 py-2 text-xs text-(--status-info-soft-text)", className)}
      style={{
        background: "color-mix(in srgb, var(--surface-panel-background) 86%, transparent)",
        borderColor: "color-mix(in srgb, var(--surface-panel-subtle-border) 80%, transparent)",
      }}
    >
      {children}
    </div>
  );
}

function MessageCalloutTitle({
  children,
  className,
  ...props
}: {
  children: ReactNode;
  className?: string;
} & HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn("font-semibold text-(--status-info-soft-text)", className)} {...props}>
      {children}
    </div>
  );
}

type MessageResultTone = "success" | "error";

const RESULT_TONE_CLASS_MAP: Record<MessageResultTone, string> = {
  success: "text-(--success)",
  error: "text-(--destructive)",
};

function MessageResultLabel({
  children,
  tone,
  className,
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  children: ReactNode;
  tone: MessageResultTone;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "mb-2 flex items-center gap-2 text-[11px] font-semibold",
        RESULT_TONE_CLASS_MAP[tone],
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}
