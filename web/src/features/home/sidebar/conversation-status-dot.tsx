// INPUT: 会话的活动状态（等你回复 / 工作中）。
// OUTPUT: 压在头像右上角的状态色点，带一圈页面底色描边，文字作为读屏标签与悬停提示。
// POS: 侧栏聊天行与首页「正在进行」共用的状态标记；外层需要 relative 定位的头像容器。

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";

import type { SidebarConversationItem } from "./sidebar-conversation-model";

const STATUS_PRESENTATION = {
  waiting: { labelKey: "status.needs_response", dotClassName: "bg-(--primary)" },
  working: { labelKey: "status.working", dotClassName: "bg-(--status-running-soft-text)" },
} as const;

export function ConversationStatusDot({
  status,
}: {
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
        "pointer-events-none absolute -right-1 -top-1 h-2.5 w-2.5 rounded-full ring-2 ring-(--background)",
        presentation.dotClassName,
      )}
      role="img"
      title={label}
    />
  );
}
