/**
 * INPUT: stale 目录刷新失败状态与重试命令。
 * OUTPUT: 不遮挡最后成功目录的可访问错误提示。
 * POS: Home 与 Launcher 复用的目录降级提示视图。
 */
import { CircleAlert } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";

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
    <UiInlineNotice
      action={{
        label: t("sidebar.retry"),
        onClick: onRetry,
      }}
      className={cn("shrink-0", className)}
      icon={<CircleAlert />}
      message={t("sidebar.directory_refresh_failed_impact")}
      title={t("sidebar.directory_refresh_failed_description")}
      tone="danger"
    />
  );
}
