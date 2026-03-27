"use client";

import { HOME_AGENT_INSPECTOR_WIDTH_CLASS } from "@/lib/home-layout";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { TodoItem } from "@/types/todo";

import { RoomCollaborationStatusSection } from "./room-collaboration-status-section";
import { RoomMemberSummaryCard } from "./room-member-summary-card";
import { RoomProgressSection } from "./room-progress-section";

interface RoomContextPanelProps {
  agent: Agent;
  room_name: string;
  room_members: Agent[];
  room_conversations: Conversation[];
  active_conversation: Conversation | null;
  todos: TodoItem[];
  is_conversation_busy: boolean;
  on_edit_agent: (agent_id: string) => void;
}

export function RoomContextPanel({
  agent,
  room_name,
  room_members,
  room_conversations,
  active_conversation,
  todos,
  is_conversation_busy,
  on_edit_agent,
}: RoomContextPanelProps) {
  const runtime_status = is_conversation_busy ? "Running" : active_conversation?.is_active === false ? "Idle" : "Active";
  const localized_runtime_status =
    runtime_status === "Running" ? "协作中" : runtime_status === "Idle" ? "待命" : "在线";
  return (
    <aside className={`flex min-h-0 flex-col bg-transparent ${HOME_AGENT_INSPECTOR_WIDTH_CLASS}`}>
      <div className="flex-1 overflow-y-auto scrollbar-hide">
        <RoomMemberSummaryCard
          agent={agent}
          localized_runtime_status={localized_runtime_status}
          on_edit_agent={on_edit_agent}
          runtime_status={runtime_status}
        />

        <RoomCollaborationStatusSection
          active_conversation={active_conversation}
          localized_runtime_status={localized_runtime_status}
          total_member_count={room_members.length}
        />

        <RoomProgressSection todos={todos} />
      </div>
    </aside>
  );
}
