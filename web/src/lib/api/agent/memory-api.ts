import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type { MemorySnapshot } from "@/types/memory/memory";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

/** 读取 SDK 文件式记忆在 Agent workspace 中的只读投影。 */
export async function getAgentMemorySnapshotApi(agentId: string): Promise<MemorySnapshot> {
  return requestApi<MemorySnapshot>(
    `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agentId)}/workspace/memory`,
    { method: "GET" },
  );
}
