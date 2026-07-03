/**
 * =====================================================
 * @File   : message-primitives.tsx
 * @Date   : 2026-04-05 15:26
 * @Author : leemysw
 * 2026-04-05 15:26   Create
 * =====================================================
 */

"use client";

import { ButtonHTMLAttributes, MouseEvent, ReactNode, useEffect, useRef } from "react";
import { Brain, Globe, MessageCircleMore, MessageSquareText, ShieldAlert, Wrench } from "lucide-react";
import spinners, { type BrailleSpinnerName } from "unicode-animations";

import { usePrefersReducedMotion } from "@/hooks/ui/use-prefers-reduced-motion";
import { cn, getIconAvatarSrc } from "@/lib/utils";

type MessageAvatarSize = "full" | "compact";
type MessageActionTone = "default" | "success" | "danger";
type MessageLoadingDotsSize = "sm" | "md";
export type MessageActivityState =
  | "sending"
  | "thinking"
  | "replying"
  | "browsing"
  | "executing"
  | "waiting_permission"
  | "waiting_input";

const AVATAR_SIZE_CLASS_MAP: Record<MessageAvatarSize, string> = {
  full: "h-10 w-10 rounded-xl",
  compact: "h-6 w-6 rounded-lg",
};

const ACTION_TONE_CLASS_MAP: Record<MessageActionTone, string> = {
  default: "hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
  success: "text-(--success) hover:bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] hover:text-(--success)",
  danger: "text-(--destructive) hover:bg-[color:color-mix(in_srgb,var(--destructive)_10%,transparent)] hover:text-(--destructive)",
};

function getFirstVisibleSpinnerFrameIndex(name: BrailleSpinnerName): number {
  const firstVisibleFrameIndex = spinners[name].frames.findIndex(
    (frame) => frame.replace(/⠀/g, "").length > 0,
  );
  return firstVisibleFrameIndex >= 0 ? firstVisibleFrameIndex : 0;
}

const ACTIVITY_LABEL_MAP: Record<MessageActivityState, string> = {
  sending: "正在发送",
  thinking: "正在思考",
  replying: "正在回复",
  browsing: "正在浏览",
  executing: "正在执行",
  waiting_permission: "等待确认",
  waiting_input: "等待输入",
};

const ACTIVITY_TONE_CLASS_MAP: Record<MessageActivityState, string> = {
  sending: "text-(--text-muted)",
  thinking: "text-(--text-muted)",
  replying: "text-(--text-default)",
  browsing: "text-[color:color-mix(in_srgb,var(--primary)_76%,var(--accent)_24%)]",
  executing: "text-(--primary)",
  waiting_permission: "text-(--warning)",
  waiting_input: "text-[color:color-mix(in_srgb,var(--primary)_72%,var(--text-strong)_28%)]",
};

const ACTIVITY_SPINNER_MAP: Record<MessageActivityState, BrailleSpinnerName> = {
  sending: "braille",
  thinking: "braille",
  replying: "dna",
  browsing: "braille",
  executing: "dna",
  waiting_permission: "braille",
  waiting_input: "dna",
};

export function MessageAvatar({
  ariaLabel: ariaLabel,
  avatarUrl: avatarUrl,
  children,
  size = "full",
  className: className,
  onClick: onClick,
  title,
}: {
  ariaLabel?: string;
  avatarUrl?: string | null;
  children?: ReactNode;
  size?: MessageAvatarSize;
  className?: string;
  onClick?: (event: MouseEvent<HTMLButtonElement>) => void;
  title?: string;
}) {
  const resolvedAvatarUrl = getIconAvatarSrc(avatarUrl);
  const avatarShellClassName = cn(
    "overflow-hidden border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)",
    "transition-[transform,box-shadow,border-color] duration-(--motion-duration-fast) ease-out",
    "motion-safe:hover:-translate-y-[1px] motion-safe:hover:scale-[1.06]",
    "motion-safe:hover:border-(--surface-interactive-active-border)",
    "motion-safe:hover:shadow-[0_10px_22px_rgba(15,23,42,0.14)]",
    AVATAR_SIZE_CLASS_MAP[size],
    className,
  );
  const avatarContent = resolvedAvatarUrl ? (
    <img
      src={resolvedAvatarUrl}
      alt=""
      className="h-full w-full object-cover transition-transform duration-(--motion-duration-fast) ease-out motion-safe:hover:scale-[1.04]"
    />
  ) : (
    <span className="flex h-full w-full items-center justify-center text-(--surface-avatar-foreground)">
      {children}
    </span>
  );

  if (onClick) {
    return (
      <button
        aria-label={ariaLabel ?? "查看头像详情"}
        className={cn(
          avatarShellClassName,
          "cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/45",
        )}
        onClick={onClick}
        title={title}
        type="button"
      >
        {avatarContent}
      </button>
    );
  }

  return (
    <div
      className={cn(
        avatarShellClassName,
        !resolvedAvatarUrl && "flex items-center justify-center text-(--surface-avatar-foreground)",
      )}
      title={title}
    >
      {avatarContent}
    </div>
  );
}

