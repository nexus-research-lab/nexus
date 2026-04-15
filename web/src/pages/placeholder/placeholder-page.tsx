/**
 * 通用占位页面
 *
 * 用于尚未实现的路由（connectors、scheduled-tasks、channels、pairings、files、settings）。
 * 接受标题和描述作为 props，显示统一的空状态。
 */

import { Construction } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceEntryPage } from "@/shared/ui/workspace/frame/workspace-entry-page";

interface PlaceholderPageProps {
  title: string;
  description?: string;
}

export function PlaceholderPage({
  title,
  description,
}: PlaceholderPageProps) {
  const { t } = useI18n();

  return (
    <WorkspaceEntryPage
      description={description ?? t("common.coming_soon")}
      icon={<Construction className="h-6 w-6 text-(--text-strong)" />}
      title={title}
    />
  );
}
