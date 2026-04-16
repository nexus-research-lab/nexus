/**
 * =====================================================
 * @File   : message-primitives.tsx
 * @Date   : 2026-04-05 15:26
 * @Author : leemysw
 * 2026-04-05 15:26   Create
 * =====================================================
 */

"use client";

import { ButtonHTMLAttributes, ReactNode, useEffect, useRef, useState } from "react";
import { Brain, Globe, MessageCircleMore, MessageSquareText, ShieldAlert, Wrench } from "lucide-react";
import spinners, { type BrailleSpinnerName } from "unicode-animations";

import { usePrefersReducedMotion } from "@/hooks/ui/use-prefers-reduced-motion";
import { cn, get_icon_avatar_src } from "@/lib/utils";

type MessageAvatarSize = "full" | "compact";
type MessageActionTone = "default" | "success" | "danger";
type MessageLoadingDotsSize = "sm" | "md";
export type MessageActivityState =
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
  success: "text-green-500 hover:bg-emerald-500/10 hover:text-emerald-500",
  danger: "hover:bg-rose-500/10 hover:text-rose-500",
};

function get_first_visible_spinner_frame_index(name: BrailleSpinnerName): number {
  const first_visible_frame_index = spinners[name].frames.findIndex(
    (frame) => frame.replace(/⠀/g, "").length > 0,
  );
  return first_visible_frame_index >= 0 ? first_visible_frame_index : 0;
}

const ACTIVITY_LABEL_MAP: Record<MessageActivityState, string> = {
  thinking: "正在思考",
  replying: "正在回复",
  browsing: "正在浏览",
  executing: "正在执行",
  waiting_permission: "等待确认",
  waiting_input: "等待输入",
};

const ACTIVITY_TONE_CLASS_MAP: Record<MessageActivityState, string> = {
  thinking: "text-(--text-muted)",
  replying: "text-(--text-default)",
  browsing: "text-cyan-600",
  executing: "text-indigo-600",
  waiting_permission: "text-amber-700",
  waiting_input: "text-violet-600",
};

const ACTIVITY_SPINNER_MAP: Record<MessageActivityState, BrailleSpinnerName> = {
  thinking: "braille",
  replying: "dna",
  browsing: "braille",
  executing: "dna",
  waiting_permission: "braille",
  waiting_input: "dna",
};

export function MessageAvatar({
  avatar_url,
  children,
  size = "full",
  class_name,
}: {
  avatar_url?: string | null;
  children?: ReactNode;
  size?: MessageAvatarSize;
  class_name?: string;
}) {
  const resolved_avatar_url = get_icon_avatar_src(avatar_url);
  const avatar_shell_class_name = cn(
    "overflow-hidden border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)",
    "transition-[transform,box-shadow,border-color] duration-(--motion-duration-fast) ease-out",
    "motion-safe:hover:-translate-y-[1px] motion-safe:hover:scale-[1.06]",
    "motion-safe:hover:border-(--surface-interactive-active-border)",
    "motion-safe:hover:shadow-[0_10px_22px_rgba(15,23,42,0.14)]",
    AVATAR_SIZE_CLASS_MAP[size],
    class_name,
  );

  if (resolved_avatar_url) {
    return (
      <div className={avatar_shell_class_name}>
        <img
          src={resolved_avatar_url}
          alt=""
          className="h-full w-full object-cover transition-transform duration-(--motion-duration-fast) ease-out motion-safe:hover:scale-[1.04]"
        />
      </div>
    );
  }

  return (
    <div
      className={cn(
        avatar_shell_class_name,
        "flex items-center justify-center text-(--surface-avatar-foreground)",
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
        "rounded-lg p-1 text-(--icon-default) transition-colors duration-(--motion-duration-fast) focus-visible:ring-2 focus-visible:ring-primary/50",
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
  size: _size = "md",
  class_name,
  name = "braille",
}: {
  size?: MessageLoadingDotsSize;
  class_name?: string;
  name?: BrailleSpinnerName;
}) {
  const prefers_reduced_motion = usePrefersReducedMotion();
  const spinner = spinners[name];
  const first_visible_frame_index = get_first_visible_spinner_frame_index(name);
  const [frame_index, setFrameIndex] = useState(first_visible_frame_index);

  useEffect(() => {
    const current_spinner = spinners[name];
    setFrameIndex(first_visible_frame_index);

    if (prefers_reduced_motion) {
      return;
    }

    // 帧动画完全由包内 interval 驱动，切换 spinner 时直接重置，避免旧帧残留。
    const timer = window.setInterval(() => {
      setFrameIndex((current_index) => (current_index + 1) % current_spinner.frames.length);
    }, current_spinner.interval);

    return () => {
      window.clearInterval(timer);
    };
  }, [first_visible_frame_index, name, prefers_reduced_motion]);

  const spinner_width = Math.max(
    ...spinner.frames.map((frame) => Array.from(frame).length),
  );
  const current_frame = prefers_reduced_motion
    ? spinner.frames[first_visible_frame_index] ?? spinner.frames[0]
    : spinner.frames[frame_index] ?? spinner.frames[first_visible_frame_index] ?? spinner.frames[0];

  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-grid h-[1em] select-none place-items-center whitespace-pre leading-[1em] text-current align-middle text-[1.4em]",
        class_name,
      )}
      style={{ width: `${spinner_width}ch` }}
    >
      <span
        className="block font-mono leading-none"
        style={{ transform: "translateY(0.02em)" }}
      >
        {current_frame}
      </span>
    </span>
  );
}

function MessageActivityIcon({ state }: { state: MessageActivityState }) {
  switch (state) {
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
  const prefers_reduced_motion = usePrefersReducedMotion();
  const shimmer_ref = useRef<HTMLSpanElement | null>(null);

  useEffect(() => {
    const element = shimmer_ref.current;
    if (!element || prefers_reduced_motion || typeof element.animate !== "function") {
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
  }, [prefers_reduced_motion, state]);

  if (prefers_reduced_motion) {
    return <span className="truncate">{ACTIVITY_LABEL_MAP[state]}</span>;
  }

  return (
    <span
      className="relative inline-flex min-w-0 truncate text-current"
    >
      <span className="truncate">{ACTIVITY_LABEL_MAP[state]}</span>
      <span
        ref={shimmer_ref}
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
  class_name,
}: {
  state: MessageActivityState;
  class_name?: string;
}) {
  return (
    <div className={cn("flex min-w-0 items-center", class_name)}>
      <div className={cn("inline-flex min-w-0 items-center gap-2 py-1 text-xs font-medium transition-colors", ACTIVITY_TONE_CLASS_MAP[state])}>
        <span className="shrink-0 opacity-75">
          <MessageActivityIcon state={state} />
        </span>
        <MessageActivityLabel state={state} />
        <MessageLoadingDots
          size="sm"
          name={ACTIVITY_SPINNER_MAP[state]}
          class_name="shrink-0 opacity-70"
        />
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
        separated && "border-b border-(--divider-subtle-color)",
        class_name,
      )}
    >
      {children}
    </div>
  );
}
