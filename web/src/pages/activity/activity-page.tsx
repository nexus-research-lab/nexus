/**
 * 活动中心页面（/activity）
 *
 * 空占位页面，Phase 3 后端就绪后填充实际内容。
 * 使用 WorkspaceEntryPage 显示空状态。
 */

import { Activity } from "lucide-react";

import { WorkspaceEntryPage } from "@/shared/ui/workspace-entry-page";

export function ActivityPage() {
  return (
    <WorkspaceEntryPage
      description="活动中心即将推出。当有新的通知、权限请求或系统事件时，会在这里集中展示。"
      icon={<Activity className="h-6 w-6 text-slate-900/78" />}
      title="活动中心"
    />
  );
}
