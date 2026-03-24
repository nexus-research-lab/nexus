import { ShieldCheck } from "lucide-react";

interface RoomWorkspaceContextSectionProps {
  allowed_tool_count: number;
  localized_agent_skill: string;
  permission_mode: string;
  served_room_count: number;
}

export function RoomWorkspaceContextSection({
  allowed_tool_count,
  localized_agent_skill,
  permission_mode,
  served_room_count,
}: RoomWorkspaceContextSectionProps) {
  return (
    <section className="px-4 py-4">
      <div className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/56">
        <ShieldCheck className="h-3.5 w-3.5" />
        Workspace Context
      </div>
      <div className="space-y-1.5">
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-slate-700/54">技能画像</span>
          <span className="text-[11px] font-medium text-slate-900/84">
            {localized_agent_skill}
          </span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-slate-700/54">可用工具</span>
          <span className="text-[11px] font-medium text-slate-900/84">
            {allowed_tool_count}
          </span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-slate-700/54">权限模式</span>
          <span className="text-[11px] font-medium text-slate-900/84">
            {permission_mode}
          </span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-slate-700/54">服务中的 room</span>
          <span className="text-[11px] font-medium text-slate-900/84">
            {served_room_count}
          </span>
        </div>
      </div>
    </section>
  );
}
