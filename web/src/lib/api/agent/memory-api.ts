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

/** 删除 Agent workspace 中的一份正文记忆；MEMORY.md 索引由服务端同步维护。 */
export async function deleteAgentMemoryDocumentApi(
  agentId: string,
  path: string,
): Promise<unknown> {
  const query = new URLSearchParams({ path });
  return requestApi<unknown>(
    `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agentId)}/workspace/memory?${query.toString()}`,
    { method: "DELETE" },
  );
}
