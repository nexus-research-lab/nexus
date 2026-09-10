/**
 * INPUT: 领域已验证的目标成员、权限模式、群主接管开关与当前翻译器。
 * OUTPUT: DM/Room 共用的本地化 Goal 续跑约束；姓名沿当前目录显示，不暴露内部身份。
 * POS: Goal continuation 展示纯模型；不重建服务端运行状态或创建权限。
 */
import { buildAgentSelectionOptions } from "@/lib/agent-selection-options";
import { getAgentDisplayName } from "@/lib/agent-display-name";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";

export interface GoalContinuationHold {
  detail: string;
  label: string;
}

export function goalContinuationHoldForPermission(
  agentName: string | null | undefined,
  permissionMode: string | null | undefined,
  t: I18nContextValue["t"],
): GoalContinuationHold | null {
  if ((permissionMode ?? "").trim() !== "plan") {
    return null;
  }
  return {
    detail: t("goal.hold_plan_detail", { name: getAgentDisplayName(agentName, t) }),
    label: t("goal.hold_plan_label"),
  };
}

export function goalContinuationHoldForRoomTarget(
  roomMembers: Agent[],
  leadAgentId: string | null | undefined,
  roomHostAutoReplyEnabled: boolean,
  t: I18nContextValue["t"],
): GoalContinuationHold | null {
  const targetAgent = resolveGoalContinuationTargetAgent(
    roomMembers,
    leadAgentId,
  );
  if (targetAgent) {
    const label = buildAgentSelectionOptions(roomMembers, t).find((option) => option.value === targetAgent.agent_id)?.label;
    return goalContinuationHoldForPermission(label, targetAgent.options?.permission_mode, t);
  }
  if (roomMembers.length <= 1) {
    return null;
  }
  if (!roomHostAutoReplyEnabled) {
    return {
      detail: t("goal.hold_owner_detail"),
      label: t("goal.hold_owner_label"),
    };
  }
  return {
    detail: t("goal.hold_target_detail"),
    label: t("goal.hold_target_label"),
  };
}

function resolveGoalContinuationTargetAgent(
  roomMembers: Agent[],
  leadAgentId: string | null | undefined,
): Agent | null {
  if (roomMembers.length === 1) {
    return roomMembers[0] ?? null;
  }
  const normalizedLeadAgentId = leadAgentId?.trim();
  if (!normalizedLeadAgentId) {
    return null;
  }
  return roomMembers.find((agent) => agent.agent_id === normalizedLeadAgentId) ?? null;
}
