"use client";

/**
 * INPUT: 消息活动状态。
 * OUTPUT: 图标、思考/执行/回复语义与真实逐帧运行的紧凑活动提示。
 * POS: DM/Room 共用的活动呈现；不推导 runtime 状态，也不占用消息正文身份。
 */
import {
  Brain,
  Globe,
  type LucideIcon,
  MessageCircleMore,
  MessageSquareText,
  RefreshCw,
  ShieldAlert,
  Wrench,
} from "lucide-react";
import type { CSSProperties } from "react";
import spinners, { type BrailleSpinnerName } from "unicode-animations";

import { cn } from "@/shared/ui/class-name";

import type { MessageActivityState } from "../activity/message-activity-state";

interface MessageActivityPresentation {
  icon: LucideIcon;
  label: string;
  spinner: BrailleSpinnerName;
  toneClassName: string;
}

const ACTIVITY_PRESENTATION: Record<
  MessageActivityState,
  MessageActivityPresentation
> = {
  compacting: {
    icon: RefreshCw,
    label: "正在压缩上下文",
    spinner: "braille",
    toneClassName: "text-(--text-muted)",
  },
  sending: {
    icon: MessageSquareText,
    label: "正在发送",
    spinner: "braille",
    toneClassName: "text-(--text-muted)",
  },
  thinking: {
    icon: Brain,
    label: "正在思考",
    spinner: "braille",
    toneClassName: "text-(--text-muted)",
  },
  replying: {
    icon: MessageSquareText,
    label: "正在回复",
    spinner: "braille",
    toneClassName: "text-(--text-default)",
  },
  browsing: {
    icon: Globe,
    label: "正在浏览",
    spinner: "braille",
    toneClassName: "text-[color:color-mix(in_srgb,var(--primary)_76%,var(--accent)_24%)]",
  },
  executing: {
    icon: Wrench,
    label: "正在执行",
    spinner: "dna",
    toneClassName: "text-(--primary)",
  },
  waiting_permission: {
    icon: ShieldAlert,
    label: "等待确认",
    spinner: "braille",
    toneClassName: "text-(--warning)",
  },
  waiting_input: {
    icon: MessageCircleMore,
    label: "等待输入",
    spinner: "dna",
    toneClassName: "text-[color:color-mix(in_srgb,var(--primary)_72%,var(--text-strong)_28%)]",
  },
};

export function MessageActivityStatus({
  className,
  state,
}: {
  className?: string;
  state: MessageActivityState;
}) {
  const presentation = ACTIVITY_PRESENTATION[state];
  const ActivityIcon = presentation.icon;
  return (
    <div className={cn("flex min-w-0 items-center", className)}>
      <div className={cn(
        "inline-flex min-w-0 items-center gap-2 py-1 text-xs font-medium transition-colors",
        presentation.toneClassName,
      )}>
        <span className="shrink-0 opacity-75">
          <ActivityIcon className="h-3.5 w-3.5" />
        </span>
        <MessageActivityLabel label={presentation.label} />
        <MessageLoadingDots
          className="shrink-0 opacity-70"
          name={presentation.spinner}
        />
      </div>
    </div>
  );
}

function MessageActivityLabel({ label }: { label: string }) {
  return <span className="truncate">{label}</span>;
}

function MessageLoadingDots({
  className,
  name,
}: {
  className?: string;
  name: BrailleSpinnerName;
}) {
  const spinner = spinners[name];
  const frames = spinner.frames.length > 0 ? spinner.frames : ["·"];
  const trackFrames = [...frames, frames[0]];
  const spinnerWidth = Math.max(
    ...frames.map((frame) => Array.from(frame).length),
  );
  const trackStyle = {
    "--message-activity-spinner-distance": `-${frames.length}em`,
    animation: `nexus-message-activity-frames ${spinner.interval * frames.length}ms steps(${frames.length}, end) infinite`,
  } as CSSProperties;

  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-block h-[1em] overflow-hidden select-none whitespace-pre leading-[1em] text-current align-middle text-[1.4em]",
        className,
      )}
      style={{ width: `${spinnerWidth}ch` }}
    >
      <span
        className="message-activity-spinner-track flex flex-col font-mono leading-none will-change-transform"
        style={trackStyle}
      >
        {trackFrames.map((frame, index) => (
          <span
            className="block h-[1em] shrink-0 leading-[1em]"
            key={`${frame}:${index}`}
          >
            {frame}
          </span>
        ))}
      </span>
    </span>
  );
}
