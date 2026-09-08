// INPUT: 当前 Agent 展示名、Thread 展开态与切换命令。
// OUTPUT: 共享 xs Button 投影的具名 Thread 展开/关闭动作。
// POS: Room Agent 执行条动作；不拥有 Thread 内容或展开状态。
import type { MouseEventHandler } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { getAgentDisplayName } from "@/lib/agent-display-name";
import { UiButton } from "@/shared/ui/button/button";

interface ThreadActionButtonProps {
  active: boolean;
  agentName: string;
  onClick: MouseEventHandler<HTMLButtonElement>;
}

export function ThreadActionButton({
  active,
  agentName,
  onClick,
}: ThreadActionButtonProps) {
  const { t } = useI18n();
  const actionLabel = t(active ? "room.thread_action_close" : "room.thread_action_open", {
    name: getAgentDisplayName(agentName, t),
  });
  return (
    <UiButton
      aria-label={actionLabel}
      aria-expanded={active}
      data-room-agent-action="thread"
      onClick={onClick}
      size="xs"
      title={actionLabel}
      variant="text"
    >
      {t("room.thread_label")}
    </UiButton>
  );
}
