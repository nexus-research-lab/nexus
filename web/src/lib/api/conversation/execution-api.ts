/**
 * INPUT: 会话 session_key 与 exact 完成态 Execution。
 * OUTPUT: managed WorkGraph 读取、非持久化草图预览和已保存草图目录。
 * POS: Execution/WorkGraph HTTP 协议的 Web 客户端；不负责持久化草图。
 */
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type { ExecutionView } from "@/types/conversation/execution";
import type {
  WorkGraphWorkflow,
  WorkGraphWorkflowPreview,
  WorkGraphWorkflowSaveReceipt,
} from "@/types/conversation/workgraph-workflow";

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

export async function previewWorkGraphWorkflowApi(
  sessionKey: string,
  executionId: string,
): Promise<WorkGraphWorkflowPreview> {
  return requestApi<WorkGraphWorkflowPreview>(
    `${AGENT_API_BASE_URL}/workgraph/previews`,
    {
      body: {
        source_execution_id: executionId,
        source_session_key: sessionKey,
      },
      method: "POST",
    },
  );
}

export async function scheduleWorkGraphWorkflowSaveApi(
  sessionKey: string,
  previewId: string,
): Promise<WorkGraphWorkflowSaveReceipt> {
  return requestApi<WorkGraphWorkflowSaveReceipt>(
    `${AGENT_API_BASE_URL}/workgraph/previews/${encodeURIComponent(previewId)}/save`,
    {
      body: { source_session_key: sessionKey },
      method: "POST",
    },
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
