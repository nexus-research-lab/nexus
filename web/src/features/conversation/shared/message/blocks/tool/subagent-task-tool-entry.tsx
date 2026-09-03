/**
 * INPUT: 子智能体启动工具的精确 ToolUse、统一工具状态与打开任务动作。
 * OUTPUT: 单行、可点击且不重复展示 live progress 的子智能体任务入口。
 * POS: Agent/Task 工具在消息流中的紧凑导航视图；任务详情仍由右侧子智能体面板持有。
 */
import {
  Check,
  Clock3,
  LoaderCircle,
  Sparkles,
  Square,
  X,
  type LucideIcon,
} from "lucide-react";

import { getCompactToolInputSummary } from "../../tool-activity";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import type { ToolUseContent } from "@/types/conversation/message/content";

import type {
  ToolBlockStatus,
  ToolBlockViewModel,
  ToolStatusTone,
} from "./tool-block-types";

const STATUS_ICON: Readonly<Record<
  ToolBlockStatus,
  { icon: LucideIcon; spinning?: boolean }
>> = {
  error: { icon: X },
  pending: { icon: Sparkles },
  rejected: { icon: X },
  superseded: { icon: Square },
  running: { icon: LoaderCircle, spinning: true },
  stopped: { icon: Square },
  success: { icon: Check },
  waiting_permission: { icon: Clock3 },
};

const STATUS_TONE_CLASS: Readonly<Record<ToolStatusTone, string>> = {
  default: "text-(--icon-muted)",
  error: "text-(--destructive)",
  running: "text-(--primary)",
  success: "text-(--success)",
  waiting: "text-(--icon-muted)",
};

interface SubagentTaskToolEntryProps {
  model: ToolBlockViewModel;
  onOpen: () => void;
  toolUse: ToolUseContent;
}

export function SubagentTaskToolEntry({
  model,
  onOpen,
  toolUse,
}: SubagentTaskToolEntryProps) {
  const { t } = useI18n();
  const taskTitle = getCompactToolInputSummary(toolUse.input)
    ?? model.collapsedDetailText
    ?? model.toolTitle;
  const statusIcon = STATUS_ICON[model.status];
  const StatusIcon = statusIcon.icon;
  const accessibleLabel = [
    taskTitle,
    model.statusText,
    t("subagents.open_task"),
  ].join(" · ");

  return (
    <button
      aria-label={accessibleLabel}
      className="group/subagent-task inline-flex h-9 w-60 max-w-full items-center gap-2 radius-control-sm border border-(--divider-subtle-color) bg-transparent px-1.5 text-left text-sm font-medium text-(--text-muted) transition-[background,border-color,color] duration-(--motion-duration-fast) hover:border-(--divider-strong-color) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]"
      data-subagent-task-tool-entry
      onClick={onOpen}
      title={`${taskTitle} · ${model.statusText}`}
      type="button"
    >
      <UiSeededAvatar
        data-subagent-task-avatar
        seed={toolUse.id}
        size="2xs"
      />
      <span className="min-w-0 flex-1 truncate">{taskTitle}</span>
      <span
        aria-hidden="true"
        className={cn(
          "flex h-4 w-4 shrink-0 items-center justify-center",
          STATUS_TONE_CLASS[model.statusTone],
        )}
        data-subagent-task-status={model.status}
        title={model.statusText}
      >
        <StatusIcon
          className={statusIcon.spinning
            ? getUiSpinnerClassName({ size: "sm" })
            : "h-3.5 w-3.5"}
        />
      </span>
    </button>
  );
}
