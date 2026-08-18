/**
 * INPUT: stale 目录刷新失败状态与重试命令。
 * OUTPUT: 不遮挡最后成功目录的可访问错误提示。
 * POS: Home 与 Launcher 复用的目录降级提示视图。
 */
import { CircleAlert } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";

interface HomeDirectoryRefreshErrorNoticeProps {
  className?: string;
  onRetry: () => void;
}

export function HomeDirectoryRefreshErrorNotice({
  className,
  onRetry,
}: HomeDirectoryRefreshErrorNoticeProps) {
  const { t } = useI18n();

  return (
    <div
      aria-live="polite"
      className={cn(
        "flex shrink-0 items-center gap-2 rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-panel-background)_96%,var(--destructive))] px-2.5 py-2 text-xs text-(--destructive)",
        className,
      )}
      role="status"
    >
      <CircleAlert aria-hidden="true" className="h-3.5 w-3.5 shrink-0" />
      <span className="min-w-0 flex-1 text-(--text-muted)">
        {t("sidebar.directory_refresh_failed_description")}
      </span>
      <button
        className="shrink-0 font-semibold text-(--destructive) hover:underline"
        onClick={onRetry}
        type="button"
      >
        {t("sidebar.retry")}
      </button>
    </div>
  );
}
