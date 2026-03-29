/**
 * 定时任务页面
 *
 * 包含页面头部（标题 + 创建按钮）和空状态展示。
 * 点击「创建任务」按钮弹出 CreateTaskDialog 对话框。
 */

import { useState } from "react";
import { Clock, Plus } from "lucide-react";

import { AppStage } from "@/shared/ui/app-stage";
import { WorkspacePageFrame } from "@/shared/ui/workspace-page-frame";

import { CreateTaskDialog } from "./create-task-dialog";

export function ScheduledTasksPage() {
  const [is_dialog_open, set_is_dialog_open] = useState(false);

  return (
    <AppStage>
      <WorkspacePageFrame>
        {/* 页面头部 */}
        <div className="flex items-center justify-between pb-6">
          <div className="flex items-center gap-3">
            <div className="workspace-chip flex h-10 w-10 items-center justify-center rounded-[14px]">
              <Clock className="h-5 w-5 text-slate-900/78" />
            </div>
            <div>
              <h1 className="text-lg font-bold tracking-[-0.02em] text-slate-950/90">
                定时任务
              </h1>
              <p className="text-sm text-slate-700/60">管理自动化定时任务</p>
            </div>
          </div>
          <button
            className="flex items-center gap-1.5 rounded-[14px] workspace-chip px-4 py-2 text-sm font-semibold text-slate-700 shadow-[0_10px_20px_rgba(110,117,142,0.10)] transition-all hover:bg-white/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/40"
            onClick={() => set_is_dialog_open(true)}
            type="button"
          >
            <Plus className="h-4 w-4" />
            创建任务
          </button>
        </div>

        {/* 空状态 */}
        <div className="relative flex min-h-0 flex-1 items-center justify-center">
          <div className="flex flex-col items-center text-center">
            <div className="workspace-chip flex h-14 w-14 items-center justify-center rounded-[20px]">
              <Clock className="h-6 w-6 text-slate-900/78" />
            </div>
            <h2 className="mt-5 text-xl font-bold tracking-[-0.03em] text-slate-950/90">
              暂无定时任务
            </h2>
            <p className="mt-2 max-w-sm text-sm leading-6 text-slate-700/60">
              创建你的第一个定时任务，让 Agent 自动执行重复性工作
            </p>
          </div>
        </div>

        {/* 创建任务对话框 */}
        <CreateTaskDialog
          is_open={is_dialog_open}
          on_close={() => set_is_dialog_open(false)}
        />
      </WorkspacePageFrame>
    </AppStage>
  );
}
