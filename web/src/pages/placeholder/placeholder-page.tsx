/**
 * 通用占位页面
 *
 * 用于尚未实现的路由（connectors、scheduled-tasks、channels、pairings、files、settings）。
 * 接受标题和描述作为 props，显示统一的空状态。
 */

import { Construction } from "lucide-react";

import { WorkspaceEntryPage } from "@/shared/ui/workspace/workspace-entry-page";

interface PlaceholderPageProps {
  title: string;
  description?: string;
}

export function PlaceholderPage({
  title,
  description = "此功能即将推出，敬请期待。",
}: PlaceholderPageProps) {
  return (
    <WorkspaceEntryPage
      description={description}
      icon={<Construction className="h-6 w-6 text-slate-900/78" />}
      title={title}
    />
  );
}
