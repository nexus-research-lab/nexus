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
  activity_key: activityKey,
  can_control_session: canControlSession,
  is_loading: isLoading,
  is_mobile_layout: isMobileLayout,
  room_host_agent_id: roomHostAgentId,
  room_host_auto_reply_enabled: roomHostAutoReplyEnabled,
  room_members: roomMembers,
  session_key: sessionKey,
}: RoomGoalPanelProps) {
  const [currentGoal, setCurrentGoal] = useState<Goal | null>(null);
  const defaultLeadAgentId = useMemo(
    () => resolve_default_room_goal_lead(roomMembers, roomHostAgentId),
    [roomHostAgentId, roomMembers],
  );
  const effectiveLeadAgentId = useMemo(
    () =>
      resolve_room_goal_lead_agent_id(
        currentGoal,
        roomMembers,
        defaultLeadAgentId,
      ),
    [currentGoal, defaultLeadAgentId, roomMembers],
  );
  const leadAgent = useMemo(
    () =>
      roomMembers.find((agent) => agent.agent_id === effectiveLeadAgentId) ??
      null,
    [effectiveLeadAgentId, roomMembers],
  );
  const continuationHold = useMemo(
    () =>
      goal_continuation_hold_for_room_target(
        roomMembers,
        effectiveLeadAgentId,
        roomHostAutoReplyEnabled,
      ),
    [effectiveLeadAgentId, roomHostAutoReplyEnabled, roomMembers],
  );
  const handleGoalChange = useCallback((goal: Goal | null) => {
    setCurrentGoal(goal);
  }, []);
  const statusExtra = leadAgent ? (
    <span
      className="inline-flex min-w-0 items-center gap-1 truncate text-(--text-muted)"
      title={`Room Goal 负责人：${leadAgent.name}`}
    >
      <UserRound className="h-3 w-3 shrink-0" />
      <span className="truncate">负责人 {leadAgent.name}</span>
    </span>
  ) : null;

  return (
    <GoalPanel
      activity_key={activityKey}
      compact={isMobileLayout}
      continuation_hold={continuationHold}
      disabled={!canControlSession}
      is_generating={isLoading}
      session_key={sessionKey}
      scope_label={ROOM_GOAL_SCOPE_LABEL}
      status_extra={statusExtra}
      on_goal_change={handleGoalChange}
    />
  );
}
