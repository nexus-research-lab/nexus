import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";
import type { Message } from "@/types/conversation/message";

const AGENT_API_BASE_URL = get_agent_api_base_url();

export interface SubagentTask {
  task_id: string;
  session_key?: string;
  agent_id?: string;
  agent_type?: string;
  description?: string;
  model?: string;
  name?: string;
  parent_task_id?: string;
  round_id?: string;
  status: string;
  team_name?: string;
  tool_use_id?: string;
  output_file?: string;
  transcript_path?: string;
  usage?: Record<string, unknown>;
  started_at?: number;
  updated_at?: number;
}

interface SubagentTaskListResponse {
  items: SubagentTask[];
}

export interface SubagentTaskMessagesResponse {
  task: SubagentTask;
  messages: Message[];
  output?: string;
}

export interface SubagentTaskStopResponse {
  success: boolean;
  task_id: string;
  status: string;
}

export interface SubagentTaskMessageResponse {
  success: boolean;
  task_id: string;
  status: string;
}

export async function list_session_subagent_tasks_api(
  session_key: string,
): Promise<SubagentTask[]> {
  const result = await request_api<SubagentTaskListResponse>(
    `${AGENT_API_BASE_URL}/sessions/${encodeURIComponent(session_key)}/tasks`,
    { method: "GET" },
  );
  return result.items ?? [];
}

export async function get_session_subagent_task_messages_api(
  session_key: string,
  task_id: string,
): Promise<SubagentTaskMessagesResponse> {
  return request_api<SubagentTaskMessagesResponse>(
    `${AGENT_API_BASE_URL}/sessions/${encodeURIComponent(session_key)}/tasks/${encodeURIComponent(task_id)}/messages`,
    { method: "GET" },
  );
}

export async function stop_session_subagent_task_api(
  session_key: string,
  task_id: string,
): Promise<SubagentTaskStopResponse> {
  return request_api<SubagentTaskStopResponse>(
    `${AGENT_API_BASE_URL}/sessions/${encodeURIComponent(session_key)}/tasks/${encodeURIComponent(task_id)}/stop`,
    { method: "POST" },
  );
}

export async function send_session_subagent_task_message_api(
  session_key: string,
  task_id: string,
  message: string,
): Promise<SubagentTaskMessageResponse> {
  return request_api<SubagentTaskMessageResponse>(
    `${AGENT_API_BASE_URL}/sessions/${encodeURIComponent(session_key)}/tasks/${encodeURIComponent(task_id)}/messages`,
    {
      body: JSON.stringify({ message }),
      method: "POST",
    },
  );
}

export async function list_room_subagent_tasks_api(
  room_id: string,
  conversation_id: string,
): Promise<SubagentTask[]> {
  const result = await request_api<SubagentTaskListResponse>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}/tasks`,
    { method: "GET" },
  );
  return result.items ?? [];
}

export async function get_room_subagent_task_messages_api(
  room_id: string,
  conversation_id: string,
  task_id: string,
): Promise<SubagentTaskMessagesResponse> {
  return request_api<SubagentTaskMessagesResponse>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}/tasks/${encodeURIComponent(task_id)}/messages`,
    { method: "GET" },
  );
}

export async function stop_room_subagent_task_api(
  room_id: string,
  conversation_id: string,
  task_id: string,
): Promise<SubagentTaskStopResponse> {
  return request_api<SubagentTaskStopResponse>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}/tasks/${encodeURIComponent(task_id)}/stop`,
    { method: "POST" },
  );
}

export async function send_room_subagent_task_message_api(
  room_id: string,
  conversation_id: string,
  task_id: string,
  message: string,
): Promise<SubagentTaskMessageResponse> {
  return request_api<SubagentTaskMessageResponse>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}/tasks/${encodeURIComponent(task_id)}/messages`,
    {
      body: JSON.stringify({ message }),
      method: "POST",
    },
  );
}
