import type { ChangedResource, EventMessage } from "@/types/generated/protocol";

/** 判定 `resource_changed` 是否命中目标资源；带 agentId 时还要求事件涉及该 Agent。 */
export function isResourceChange(
  event: EventMessage,
  resource: ChangedResource,
  agentId?: string,
): boolean {
  if (event.event_type !== "resource_changed" || event.data.resource !== resource) {
    return false;
  }
  if (!agentId) {
    return true;
  }
  const agentIds = event.data.agent_ids;
  return Array.isArray(agentIds) && agentIds.includes(agentId);
}
