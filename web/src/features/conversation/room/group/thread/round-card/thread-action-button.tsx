// INPUT: 当前 Thread 展开态与切换命令。
// OUTPUT: 共享微型 Button 投影的 Thread 打开/关闭动作。
// POS: Room Agent 执行条动作；不拥有 Thread 内容或展开状态。
import type { MouseEventHandler } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";

interface ThreadActionButtonProps {
  active: boolean;
  onClick: MouseEventHandler<HTMLButtonElement>;
}

export function ThreadActionButton({
  active,
  onClick,
}: ThreadActionButtonProps) {
  const { t } = useI18n();
  const actionLabel = t(active ? "room.thread_close" : "room.thread_open");
  return (
    <UiButton
      aria-label={actionLabel}
      aria-pressed={active}
      data-room-agent-action="thread"
      onClick={onClick}
      size="2xs"
      title={actionLabel}
      variant="text"
    >
      {t("room.thread_label")}
    </UiButton>
  );
}
