"use client";

/**
 * INPUT: 消息活动状态，以及（可选）轮次计时源。
 * OUTPUT: 运行中为“动效 + 趣味动词/真实阶段 + 已用时长”，等待态为图标 + 静态文案；与工具行等高。
 * POS: DM/Room 共用的单行活动呈现；不推导 runtime 状态，也不把即时标签伪装成正式回复。
 */
import {
  Brain,
  Globe,
  type LucideIcon,
  MessageCircleMore,
  MessageCircle,
  RefreshCw,
  Shield,
  Wrench,
} from "lucide-react";
import { useI18n, type I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import {
  LoadingOrb,
  type LoadingOrbVariant,
} from "@/shared/ui/feedback/loading-orb";

import type { MessageActivityState } from "../activity/message-activity-state";
import { useActivityClock, type ActivityClockReading } from "./message-activity-clock";

export const ROOM_RESULT_ACTIVITY_ALIGNMENT_CLASS_NAME =
  "px-0";

interface MessageActivityPresentation {
  icon: LucideIcon;
  indicator: LoadingOrbVariant | null;
  labelKey: TranslationKey;
  /** 是否显示轮次已用时长；只给“模型在干活”的阶段，等待用户的阶段不计时。 */
  timed: boolean;
  toneClassName: string;
  /** playful：泛化的生成阶段改用轮换趣味动词；literal：工具、压缩、等待等关键阶段保留真实文案。 */
  voice: "playful" | "literal";
}

const ACTIVITY_PRESENTATION: Record<
  MessageActivityState,
  MessageActivityPresentation
> = {
  compacting: {
    icon: RefreshCw,
    indicator: "active",
    labelKey: "message.activity_compacting",
    timed: true,
    toneClassName: "text-(--text-muted)",
    voice: "literal",
  },
  sending: {
    icon: MessageCircle,
    indicator: "active",
    labelKey: "message.activity_sending",
    timed: true,
    toneClassName: "text-(--text-muted)",
    voice: "playful",
  },
  thinking: {
    icon: Brain,
    indicator: "active",
    labelKey: "message.activity_thinking",
    timed: true,
    toneClassName: "text-(--text-muted)",
    voice: "playful",
  },
  replying: {
    icon: MessageCircle,
    indicator: "active",
    labelKey: "message.activity_replying",
    timed: true,
    toneClassName: "text-(--text-default)",
    voice: "playful",
  },
  browsing: {
    icon: Globe,
    indicator: "active",
    labelKey: "message.activity_browsing",
    timed: true,
    toneClassName: "text-[color:color-mix(in_srgb,var(--primary)_76%,var(--accent)_24%)]",
    voice: "literal",
  },
  executing: {
    icon: Wrench,
    indicator: "active",
    labelKey: "message.activity_executing",
    timed: true,
    toneClassName: "text-(--primary)",
    voice: "literal",
  },
  waiting_permission: {
    icon: Shield,
    indicator: null,
    labelKey: "message.activity_waiting_permission",
    timed: false,
    toneClassName: "text-(--text-muted)",
    voice: "literal",
  },
  waiting_input: {
    icon: MessageCircleMore,
    indicator: "active",
    labelKey: "message.activity_waiting_input",
    timed: false,
    toneClassName: "text-[color:color-mix(in_srgb,var(--primary)_72%,var(--text-strong)_28%)]",
    voice: "literal",
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
  const presentation = ACTIVITY_PRESENTATION[state];
  const clock = useActivityClock(presentation.timed);
  const narration = label?.trim();
  // 模型即时旁白是真实信息，优先于趣味动词；没有轮次计时源时退回静态阶段文案。
  const verbKey = !narration && clock && presentation.voice === "playful"
    ? clock.verbKey
    : null;
  return (
    <MessageActivityStatus
      className={className}
      elapsed={clock ? formatElapsed(clock, t) : null}
      label={narration || t(verbKey ?? presentation.labelKey)}
      labelKey={verbKey}
      stableSlot={stableSlot}
      state={state}
      uniformTone={uniformTone}
    />
  );
}

function formatElapsed(
  { elapsedSeconds }: ActivityClockReading,
  t: I18nContextValue["t"],
): string {
  const minutes = Math.floor(elapsedSeconds / 60);
  const seconds = elapsedSeconds % 60;
  return minutes > 0
    ? t("message.activity_elapsed_minutes", { minutes, seconds })
    : t("message.activity_elapsed_seconds", { seconds });
}

export function MessageActivityStatus({
  className,
  elapsed = null,
  label,
  labelKey = null,
  stableSlot = false,
  state,
  uniformTone = false,
}: {
  className?: string;
  /** 已格式化的已用时长；存在时以前置动效替换阶段图标，形成“动效 + 文案 + 时长”的运行行。 */
  elapsed?: string | null;
  label: string;
  /** 轮换动词的身份；换词时据此做一次淡入交接。 */
  labelKey?: string | null;
  stableSlot?: boolean;
  state: MessageActivityState;
  uniformTone?: boolean;
}) {
  const presentation = ACTIVITY_PRESENTATION[state];
  const ActivityIcon = presentation.icon;
  const running = elapsed !== null && presentation.indicator !== null;
  return (
    <div
      className={cn(
        "flex h-7 min-w-0 items-center px-0",
        className,
      )}
      data-message-activity-stable-slot={stableSlot || undefined}
    >
      <div className={cn(
        "inline-flex min-w-0 items-center gap-1.5 text-sm transition-colors",
        stableSlot ? "font-normal leading-5" : "font-medium leading-5",
        uniformTone ? "text-primary" : presentation.toneClassName,
      )}>
        <span
          className="flex h-5 w-5 shrink-0 items-center justify-center opacity-75"
          data-message-activity-icon
        >
          {running && presentation.indicator ? (
            <LoadingOrb variant={presentation.indicator} />
          ) : (
            <ActivityIcon className="h-3.5 w-3.5" />
          )}
        </span>
        {/* 阶段切换或换词时以 key 重挂载做一次淡入交接；同阶段旁白逐字更新不重播，只变透明度、不位移。 */}
        <span className="ui-status-swap min-w-0 truncate" key={labelKey ?? state}>{label}</span>
        {running ? (
          <span
            className="shrink-0 font-normal tabular-nums text-(--text-soft)"
            data-message-activity-elapsed
          >
            {elapsed}
          </span>
        ) : presentation.indicator ? (
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
