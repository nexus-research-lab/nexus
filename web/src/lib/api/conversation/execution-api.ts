/**
 * INPUT: 会话 session_key 与 exact 完成态 Execution。
 * OUTPUT: managed WorkGraph 读取、durable Draft/版本编辑、已保存草图目录与隐藏保存调度。
 * POS: Execution/WorkGraph HTTP 协议的 Web 客户端；命名图持久化仍由隐藏 Skill + CLI round 完成。
 */
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type { ExecutionView } from "@/types/conversation/execution";
import type {
  WorkGraphWorkflow,
  WorkGraphWorkflowEditorSession,
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

export async function getExecutionApi(
  sessionKey: string,
  executionId: string,
): Promise<ExecutionView> {
  const query = new URLSearchParams({ session_key: sessionKey });
  return requestApi<ExecutionView>(
    `${AGENT_API_BASE_URL}/executions/${encodeURIComponent(executionId)}?${query.toString()}`,
    { method: "GET" },
  );
}

export async function startWorkGraphWorkflowEditorApi(
  sessionKey: string,
  preview: WorkGraphWorkflowPreview,
  outputLanguage: "zh" | "en",
): Promise<WorkGraphWorkflowEditorSession> {
  return requestApi<WorkGraphWorkflowEditorSession>(
    `${AGENT_API_BASE_URL}/workgraph/previews/${encodeURIComponent(preview.preview_id)}/editor`,
    {
      body: {
        description: preview.description,
        output_language: outputLanguage,
        slash_name: preview.slash_name,
        source_session_key: sessionKey,
        title: preview.title,
      },
      method: "POST",
    },
  );
}

export async function getWorkGraphWorkflowEditorApi(
  sessionKey: string,
  editorId: string,
): Promise<WorkGraphWorkflowEditorSession> {
  const query = new URLSearchParams({ source_session_key: sessionKey });
  return requestApi<WorkGraphWorkflowEditorSession>(
    `${AGENT_API_BASE_URL}/workgraph/editors/${encodeURIComponent(editorId)}?${query.toString()}`,
    { method: "GET" },
  );
}

export async function applyWorkGraphWorkflowEditorApi(
  sessionKey: string,
  editorId: string,
  revision: number,
): Promise<WorkGraphWorkflowPreview> {
  return requestApi<WorkGraphWorkflowPreview>(
    `${AGENT_API_BASE_URL}/workgraph/editors/${encodeURIComponent(editorId)}/apply`,
    {
      body: { revision, source_session_key: sessionKey },
      method: "POST",
    },
  );
}

export async function selectWorkGraphWorkflowEditorVersionApi(
  sessionKey: string,
  editorId: string,
  revision: number,
  selectedRevision: number,
): Promise<WorkGraphWorkflowEditorSession> {
  return requestApi<WorkGraphWorkflowEditorSession>(
    `${AGENT_API_BASE_URL}/workgraph/editors/${encodeURIComponent(editorId)}/versions/select`,
    {
      body: {
        revision,
        selected_revision: selectedRevision,
        source_session_key: sessionKey,
      },
      method: "POST",
    },
  );
}

export async function closeWorkGraphWorkflowEditorApi(
  sessionKey: string,
  editorId: string,
): Promise<{ deleted: boolean }> {
  const query = new URLSearchParams({ source_session_key: sessionKey });
  return requestApi<{ deleted: boolean }>(
    `${AGENT_API_BASE_URL}/workgraph/editors/${encodeURIComponent(editorId)}?${query.toString()}`,
    { method: "DELETE" },
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

export async function previewSavedWorkGraphWorkflowApi(
  workflowId: string,
  outputLanguage: "zh" | "en",
): Promise<WorkGraphWorkflowPreview> {
  return requestApi<WorkGraphWorkflowPreview>(
    `${AGENT_API_BASE_URL}/workgraph/workflows/${encodeURIComponent(workflowId)}/preview`,
    { body: { output_language: outputLanguage }, method: "POST" },
  );
}

export async function previewWorkGraphWorkflowApi(
  sessionKey: string,
  executionId: string,
  outputLanguage: "zh" | "en",
): Promise<WorkGraphWorkflowPreview> {
  return requestApi<WorkGraphWorkflowPreview>(
    `${AGENT_API_BASE_URL}/workgraph/previews`,
    {
      body: {
        source_execution_id: executionId,
        source_session_key: sessionKey,
        output_language: outputLanguage,
      },
      method: "POST",
    },
  );
}

export async function scheduleWorkGraphWorkflowSaveApi(
  sessionKey: string,
  previewId: string,
  metadata: Pick<WorkGraphWorkflowPreview, "description" | "slash_name" | "title">,
): Promise<WorkGraphWorkflowSaveReceipt> {
  return requestApi<WorkGraphWorkflowSaveReceipt>(
    `${AGENT_API_BASE_URL}/workgraph/previews/${encodeURIComponent(previewId)}/save`,
    {
      body: {
        description: metadata.description,
        slash_name: metadata.slash_name,
        source_session_key: sessionKey,
        title: metadata.title,
      },
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
