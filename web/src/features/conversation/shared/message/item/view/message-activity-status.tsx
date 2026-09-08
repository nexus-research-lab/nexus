"use client";

/**
 * INPUT: 消息活动状态。
 * OUTPUT: 图标、静态文案、共享局部指示器，以及可与工具行等高的稳定活动槽位。
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
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import {
  LoadingOrb,
  type LoadingOrbVariant,
} from "@/shared/ui/feedback/loading-orb";

import type { MessageActivityState } from "../activity/message-activity-state";

interface MessageActivityPresentation {
  icon: LucideIcon;
  indicator: LoadingOrbVariant | null;
  labelKey: TranslationKey;
  toneClassName: string;
}

const ACTIVITY_PRESENTATION: Record<
  MessageActivityState,
  MessageActivityPresentation
> = {
  compacting: {
    icon: RefreshCw,
    indicator: "active",
    labelKey: "message.activity_compacting",
    toneClassName: "text-(--text-muted)",
  },
  sending: {
    icon: MessageSquareText,
    indicator: "active",
    labelKey: "message.activity_sending",
    toneClassName: "text-(--text-muted)",
  },
  thinking: {
    icon: Brain,
    indicator: "active",
    labelKey: "message.activity_thinking",
    toneClassName: "text-(--text-muted)",
  },
  replying: {
    icon: MessageSquareText,
    indicator: "active",
    labelKey: "message.activity_replying",
    toneClassName: "text-(--text-default)",
  },
  browsing: {
    icon: Globe,
    indicator: "active",
    labelKey: "message.activity_browsing",
    toneClassName: "text-[color:color-mix(in_srgb,var(--primary)_76%,var(--accent)_24%)]",
  },
  executing: {
    icon: Wrench,
    indicator: "active",
    labelKey: "message.activity_executing",
    toneClassName: "text-(--primary)",
  },
  waiting_permission: {
    icon: Shield,
    indicator: null,
    labelKey: "message.activity_waiting_permission",
    toneClassName: "text-(--text-muted)",
  },
  waiting_input: {
    icon: MessageCircleMore,
    indicator: "active",
    labelKey: "message.activity_waiting_input",
    toneClassName: "text-[color:color-mix(in_srgb,var(--primary)_72%,var(--text-strong)_28%)]",
  },
};

export function LocalizedMessageActivityStatus({
  className,
  label,
  stableSlot = false,
  state,
  uniformTone = false,
}: {
  className?: string;
  label?: string | null;
  stableSlot?: boolean;
  state: MessageActivityState;
  uniformTone?: boolean;
}) {
  const { t } = useI18n();
  return (
    <MessageActivityStatus
      className={className}
      label={label?.trim() || t(ACTIVITY_PRESENTATION[state].labelKey)}
      stableSlot={stableSlot}
      state={state}
      uniformTone={uniformTone}
    />
  );
}

export function MessageActivityStatus({
  className,
  label,
  stableSlot = false,
  state,
  uniformTone = false,
}: {
  className?: string;
  label: string;
  stableSlot?: boolean;
  state: MessageActivityState;
  uniformTone?: boolean;
}) {
  const presentation = ACTIVITY_PRESENTATION[state];
  const ActivityIcon = presentation.icon;
  return (
    <div
      className={cn(
        "flex min-w-0 items-center px-1.5",
        stableSlot && "h-7",
        className,
      )}
      data-message-activity-stable-slot={stableSlot || undefined}
    >
      <div className={cn(
        "inline-flex min-w-0 items-center gap-1.5 text-sm transition-colors",
        stableSlot ? "py-0 font-normal leading-5" : "py-1 font-medium",
        uniformTone ? "text-primary" : presentation.toneClassName,
      )}>
        <span
          className="flex h-5 w-5 shrink-0 items-center justify-center opacity-75"
          data-message-activity-icon
        >
          <ActivityIcon className="h-3.5 w-3.5" />
        </span>
        <span className="min-w-0 truncate">{label}</span>
        {presentation.indicator ? (
          <span
            className="flex h-5 w-3 shrink-0 items-center justify-center opacity-70"
            data-message-activity-indicator
          >
            <LoadingOrb variant={presentation.indicator} />
          </span>
        ) : null}
      </div>
    </div>
  );
}
