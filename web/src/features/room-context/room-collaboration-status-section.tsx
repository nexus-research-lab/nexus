import { Activity } from "lucide-react";

import { Conversation } from "@/types/conversation";
import { formatRelativeTime, truncate } from "@/lib/utils";

interface RoomCollaborationStatusSectionProps {
  activeConversation: Conversation | null;
  localizedAgentSkill: string;
  localizedRuntimeStatus: string;
  modelName: string;
  totalRoomCount: number;
}

export function RoomCollaborationStatusSection({
  activeConversation,
  localizedAgentSkill,
  localizedRuntimeStatus,
  modelName,
  totalRoomCount,
}: RoomCollaborationStatusSectionProps) {
  const currentRoomName = activeConversation?.title?.trim() || "未命名房间";

  return (
    <section className="border-b workspace-divider px-4 py-4">
      <div className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/56">
        <Activity className="h-3.5 w-3.5" />
        Collaboration Status
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div className="workspace-card rounded-[22px] px-3 py-2">
          <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/50">参与房间</p>
          <p className="mt-1 text-sm font-semibold text-slate-900/86">{totalRoomCount}</p>
        </div>
        <div className="workspace-card rounded-[22px] px-3 py-2">
          <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/50">当前状态</p>
          <p className="mt-1 text-sm font-semibold text-slate-900/86">{localizedRuntimeStatus}</p>
        </div>
      </div>
      <div className="mt-3 space-y-1.5">
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-slate-700/54">当前 room</span>
          <span className="text-[11px] font-medium text-slate-900/84">{truncate(currentRoomName, 18)}</span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-slate-700/54">模型</span>
          <span className="text-[11px] font-medium text-slate-900/84">{modelName}</span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-slate-700/54">成员角色</span>
          <span className="text-[11px] font-medium text-slate-900/84">{localizedAgentSkill}</span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-slate-700/54">最后活动</span>
          <span className="text-[11px] font-medium text-slate-900/84">
            {activeConversation ? formatRelativeTime(activeConversation.last_activity_at) : "未选择"}
          </span>
        </div>
      </div>
    </section>
  );
}
