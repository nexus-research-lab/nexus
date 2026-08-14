import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type {
	SubagentTaskActionResult,
  SubagentTaskListResponse,
  SubagentTaskMessagesResponse,
  SubagentTaskSource,
} from "@/types/conversation/subagent-task";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

function subagentTaskSourceUrl(source: SubagentTaskSource): string {
  if (source.kind === "session") {
    return `${AGENT_API_BASE_URL}/sessions/${encodeURIComponent(source.session_key)}/tasks`;
  }
  return `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(source.room_id)}/conversations/${encodeURIComponent(source.conversation_id)}/tasks`;
}

function subagentTaskUrl(source: SubagentTaskSource, taskId: string): string {
  return `${subagentTaskSourceUrl(source)}/${encodeURIComponent(taskId)}`;
}

export async function listSubagentTasksApi(
  source: SubagentTaskSource,
): Promise<SubagentTaskListResponse> {
  return requestApi<SubagentTaskListResponse>(subagentTaskSourceUrl(source), {
    method: "GET",
  });
}

export async function getSubagentTaskMessagesApi(
  source: SubagentTaskSource,
  taskId: string,
): Promise<SubagentTaskMessagesResponse> {
  return requestApi<SubagentTaskMessagesResponse>(
    `${subagentTaskUrl(source, taskId)}/messages`,
    { method: "GET" },
  );
}

export async function stopSubagentTaskApi(
  source: SubagentTaskSource,
  taskId: string,
  signal?: AbortSignal,
): Promise<SubagentTaskActionResult> {
  return requestApi<SubagentTaskActionResult>(
    `${subagentTaskUrl(source, taskId)}/stop`,
    { method: "POST", signal },
  );
}

export async function sendSubagentTaskMessageApi(
  source: SubagentTaskSource,
  taskId: string,
  message: string,
  signal?: AbortSignal,
): Promise<SubagentTaskActionResult> {
  return requestApi<SubagentTaskActionResult>(subagentTaskUrl(source, taskId) + "/messages", {
    body: { message },
    method: "POST",
    signal,
  });
}
