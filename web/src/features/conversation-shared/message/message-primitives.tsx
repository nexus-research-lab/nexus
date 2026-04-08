/**
 * =====================================================
 * @File   : message-primitives.tsx
 * @Date   : 2026-04-05 15:26
 * @Author : leemysw
 * 2026-04-05 15:26   Create
 * =====================================================
 */

"use client";

import { ButtonHTMLAttributes, ReactNode } from "react";
import { Brain, Globe, Loader2, MessageCircleMore, ShieldAlert } from "lucide-react";

import { cn } from "@/lib/utils";

type MessageAvatarSize = "full" | "compact";
type MessageActionTone = "default" | "success" | "danger";
type MessageLoadingDotsSize = "sm" | "md";
export type MessageActivityState =
  | "thinking"
  | "browsing"
  | "executing"
  | "waiting_permission"
  | "waiting_input";

const AVATAR_SIZE_CLASS_MAP: Record<MessageAvatarSize, string> = {
  full: "h-10 w-10 rounded-xl",
  compact: "h-6 w-6 rounded-lg",
};

const ACTION_TONE_CLASS_MAP: Record<MessageActionTone, string> = {
  default: "hover:bg-[var(--surface-interactive-hover-background)] hover:text-[color:var(--text-strong)]",
  success: "text-green-500 hover:bg-emerald-500/10 hover:text-emerald-500",
  danger: "hover:bg-rose-500/10 hover:text-rose-500",
};

const DOT_SIZE_CLASS_MAP: Record<MessageLoadingDotsSize, string> = {
  sm: "h-1 w-1",
  md: "h-1.5 w-1.5",
};

const ACTIVITY_LABEL_MAP: Record<MessageActivityState, string> = {
  thinking: "正在思考",
  browsing: "正在浏览",
  executing: "正在执行",
  waiting_permission: "等待确认",
  waiting_input: "等待输入",
};

const ACTIVITY_TONE_CLASS_MAP: Record<MessageActivityState, string> = {
  thinking: "border-sky-200/70 bg-sky-50/80 text-sky-600",
  browsing: "border-cyan-200/70 bg-cyan-50/80 text-cyan-600",
  executing: "border-indigo-200/70 bg-indigo-50/80 text-indigo-600",
  waiting_permission: "border-amber-200/80 bg-amber-50/90 text-amber-700",
  waiting_input: "border-violet-200/75 bg-violet-50/85 text-violet-600",
};

export function MessageAvatar({
  children,
  size = "full",
  class_name,
}: {
  children: ReactNode;
  size?: MessageAvatarSize;
  class_name?: string;
}) {
  return (
    <div
      className={cn(
        "flex items-center justify-center border border-[var(--surface-avatar-border)] bg-[var(--surface-avatar-background)] text-[color:var(--surface-avatar-foreground)]",
        AVATAR_SIZE_CLASS_MAP[size],
        class_name,
      )}
    >
      {children}
    </div>
  );
}

export function MessageActionButton({
  children,
  class_name,
  tone = "default",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  class_name?: string;
  tone?: MessageActionTone;
}) {
  return (
    <button
      className={cn(
        "rounded-lg p-1 text-[color:var(--icon-default)] transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-primary/50",
        ACTION_TONE_CLASS_MAP[tone],
        class_name,
      )}
      {...props}
    >
      {children}
    </button>
  );
}

export function MessageLoadingDots({
  size = "md",
  class_name,
}: {
  size?: MessageLoadingDotsSize;
  class_name?: string;
}) {
  const dot_class_name = cn(
    "rounded-full bg-[color:var(--icon-muted)] animate-bounce",
    DOT_SIZE_CLASS_MAP[size],
  );

  return (
    <span className={cn("inline-flex items-center gap-1.5", class_name)}>
      <span className={cn(dot_class_name, "[animation-delay:0ms]")} />
      <span className={cn(dot_class_name, "[animation-delay:150ms]")} />
      <span className={cn(dot_class_name, "[animation-delay:300ms]")} />
    </span>
  );
}

function MessageActivityIcon({ state }: { state: MessageActivityState }) {
  switch (state) {
    case "thinking":
      return <Brain className="h-3.5 w-3.5 animate-pulse" />;
    case "browsing":
      return <Globe className="h-3.5 w-3.5 animate-pulse" />;
    case "executing":
      return <Loader2 className="h-3.5 w-3.5 animate-spin" />;
    case "waiting_permission":
      return <ShieldAlert className="h-3.5 w-3.5 animate-pulse" />;
    case "waiting_input":
      return <MessageCircleMore className="h-3.5 w-3.5 animate-pulse" />;
  }
}

export function MessageActivityStatus({
  state,
  class_name,
}: {
  state: MessageActivityState;
  class_name?: string;
}) {
  return (
    <div className={cn("flex min-w-0 items-center", class_name)}>
      <div
        className={cn(
          "inline-flex min-w-0 items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-medium shadow-[0_1px_2px_rgba(15,23,42,0.04)] backdrop-blur-sm transition-colors",
          ACTIVITY_TONE_CLASS_MAP[state],
        )}
      >
        <span className="shrink-0">
          <MessageActivityIcon state={state} />
        </span>
        <span className="truncate">{ACTIVITY_LABEL_MAP[state]}</span>
        <MessageLoadingDots size="sm" class_name="shrink-0 opacity-70 [&>span]:bg-current" />
      </div>
    </div>
  );
}

export function MessageShell({
  children,
  separated = false,
  class_name,
}: {
  children: ReactNode;
  separated?: boolean;
  class_name?: string;
}) {
  return (
    <div
      className={cn(
        "w-full min-w-0",
        separated && "border-b border-[var(--divider-subtle-color)]",
        class_name,
      )}
    >
      {children}
    </div>
  );
}
