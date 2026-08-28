"use client";

/**
 * INPUT: 消息活动状态。
 * OUTPUT: 图标、逐帧提示、可替换通用状态的自然语言活动标签，以及 Room 公区统一主色投影。
 * POS: DM/Room 共用的单行活动呈现；不推导 runtime 状态，也不把即时标签伪装成正式回复。
 */
import {
  Brain,
  Globe,
  type LucideIcon,
  MessageCircleMore,
  MessageSquareText,
  RefreshCw,
  Shield,
  Wrench,
} from "lucide-react";
import type { CSSProperties } from "react";
import spinners, { type BrailleSpinnerName } from "unicode-animations";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";

import type { MessageActivityState } from "../activity/message-activity-state";
import "./message-activity-status.css";

interface MessageActivityPresentation {
  icon: LucideIcon;
  labelKey: TranslationKey;
  spinner: BrailleSpinnerName | null;
  toneClassName: string;
}

const ACTIVITY_PRESENTATION: Record<
  MessageActivityState,
  MessageActivityPresentation
> = {
  compacting: {
    icon: RefreshCw,
    labelKey: "message.activity_compacting",
    spinner: "braille",
    toneClassName: "text-(--text-muted)",
  },
  sending: {
    icon: MessageSquareText,
    labelKey: "message.activity_sending",
    spinner: "braille",
    toneClassName: "text-(--text-muted)",
  },
  thinking: {
    icon: Brain,
    labelKey: "message.activity_thinking",
    spinner: "braille",
    toneClassName: "text-(--text-muted)",
  },
  replying: {
    icon: MessageSquareText,
    labelKey: "message.activity_replying",
    spinner: "braille",
    toneClassName: "text-(--text-default)",
  },
  browsing: {
    icon: Globe,
    labelKey: "message.activity_browsing",
    spinner: "braille",
    toneClassName: "text-[color:color-mix(in_srgb,var(--primary)_76%,var(--accent)_24%)]",
  },
  executing: {
    icon: Wrench,
    labelKey: "message.activity_executing",
    spinner: "dna",
    toneClassName: "text-(--primary)",
  },
  waiting_permission: {
    icon: Shield,
    labelKey: "message.activity_waiting_permission",
    spinner: null,
    toneClassName: "text-(--text-muted)",
  },
  waiting_input: {
    icon: MessageCircleMore,
    labelKey: "message.activity_waiting_input",
    spinner: "dna",
    toneClassName: "text-[color:color-mix(in_srgb,var(--primary)_72%,var(--text-strong)_28%)]",
  },
};

export function LocalizedMessageActivityStatus({
  className,
  label,
  state,
  uniformTone = false,
}: {
  className?: string;
  label?: string | null;
  state: MessageActivityState;
  uniformTone?: boolean;
}) {
  const { t } = useI18n();
  return (
    <MessageActivityStatus
      className={className}
      label={label?.trim() || t(ACTIVITY_PRESENTATION[state].labelKey)}
      state={state}
      uniformTone={uniformTone}
    />
  );
}

export function MessageActivityStatus({
  className,
  label,
  state,
  uniformTone = false,
}: {
  className?: string;
  label: string;
  state: MessageActivityState;
  uniformTone?: boolean;
}) {
  const presentation = ACTIVITY_PRESENTATION[state];
  const ActivityIcon = presentation.icon;
  return (
    <div className={cn("flex min-w-0 items-center", className)}>
      <div className={cn(
        "inline-flex min-w-0 items-center gap-2 py-1 text-sm font-medium transition-colors",
        uniformTone ? "text-primary" : presentation.toneClassName,
      )}>
        <span className="shrink-0 opacity-75">
          <ActivityIcon className="h-3.5 w-3.5" />
        </span>
        <MessageActivityLabel label={label} />
        {presentation.spinner ? (
          <MessageLoadingDots
            className="shrink-0 opacity-70"
            name={presentation.spinner}
          />
        ) : null}
      </div>
    </div>
  );
}

function MessageActivityLabel({ label }: { label: string }) {
  return (
    <span className="message-activity-label-flow truncate">
      {label}
    </span>
  );
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
        "inline-block h-[1em] translate-y-[2px] overflow-hidden select-none whitespace-pre leading-[1em] text-current text-[1.4em]",
        className,
      )}
      style={{ width: `${spinnerWidth}ch` }}
    >
      <span
        className="message-activity-spinner-track flex flex-col font-mono leading-none"
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
