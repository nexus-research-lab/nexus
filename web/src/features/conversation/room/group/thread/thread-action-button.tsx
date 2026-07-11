import type { MouseEventHandler } from "react";

import { cn } from "@/lib/utils";
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
  const label = t(active ? "room.thread_close" : "room.thread_open");
  return (
    <button
      aria-label={label}
      className={cn(
        "rounded-md border px-2 py-1 text-[11px] font-medium transition-colors",
        active
          ? "border-(--status-info-soft-border) bg-(--status-info-soft-bg) text-(--status-info-soft-text)"
          : "border-(--divider-subtle-color) bg-transparent text-(--text-muted) hover:bg-(--interaction-hover-background) hover:text-(--text-default)",
      )}
      onClick={onClick}
      title={label}
      type="button"
    >
      {label}
    </button>
  );
}
