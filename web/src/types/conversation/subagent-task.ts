import type { Message } from "@/types/conversation/message";

export type SubagentRuntimeKind = "nxs" | "claude" | "mixed" | "unknown";

export interface SubagentTaskCapabilities {
  observe: boolean;
  transcript: boolean;
  stop: boolean;
  send_message: boolean;
  resume: boolean;
}

export interface SubagentTask {
  task_id: string;
  session_key?: string;
  child_session_id?: string;
  agent_id?: string;
  host_agent_id?: string;
  agent_type?: string;
  description?: string;
  summary?: string;
  last_tool_name?: string;
  model?: string;
  name?: string;
  parent_task_id?: string;
  round_id?: string;
  status: string;
  task_type?: string;
  team_name?: string;
  tool_use_id?: string;
  output_file?: string;
  transcript_path?: string;
  usage?: Record<string, unknown>;
  started_at?: number;
  updated_at?: number;
  runtime_kind: SubagentRuntimeKind;
  capabilities: SubagentTaskCapabilities;
}

export interface SubagentTaskListResponse {
  runtime_kind: SubagentRuntimeKind;
  capabilities: SubagentTaskCapabilities;
  items: SubagentTask[];
}

export interface SubagentTaskMessagesResponse {
  task: SubagentTask;
  messages: Message[];
  output?: string;
}

export interface SubagentTaskActionResponse {
  success: boolean;
  task_id: string;
  status: string;
}

export type SubagentTaskSource =
  | { kind: "session"; session_key: string }
  | { kind: "room"; room_id: string; conversation_id: string };
