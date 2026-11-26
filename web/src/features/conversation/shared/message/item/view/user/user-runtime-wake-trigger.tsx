// INPUT: 已投影的触发事实（runtime 自唤醒、主窗口定时任务或跟进到点）与紧凑/展开阅读布局。
// OUTPUT: 位于 Assistant 阅读轴上的单行触发分隔条：24px 图标槽、触发文案、可截断的任务摘要与时间。
// POS: 非用户输入的 round 起点；替代用户气泡，不提供编辑、重跑或复制动作。
import { AlarmClock, BellRing, Hourglass } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { MessageAuthorAvatar } from "../../../ui/message-avatar";
import { resolveAssistantMessageLayout } from "../message-reading-layout";
import type { RuntimeWakePresentation } from "./user-message-model";

// 触发来源到图标的完整映射。
const TRIGGER_ICONS = { runtime_wake: BellRing, scheduled_task: AlarmClock, follow_up: Hourglass } as const;

// 触发来源到 DOM 标记的完整映射，供滚动锚点与测试定位。
const TRIGGER_KINDS = { runtime_wake: "runtime-wake", scheduled_task: "scheduled-task", follow_up: "follow-up" } as const;

interface UserRuntimeWakeTriggerProps {
  compact: boolean;
  presentation: RuntimeWakePresentation;
}

export function UserRuntimeWakeTrigger({ compact, presentation }: UserRuntimeWakeTriggerProps) {
  const { t } = useI18n();
  // 复用 Assistant 的外侧节奏与阅读轴，让图标槽与 Agent 头像、正文左缘同列。
  const layout = resolveAssistantMessageLayout(compact);
  const Icon = TRIGGER_ICONS[presentation.source];
  return (
    <div
      className={cn("nexus-chat-message-section w-full", layout.section)}
      data-conversation-round-user-anchor="true"
      data-message-kind={TRIGGER_KINDS[presentation.source]}
    >
      <div className={cn("w-full", layout.inner)}>
        <div className="flex min-h-6 min-w-0 items-center gap-2" role="note">
          <MessageAuthorAvatar>
            <Icon aria-hidden="true" className="h-3.5 w-3.5" />
          </MessageAuthorAvatar>
          <span className={cn("shrink-0", getUiTypographyClassName({ role: "metadata", tone: "default", weight: "medium" }))}>
            {t(presentation.labelKey)}
          </span>
          {presentation.summary ? (
            <UiTooltip label={presentation.summary}>
              <span className={cn("min-w-0 truncate", getUiTypographyClassName({ role: "metadata", tone: "muted" }))}>
                {presentation.summary}
              </span>
            </UiTooltip>
          ) : null}
          <span className={cn("nexus-chat-meta ml-auto shrink-0 pl-2", getUiTypographyClassName({ role: "caption", tone: "soft" }))}>
            {presentation.timestamp}
          </span>
        </div>
      </div>
    </div>
  );
}
