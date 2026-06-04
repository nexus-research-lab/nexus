"use client";

import { useMemo } from "react";

import type { Agent } from "@/types/agent/agent";
import {
  goal_continuation_hold_for_room_target,
  ROOM_GOAL_SCOPE_LABEL,
} from "@/features/conversation/shared/goal-continuation-hold";
import { GoalPanel } from "@/features/conversation/shared/goal-panel";

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
  const continuation_hold = useMemo(
    () =>
      goal_continuation_hold_for_room_target(
        room_members,
        room_host_agent_id,
        room_host_auto_reply_enabled,
      ),
    [room_host_agent_id, room_host_auto_reply_enabled, room_members],
  );

  return (
    <GoalPanel
      activity_key={activity_key}
      compact={is_mobile_layout}
      continuation_hold={continuation_hold}
      disabled={!can_control_session}
      is_generating={is_loading}
      session_key={session_key}
      scope_label={ROOM_GOAL_SCOPE_LABEL}
    />
  );
}
