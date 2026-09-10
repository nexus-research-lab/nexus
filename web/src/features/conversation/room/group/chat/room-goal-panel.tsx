/**
 * INPUT: 当前 Room 成员、宿主配置、Session 与 Goal 事件回调。
 * OUTPUT: 从共享面板当前 Goal 直接投影的负责人、续跑约束和本地化 Room 身份。
 * POS: Room Goal 展示适配；不缓存第二份 Goal 或推导跨会话运行状态。
 */
"use client";

import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { useCallback, useMemo } from "react";
import { UserRound } from "lucide-react";

import { buildAgentSelectionOptions } from "@/lib/agent-selection-options";
import type { Agent } from "@/types/agent/agent";
import type { Goal } from "@/types/conversation/goal";
import {
  goalContinuationHoldForRoomTarget,
} from "@/features/conversation/shared/goal/goal-continuation-hold";
import { GoalPanel } from "@/features/conversation/shared/goal/goal-panel";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  resolveDefaultRoomGoalLead,
  resolveRoomGoalLeadAgentId,
} from "./room-goal-model";

interface RoomGoalPanelProps {
  activityKey: string | number | null;
  isLoading: boolean;
  isMobileLayout: boolean;
  onGoalChange: (goal: Goal | null) => void;
  roomHostAgentId?: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomMembers: Agent[];
  sessionKey: string | null;
}

export function RoomGoalPanel({
  activityKey,
  isLoading,
  isMobileLayout,
  onGoalChange,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomMembers,
  sessionKey,
}: RoomGoalPanelProps) {
  const { t } = useI18n();
  const defaultLeadAgentId = useMemo(
    () => resolveDefaultRoomGoalLead(roomMembers, roomHostAgentId),
    [roomHostAgentId, roomMembers],
  );
  const memberOptions = useMemo(() => buildAgentSelectionOptions(roomMembers, t), [roomMembers, t]);
  const resolveLead = useCallback((goal: Goal) => resolveRoomGoalLeadAgentId(
    goal, roomMembers, defaultLeadAgentId,
  ), [defaultLeadAgentId, roomMembers]);
  const continuationHold = useCallback((goal: Goal) => goalContinuationHoldForRoomTarget(
    roomMembers, resolveLead(goal), roomHostAutoReplyEnabled, t,
  ), [resolveLead, roomHostAutoReplyEnabled, roomMembers, t]);
  const statusExtra = useCallback((goal: Goal) => {
    const lead = memberOptions.find((option) => option.value === resolveLead(goal));
    return lead ? (
      <UiTooltip label={t("room.goal_lead_status_title", { name: lead.label })}><span
        className="inline-flex min-w-0 max-w-full items-center gap-1 text-(--text-muted)"
      >
        <UserRound aria-hidden="true" className="h-3 w-3 shrink-0" />
        <span className="truncate">{t("room.goal_lead_status", { name: lead.label })}</span>
      </span></UiTooltip>
    ) : null;
  }, [memberOptions, resolveLead, t]);

  return (
    <GoalPanel
      activityKey={activityKey}
      compact={isMobileLayout}
      continuationHold={continuationHold}
      isGenerating={isLoading}
      sessionKey={sessionKey}
      scopeLabel={t("goal.scope_room")}
      statusExtra={statusExtra}
      onGoalChange={onGoalChange}
    />
  );
}
