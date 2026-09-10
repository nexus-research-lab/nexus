/**
 * INPUT: Room 成员、负责人草稿、当前 Goal metadata 与当前翻译器。
 * OUTPUT: 保持成员边界的默认/当前负责人、创建可用性。
 * POS: Room Goal 领域纯模型；UI 名称不参与身份校验，不调用 transport。
 */
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";
import type { Goal } from "@/types/conversation/goal";

const ROOM_GOAL_LEAD_AGENT_ID_KEY = "room_goal_lead_agent_id";

function metadataString(
  metadata: Record<string, unknown> | undefined,
  key: string,
): string {
  const value = metadata?.[key];
  return typeof value === "string" ? value.trim() : "";
}

export function resolveDefaultRoomGoalLead(
  roomMembers: Agent[],
  hostAgentId: string | null | undefined,
): string {
  const normalizedHostAgentId = hostAgentId?.trim();
  if (
    normalizedHostAgentId &&
    roomMembers.some((agent) => agent.agent_id === normalizedHostAgentId)
  ) {
    return normalizedHostAgentId;
  }
  if (roomMembers.length === 1) {
    return roomMembers[0]?.agent_id ?? "";
  }
  return "";
}

/** Creation checks the current candidate set at both the control and dispatch boundaries. */
export function resolveRoomGoalCreateDisabledReason(
  roomMembers: Agent[],
  leadAgentId: string,
  t: I18nContextValue["t"],
): string | null {
  if (roomMembers.length === 0) return t("room.goal_no_assignable_agent");
  const id = leadAgentId.trim();
  if (!id) return t("room.goal_lead_required");
  return roomMembers.some((agent) => agent.agent_id === id)
    ? null : t("room.goal_lead_unavailable");
}

export function resolveRoomGoalLeadAgentId(
  goal: Goal | null,
  roomMembers: Agent[],
  fallbackAgentId: string,
): string {
  const metadataAgentId = metadataString(
    goal?.metadata,
    ROOM_GOAL_LEAD_AGENT_ID_KEY,
  );
  if (
    metadataAgentId &&
    roomMembers.some((agent) => agent.agent_id === metadataAgentId)
  ) {
    return metadataAgentId;
  }
  return fallbackAgentId;
}
