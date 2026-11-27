// INPUT: 会话的活动状态（等你回复 / 工作中）。
// OUTPUT: 状态色点，文字作为读屏标签与悬停提示；avatar 形态压在头像右上角并带页面底色描边，inline 形态排在行尾。
// POS: 侧栏聊天行（avatar，外层需要 relative 定位的头像容器）与首页「正在进行」（inline）共用的状态标记。

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";

import type { SidebarConversationItem } from "./sidebar-conversation-model";

const STATUS_PRESENTATION = {
  waiting: { labelKey: "status.needs_response", dotClassName: "bg-(--primary)" },
  working: { labelKey: "status.working", dotClassName: "bg-(--status-running-soft-text)" },
} as const;

export function ConversationStatusDot({
  placement = "avatar",
  status,
}: {
  placement?: "avatar" | "inline";
  status: SidebarConversationItem["activityStatus"];
}) {
  const { t } = useI18n();
  if (!status) {
    return null;
  }
  const presentation = STATUS_PRESENTATION[status];
  const label = t(presentation.labelKey);
  return (
    <span
      aria-label={label}
      className={cn(
        "rounded-full",
        placement === "avatar"
          ? "pointer-events-none absolute -right-1 -top-1 h-2.5 w-2.5 ring-2 ring-(--background)"
          : "h-2 w-2 shrink-0",
        presentation.dotClassName,
      )}
      role="img"
      title={label}
    />
  );
}
