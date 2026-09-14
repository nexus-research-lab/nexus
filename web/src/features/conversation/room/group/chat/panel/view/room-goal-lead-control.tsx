/**
 * INPUT: Room Goal 当前负责人、候选成员与禁用态。
 * OUTPUT: 公共单选菜单与统一成员名称的负责人控件；缺项明确显示，scope/候选变化关闭旧菜单。
 * POS: Room Goal 模式在 Composer Footer 中的专属控制。
 */

import { UserRound } from "lucide-react";

import { buildAgentSelectionOptions, includeUnavailableAgentSelection } from "@/lib/agent-selection-options";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";

export interface RoomGoalLeadControlProps {
  agentId: string;
  disabled: boolean;
  onChange: (agentId: string) => void;
  roomMembers: Agent[];
  scopeKey?: string;
}

export function RoomGoalLeadControl({
  agentId,
  disabled,
  onChange,
  roomMembers,
  scopeKey,
}: RoomGoalLeadControlProps) {
  const { t } = useI18n();
  const options = includeUnavailableAgentSelection(
    [{ value: "", label: t("room.goal_lead_label") }, ...buildAgentSelectionOptions(roomMembers, t)],
    agentId,
    t,
  );
  return (
    <UiSelectMenu
      ariaLabel={t("room.switch_agent_current", {
        label: t("room.goal_lead_select"),
        name: options.find((option) => option.value === agentId)?.label ?? t("room.goal_lead_label"),
      })}
      className="nexus-chat-composer-goal-lead pointer-events-auto min-w-[5.5rem] max-w-[190px] flex-1"
      disabled={disabled || roomMembers.length === 0}
      leading={<UserRound aria-hidden="true" className="h-3.5 w-3.5 shrink-0" />}
      menuMinWidth={220}
      onChange={(value) => {
        if (!disabled && (value === "" || roomMembers.some((agent) => agent.agent_id === value))) onChange(value);
      }}
      options={options}
      placement="top"
      resetKey={JSON.stringify([scopeKey, roomMembers.map((agent) => agent.agent_id).sort()])}
      size="xs"
      value={agentId}
    />
  );
}
