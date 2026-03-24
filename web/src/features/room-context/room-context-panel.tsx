"use client";

import { HOME_AGENT_INSPECTOR_WIDTH_CLASS } from "@/lib/home-layout";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { AgentCostSummary, SessionCostSummary } from "@/types/cost";
import { Session } from "@/types/session";
import { TodoItem } from "@/types/todo";

import { RoomCollaborationStatusSection } from "./room-collaboration-status-section";
import { RoomMemberSummaryCard } from "./room-member-summary-card";
import { RoomProgressSection } from "./room-progress-section";
import { RoomUsageSection } from "./room-usage-section";
import { RoomWorkspaceContextSection } from "./room-workspace-context-section";

export interface RoomContextPanelProps {
  agent: Agent;
  sessions: Session[];
  active_session: Session | null;
  todos: TodoItem[];
  is_session_busy: boolean;
  session_cost_summary: SessionCostSummary;
  agent_cost_summary: AgentCostSummary;
  on_edit_agent: (agent_id: string) => void;
}

export function RoomContextPanel({
  agent,
  sessions,
  active_session,
  todos,
  is_session_busy,
  session_cost_summary,
  agent_cost_summary,
  on_edit_agent,
}: RoomContextPanelProps) {
  const runtime_status = is_session_busy ? "Running" : active_session?.is_active === false ? "Idle" : "Active";
  const model_name = agent.options.model || "inherit";
  const localized_runtime_status =
    runtime_status === "Running" ? "协作中" : runtime_status === "Idle" ? "待命" : "在线";
  const localized_agent_skill = agent.options.skills_enabled ? "技能已启用" : "通用成员";
  const current_conversation = (active_session as Conversation | null) ?? null;

  return (
    <aside className={`flex min-h-0 flex-col bg-transparent ${HOME_AGENT_INSPECTOR_WIDTH_CLASS}`}>
      <div className="flex-1 overflow-y-auto scrollbar-hide">
        <RoomMemberSummaryCard
          agent={agent}
          localized_runtime_status={localized_runtime_status}
          model_name={model_name}
          on_edit_agent={on_edit_agent}
          runtime_status={runtime_status}
        />

        <RoomCollaborationStatusSection
          active_conversation={current_conversation}
          localized_agent_skill={localized_agent_skill}
          localized_runtime_status={localized_runtime_status}
          model_name={model_name}
          total_room_count={sessions.length}
        />

        <RoomProgressSection todos={todos} />

        <RoomUsageSection
          agent_cost_summary={agent_cost_summary}
          session_cost_summary={session_cost_summary}
        />

        <RoomWorkspaceContextSection
          allowed_tool_count={agent.options.allowed_tools?.length ?? 0}
          localized_agent_skill={localized_agent_skill}
          permission_mode={agent.options.permission_mode || "default"}
          served_room_count={sessions.length}
        />
      </div>
    </aside>
  );
}
