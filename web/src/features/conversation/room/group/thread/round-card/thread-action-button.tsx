import type { MouseEventHandler } from "react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";

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
    <button
      aria-label={actionLabel}
      className={cn(
        "inline-flex h-6 items-center rounded-md px-2 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50",
        active
          ? "bg-(--status-info-soft-bg) text-(--status-info-soft-text)"
          : "text-(--text-muted) hover:bg-(--interaction-hover-background) hover:text-(--text-default)",
      )}
      data-room-agent-action="thread"
      onClick={onClick}
      title={actionLabel}
      type="button"
    >
      {t("room.thread_label")}
    </button>
  );
}
