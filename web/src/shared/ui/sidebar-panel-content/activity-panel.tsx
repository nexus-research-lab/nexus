/**
 * Activity 面板内容
 *
 * 空占位页面，Phase 3 后端就绪后填充实际活动通知流。
 * 当前显示 Bell 图标 + "暂无活动" 文案。
 */

import { memo } from "react";
import { Bell } from "lucide-react";

export const ActivityPanelContent = memo(function ActivityPanelContent() {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-12 text-center">
      {/* 图标容器 */}
      <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-white/50">
        <Bell className="h-5 w-5 text-slate-400" />
      </div>

      {/* 标题 */}
      <p className="text-sm font-semibold text-slate-600">暂无活动</p>

      {/* 说明文字 */}
      <p className="max-w-[180px] text-xs leading-5 text-slate-400">
        当有新的通知、权限请求或系统事件时，会在这里显示。
      </p>
    </div>
  );
});
