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
      className="pointer-events-auto inline-flex h-5 min-w-0 max-w-[190px] items-center gap-1 rounded-[7px] border border-(--surface-canvas-border) bg-(--surface-elevated-background) px-1.5 text-[10px] font-medium text-(--text-muted)"
      title={t("room.goal_lead_select")}
    >
      <UserRound className="h-3 w-3 shrink-0" />
      <select
        className="min-w-0 flex-1 bg-transparent text-[10px] font-semibold text-(--text-default) outline-none disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
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
