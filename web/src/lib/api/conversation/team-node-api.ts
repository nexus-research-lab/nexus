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
  jobs?: Array<{ id: string; agent_id: string; state: "claiming" | "ready" | "running" | "draining" | "review_required" | "completed" | "failed"; room_id?: string; conversation_id?: string; local_agent_id?: string; round_id?: string; source_room_id?: string; source_message_id?: string; delivery_id?: string }>;
}

export function getTeamNode(signal?: AbortSignal, query?: {roomId: string; messageIds: string[]; jobId?: string | null}) {
  const params = new URLSearchParams();
  if (query) {
    params.set("room_id", query.roomId);
    for (const id of query.messageIds) params.append("message_id", id);
    if (query.jobId) params.set("job_id", query.jobId);
  }
  return requestApi<TeamNodeView>(`${NODE_URL}${query ? `?${params}` : ""}`, { method: "GET", signal });
}

export function authorizeTeamNode(name: string, agentIds: string[], enableExecution = false) {
  return requestApi(NODE_URL, { method: "POST", body: { name, agent_ids: agentIds, enable_execution: enableExecution } });
}

export function revokeTeamNode() {
  return requestApi(NODE_URL, { method: "DELETE" });
}

export type TeamNodeJob = NonNullable<TeamNodeView["jobs"]>[number];

export interface TeamRoomBinding {
  agent_id: string;
  local_agent_id: string;
  room_id: string;
  conversation_id: string;
}

export function prepareTeamRoom(roomId: string, signal?: AbortSignal) {
  return requestApi<TeamRoomBinding[]>(`${NODE_URL}/room`, {method: "POST", body: {room_id: roomId}, signal});
}
