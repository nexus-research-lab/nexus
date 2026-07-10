import type { RedeployAgentFailure } from "@/types/capability/skill";

export function formatDeployFailureMessage(
  skillName: string,
  failures?: RedeployAgentFailure[],
): string | null {
  const items = failures?.filter((item) => item.agent_id || item.agent_name || item.error) ?? [];
  if (items.length === 0) return null;

  const agents = items
    .slice(0, 3)
    .map((item) => item.agent_name || item.agent_id || "unknown")
    .join("、");
  const suffix = items.length > 3 ? `${agents} 等 ${items.length} 个 Agent` : agents;
  return `已更新 ${skillName}，但 ${items.length} 个 Agent 部署失败：${suffix}`;
}
