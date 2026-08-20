/**
 * INPUT: 会话 session_key。
 * OUTPUT: 当前或最近一次 managed Execution WorkGraph；从未创建时返回 null。
 * POS: Execution 只读 HTTP 协议的 Web 客户端。
 */
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type { ExecutionView } from "@/types/conversation/execution";
import type { WorkGraphWorkflow } from "@/types/conversation/workgraph-workflow";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

export async function getLatestExecutionApi(
  sessionKey: string,
): Promise<ExecutionView | null> {
  const query = new URLSearchParams({ session_key: sessionKey });
  return requestApi<ExecutionView | null>(
    `${AGENT_API_BASE_URL}/executions/latest?${query.toString()}`,
    { method: "GET" },
  );
}

export async function getExecutionHistoryApi(
  sessionKey: string,
  limit = 40,
): Promise<ExecutionView[]> {
  const query = new URLSearchParams({
    session_key: sessionKey,
    limit: String(limit),
  });
  return requestApi<ExecutionView[]>(
    `${AGENT_API_BASE_URL}/executions/history?${query.toString()}`,
    { method: "GET" },
  );
}

export async function getWorkGraphWorkflowsApi(): Promise<WorkGraphWorkflow[]> {
  return requestApi<WorkGraphWorkflow[]>(
    `${AGENT_API_BASE_URL}/workgraph/workflows`,
    { method: "GET" },
  );
}

export async function deleteWorkGraphWorkflowApi(
  workflowId: string,
): Promise<{ deleted: boolean }> {
  return requestApi<{ deleted: boolean }>(
    `${AGENT_API_BASE_URL}/workgraph/workflows/${encodeURIComponent(workflowId)}`,
    { method: "DELETE" },
  );
}