export function MessageActionButton({
  children,
  className: className,
  tone = "default",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  className?: string;
  tone?: MessageActionTone;
}) {
  return (
    <button
      className={cn(
        "rounded-lg p-1 text-(--icon-default) transition-colors duration-(--motion-duration-fast) focus-visible:ring-2 focus-visible:ring-primary/50",
        ACTION_TONE_CLASS_MAP[tone],
        className,
      )}
      type="button"
      {...props}
    >
      {children}
    </button>
  );
}

function MessageLoadingDots({
  size: _size = "md",
  className: className,
  name = "braille",
}: {
  size?: MessageLoadingDotsSize;
  className?: string;
  name?: BrailleSpinnerName;
}) {
  const spinner = spinners[name];
  const firstVisibleFrameIndex = getFirstVisibleSpinnerFrameIndex(name);

  const spinnerWidth = Math.max(
    ...spinner.frames.map((frame) => Array.from(frame).length),
  );
  const currentFrame = spinner.frames[firstVisibleFrameIndex] ?? spinner.frames[0];

  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-grid h-[1em] select-none place-items-center whitespace-pre leading-[1em] text-current align-middle text-[1.4em]",
        className,
      )}
      style={{ width: `${spinnerWidth}ch` }}
    >
      <span
        className="block font-mono leading-none"
        style={{ transform: "translateY(0.02em)" }}
      >
        {currentFrame}
      </span>
    </span>
  );
}

function MessageActivityIcon({ state }: { state: MessageActivityState }) {
  switch (state) {
    case "sending":
      return <MessageSquareText className="h-3.5 w-3.5" />;
    case "thinking":
      return <Brain className="h-3.5 w-3.5" />;
    case "replying":
      return <MessageSquareText className="h-3.5 w-3.5" />;
    case "browsing":
      return <Globe className="h-3.5 w-3.5" />;
    case "executing":
      return <Wrench className="h-3.5 w-3.5" />;
    case "waiting_permission":
      return <ShieldAlert className="h-3.5 w-3.5" />;
    case "waiting_input":
      return <MessageCircleMore className="h-3.5 w-3.5" />;
  }
}

function MessageActivityLabel({ state }: { state: MessageActivityState }) {
  const prefersReducedMotion = usePrefersReducedMotion();
  const shimmerRef = useRef<HTMLSpanElement | null>(null);

  useEffect(() => {
    const element = shimmerRef.current;
    if (!element || prefersReducedMotion || typeof element.animate !== "function") {
      return;
    }

    // 流光只作用在文字本身，避免整块状态条一起闪烁，信息层级更稳定。
    const animation = element.animate(
      [
        { backgroundPosition: "200% 50%" },
        { backgroundPosition: "-200% 50%" },
      ],
      {
        duration: 1800,
        easing: "linear",
        iterations: Infinity,
      },
    );

    return () => {
      animation.cancel();
    };
  }, [prefersReducedMotion, state]);

  if (prefersReducedMotion) {
    return <span className="truncate">{ACTIVITY_LABEL_MAP[state]}</span>;
  }

  return (
    <span
      className="relative inline-flex min-w-0 truncate text-current"
    >
      <span className="truncate">{ACTIVITY_LABEL_MAP[state]}</span>
      <span
        ref={shimmerRef}
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 truncate bg-clip-text text-transparent opacity-65 [-webkit-text-fill-color:transparent]"
        style={{
          backgroundImage: "linear-gradient(90deg, transparent 0%, transparent 32%, rgba(255,255,255,0.92) 50%, transparent 68%, transparent 100%)",
          backgroundSize: "220% 100%",
          backgroundPosition: "200% 50%",
        }}
      >
        {ACTIVITY_LABEL_MAP[state]}
      </span>
    </span>
  );
}

export function MessageActivityStatus({
  state,
  className: className,
}: {
  state: MessageActivityState;
  className?: string;
}) {
  return (
    <div className={cn("flex min-w-0 items-center", className)}>
      <div className={cn("inline-flex min-w-0 items-center gap-2 py-1 text-xs font-medium transition-colors", ACTIVITY_TONE_CLASS_MAP[state])}>
        <span className="shrink-0 opacity-75">
          <MessageActivityIcon state={state} />
        </span>
        <MessageActivityLabel state={state} />
        <MessageLoadingDots
          size="sm"
          name={ACTIVITY_SPINNER_MAP[state]}
          className="shrink-0 opacity-70"
        />
      </div>
    </div>
  );
}

export function MessageShell({
  children,
  separated = false,
  className: className,
}: {
  children: ReactNode;
  separated?: boolean;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "w-full min-w-0",
        separated && "border-b border-(--divider-subtle-color)",
        className,
      )}
    >
      {children}
    </div>
  );
}
