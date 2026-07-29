/**
 * INPUT: Room Goal 当前负责人、候选成员与禁用态。
 * OUTPUT: 在 Composer 命名容器内可收缩但保持可操作的负责人选择器。
 * POS: Room Goal 模式在 Composer Footer 中的专属控制。
 */

import { UserRound } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";

export interface RoomGoalLeadControlProps {
  agentId: string;
  disabled: boolean;
  onChange: (agentId: string) => void;
  roomMembers: Agent[];
}

export function RoomGoalLeadControl({
  agentId,
  disabled,
  onChange,
  roomMembers,
}: RoomGoalLeadControlProps) {
  const { t } = useI18n();
  return (
    <label
      className="nexus-chat-composer-goal-lead pointer-events-auto inline-flex h-6 min-w-[5.5rem] max-w-[190px] flex-1 items-center gap-1 radius-control-xs border border-(--surface-canvas-border) bg-(--surface-elevated-background) px-1.5 text-2xs font-medium text-(--text-muted)"
      title={t("room.goal_lead_select")}
    >
      <UserRound className="h-3 w-3 shrink-0" />
      <select
        className="min-w-0 flex-1 bg-transparent text-2xs font-semibold text-(--text-default) outline-none disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
        disabled={disabled}
        value={agentId}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">{t("room.goal_lead_label")}</option>
        {roomMembers.map((agent) => (
          <option key={agent.agent_id} value={agent.agent_id}>
            {agent.name}
          </option>
        ))}
      </select>
    </label>
  );
}
