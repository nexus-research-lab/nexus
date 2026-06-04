"use client";

import { useCallback, useMemo, useState } from "react";
import { UserRound } from "lucide-react";

import type { Agent } from "@/types/agent/agent";
import type { Goal } from "@/types/conversation/goal";
import {
  goal_continuation_hold_for_room_target,
  ROOM_GOAL_SCOPE_LABEL,
} from "@/features/conversation/shared/goal-continuation-hold";
import { GoalPanel } from "@/features/conversation/shared/goal-panel";
import {
  resolve_default_room_goal_lead,
  resolve_room_goal_lead_agent_id,
} from "./room-goal-model";

interface RoomGoalPanelProps {
  activity_key: string | number | null;
  can_control_session: boolean;
  is_loading: boolean;
  is_mobile_layout: boolean;
  room_host_agent_id?: string | null;
  room_host_auto_reply_enabled: boolean;
  room_members: Agent[];
  session_key: string | null;
}

export function RoomGoalPanel({
  activity_key,
  can_control_session,
  is_loading,
  is_mobile_layout,
  room_host_agent_id,
  room_host_auto_reply_enabled,
  room_members,
  session_key,
}: RoomGoalPanelProps) {
  const [current_goal, set_current_goal] = useState<Goal | null>(null);
  const default_lead_agent_id = useMemo(
    () => resolve_default_room_goal_lead(room_members, room_host_agent_id),
    [room_host_agent_id, room_members],
  );
  const effective_lead_agent_id = useMemo(
    () =>
      resolve_room_goal_lead_agent_id(
        current_goal,
        room_members,
        default_lead_agent_id,
      ),
    [current_goal, default_lead_agent_id, room_members],
  );
  const lead_agent = useMemo(
    () =>
      room_members.find((agent) => agent.agent_id === effective_lead_agent_id) ??
      null,
    [effective_lead_agent_id, room_members],
  );
  const continuation_hold = useMemo(
    () =>
      goal_continuation_hold_for_room_target(
        room_members,
        effective_lead_agent_id,
        room_host_auto_reply_enabled,
      ),
    [effective_lead_agent_id, room_host_auto_reply_enabled, room_members],
  );
  const handle_goal_change = useCallback((goal: Goal | null) => {
    set_current_goal(goal);
  }, []);
  const status_extra = lead_agent ? (
    <span
      className="inline-flex min-w-0 items-center gap-1 truncate text-(--text-muted)"
      title={`Room Goal 负责人：${lead_agent.name}`}
    >
      <UserRound className="h-3 w-3 shrink-0" />
      <span className="truncate">负责人 {lead_agent.name}</span>
    </span>
  ) : null;

  return (
    <GoalPanel
      activity_key={activity_key}
      compact={is_mobile_layout}
      continuation_hold={continuation_hold}
      disabled={!can_control_session}
      is_generating={is_loading}
      session_key={session_key}
      scope_label={ROOM_GOAL_SCOPE_LABEL}
      status_extra={status_extra}
      on_goal_change={handle_goal_change}
    />
  );
}
