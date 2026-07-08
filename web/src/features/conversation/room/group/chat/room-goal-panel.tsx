"use client";

import { useCallback, useMemo, useState } from "react";
import { UserRound } from "lucide-react";

import type { Agent } from "@/types/agent/agent";
import type { Goal } from "@/types/conversation/goal";
import {
  goalContinuationHoldForRoomTarget,
  ROOM_GOAL_SCOPE_LABEL,
} from "@/features/conversation/shared/goal-continuation-hold";
import { GoalPanel } from "@/features/conversation/shared/goal-panel";
import {
  resolveDefaultRoomGoalLead,
  resolveRoomGoalLeadAgentId,
} from "./room-goal-model";

interface RoomGoalPanelProps {
  activityKey: string | number | null;
  canControlSession: boolean;
  isLoading: boolean;
  isMobileLayout: boolean;
  roomHostAgentId?: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomMembers: Agent[];
  sessionKey: string | null;
}

export function RoomGoalPanel({
  activityKey: activityKey,
  canControlSession: canControlSession,
  isLoading: isLoading,
  isMobileLayout: isMobileLayout,
  roomHostAgentId: roomHostAgentId,
  roomHostAutoReplyEnabled: roomHostAutoReplyEnabled,
  roomMembers: roomMembers,
  sessionKey: sessionKey,
}: RoomGoalPanelProps) {
  const [currentGoal, setCurrentGoal] = useState<Goal | null>(null);
  const defaultLeadAgentId = useMemo(
    () => resolveDefaultRoomGoalLead(roomMembers, roomHostAgentId),
    [roomHostAgentId, roomMembers],
  );
  const effectiveLeadAgentId = useMemo(
    () =>
      resolveRoomGoalLeadAgentId(
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
      goalContinuationHoldForRoomTarget(
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
      activityKey={activityKey}
      compact={isMobileLayout}
      continuationHold={continuationHold}
      disabled={!canControlSession}
      isGenerating={isLoading}
      sessionKey={sessionKey}
      scopeLabel={ROOM_GOAL_SCOPE_LABEL}
      statusExtra={statusExtra}
      onGoalChange={handleGoalChange}
    />
  );
}
