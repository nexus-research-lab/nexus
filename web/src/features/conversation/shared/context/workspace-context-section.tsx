import { ShieldCheck } from "lucide-react";

interface WorkspaceContextSectionProps {
  allowed_tool_count: number;
  localized_agent_skill: string;
  permission_mode: string;
  served_room_count: number;
}

export function WorkspaceContextSection({
  allowed_tool_count,
  localized_agent_skill,
  permission_mode,
  served_room_count,
}: WorkspaceContextSectionProps) {
  return (
    <section className="px-4 py-4">
      <div className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-(--text-soft)">
        <ShieldCheck className="h-3.5 w-3.5" />
        Workspace Context
      </div>
      <div className="space-y-1.5">
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-(--text-soft)">技能画像</span>
          <span className="text-[11px] font-medium text-(--text-default)">
            {localized_agent_skill}
          </span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-(--text-soft)">可用工具</span>
          <span className="text-[11px] font-medium text-(--text-default)">
            {allowed_tool_count}
          </span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-(--text-soft)">权限模式</span>
          <span className="text-[11px] font-medium text-(--text-default)">
            {permission_mode}
          </span>
        </div>
        <div className="flex justify-between gap-4">
          <span className="text-[11px] text-(--text-soft)">服务中的 room</span>
          <span className="text-[11px] font-medium text-(--text-default)">
            {served_room_count}
          </span>
        </div>
      </div>
    </section>
  );
}
