// INPUT: 本机授权、显式执行开关和读取命令。
// OUTPUT: 不包含机器凭据的授权与本机任务视图。
// POS: 本机入口不走 /team 远程代理。
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";

const NODE_URL = `${getAgentApiBaseUrl()}/team-node`;

export interface TeamNodeView {
  node_id?: string;
  state: "disconnected" | "pending" | "authorized" | "revoking" | "revoked";
  name?: string;
  agent_ids: string[];
  candidates: Array<{ id: string; name: string }>;
  execution_available: boolean;
  execution_enabled?: boolean;
  jobs?: Array<{ id: string; agent_id: string; state: "claiming" | "ready" | "running" | "draining" | "review_required" | "completed" | "failed"; room_id?: string; conversation_id?: string }>;
}

export function getTeamNode(signal?: AbortSignal) {
  return requestApi<TeamNodeView>(NODE_URL, { method: "GET", signal });
}

export function authorizeTeamNode(name: string, agentIds: string[], enableExecution = false) {
  return requestApi(NODE_URL, { method: "POST", body: { name, agent_ids: agentIds, enable_execution: enableExecution } });
}

export function revokeTeamNode() {
  return requestApi(NODE_URL, { method: "DELETE" });
}
