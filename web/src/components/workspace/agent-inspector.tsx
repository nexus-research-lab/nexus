"use client";

import { RoomCollaborationStatusSection } from "@/features/room-context/room-collaboration-status-section";
import { RoomMemberSummaryCard } from "@/features/room-context/room-member-summary-card";
import { RoomProgressSection } from "@/features/room-context/room-progress-section";
import { RoomUsageSection } from "@/features/room-context/room-usage-section";
import { RoomWorkspaceContextSection } from "@/features/room-context/room-workspace-context-section";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { Session } from "@/types/session";
import { TodoItem } from "./agent-task-widget";
import { AgentCostSummary, SessionCostSummary } from "@/types/cost";
import { HOME_AGENT_INSPECTOR_WIDTH_CLASS } from "@/lib/home-layout";

interface AgentInspectorProps {
  agent: Agent;
  sessions: Session[];
  activeSession: Session | null;
  todos: TodoItem[];
  isSessionBusy: boolean;
  sessionCostSummary: SessionCostSummary;
  agentCostSummary: AgentCostSummary;
  onEditAgent: (agentId: string) => void;
}

export function AgentInspector({
  agent,
  sessions,
  activeSession,
  todos,
  isSessionBusy,
  sessionCostSummary,
  agentCostSummary,
  onEditAgent,
}: AgentInspectorProps) {
  const runtimeStatus = isSessionBusy ? "Running" : activeSession?.is_active === false ? "Idle" : "Active";
  const modelName = agent.options.model || "inherit";
  const localizedRuntimeStatus =
    runtimeStatus === "Running" ? "协作中" : runtimeStatus === "Idle" ? "待命" : "在线";
  const localizedAgentSkill = agent.options.skills_enabled ? "技能已启用" : "通用成员";
  const currentConversation = (activeSession as Conversation | null) ?? null;

  return (
    <aside className={`flex min-h-0 flex-col bg-transparent ${HOME_AGENT_INSPECTOR_WIDTH_CLASS}`}>
      <div className="flex-1 overflow-y-auto scrollbar-hide">
        <RoomMemberSummaryCard
          agent={agent}
          modelName={modelName}
          onEditAgent={onEditAgent}
          runtimeStatus={runtimeStatus}
          localizedRuntimeStatus={localizedRuntimeStatus}
        />

        <RoomCollaborationStatusSection
          activeConversation={currentConversation}
          localizedAgentSkill={localizedAgentSkill}
          localizedRuntimeStatus={localizedRuntimeStatus}
          modelName={modelName}
          totalRoomCount={sessions.length}
        />

        <RoomProgressSection todos={todos} />

        <RoomUsageSection
          agentCostSummary={agentCostSummary}
          sessionCostSummary={sessionCostSummary}
        />

        <RoomWorkspaceContextSection
          allowedToolCount={agent.options.allowed_tools?.length ?? 0}
          localizedAgentSkill={localizedAgentSkill}
          permissionMode={agent.options.permission_mode || "default"}
          servedRoomCount={sessions.length}
        />
      </div>
    </aside>
  );
}
