import { Activity } from "lucide-react";

import { Conversation } from "@/types/conversation";
import { formatRelativeTime, truncate } from "@/lib/utils";

interface RoomCollaborationStatusSectionProps {
  active_conversation: Conversation | null;
  localized_runtime_status: string;
  total_member_count: number;
}

export function RoomCollaborationStatusSection({
  active_conversation,
  localized_runtime_status,
  total_member_count,
}: RoomCollaborationStatusSectionProps) {
  const conversation_title = active_conversation?.title?.trim()
    ? truncate(active_conversation.title, 22)
    : "未命名对话";

  return (
    <section className="border-b workspace-divider px-4 py-4">
      <div className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/56">
        <Activity className="h-3.5 w-3.5" />
        Details
      </div>
      <div className="workspace-card rounded-[22px] px-4 py-3">
        <div className="flex items-center justify-between gap-4">
          <div>
            <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/50">当前状态</p>
            <p className="mt-1 text-sm font-semibold text-slate-900/86">{localized_runtime_status}</p>
          </div>
          <div className="text-right">
            <p className="text-[11px] uppercase tracking-[0.12em] text-slate-700/50">成员</p>
            <p className="mt-1 text-sm font-semibold text-slate-900/86">{total_member_count} 人</p>
          </div>
        </div>
      </div>
      <div className="mt-3 space-y-1.5">
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-slate-700/54">最后活动</span>
          <span className="text-[11px] font-medium text-slate-900/84">
            {active_conversation ? formatRelativeTime(active_conversation.last_activity_at) : "未选择"}
          </span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-slate-700/54">当前对话</span>
          <span className="text-[11px] font-medium text-slate-900/84">{conversation_title}</span>
        </div>
      </div>
    </section>
  );
}
