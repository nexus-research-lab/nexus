/**
 * 可持久化会话消息实体契约。
 */

import type { SessionId } from "../../system/sdk";
import type { MessageAttachment } from "./attachment";
import type { ContentBlock } from "./content";

type MessageRole = "user" | "assistant" | "system";

interface BaseMessage {
  message_id: string;
  session_key: string;
  room_id?: string | null;
  conversation_id?: string | null;
  agent_id: string;
  round_id: string;
  /** Room slot / Agent 私有执行轮次 id；`round_id` 始终表示根轮次。 */
  agent_round_id?: string | null;
  session_id?: SessionId;
  parent_id?: string;
  role: MessageRole;
  timestamp: number;
  /** 仅存在于运行态投影，不属于历史消息协议。 */
  delivery_mode?: "durable" | "ephemeral";
}

export interface UserMessage extends BaseMessage {
  role: "user";
  content: string;
  delivery_policy?: "queue" | "guide" | "interrupt" | "auto";
  attachments?: MessageAttachment[];
}

export interface Usage {
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens?: number;
  cache_read_input_tokens?: number;
  [key: string]: unknown;
}

export interface ResultSummary {
  message_id?: string;
  timestamp?: number;
  subtype: "success" | "error" | "interrupted";
  duration_ms: number;
  duration_api_ms: number;
  num_turns: number;
  total_cost_usd?: number;
  usage?: Usage;
  result?: string;
  is_error: boolean;
}

export type AssistantMessageStatus =
  | "pending"
  | "streaming"
  | "done"
  | "cancelled"
  | "error";

export interface AssistantMessage extends BaseMessage {
  role: "assistant";
  content: ContentBlock[];
  is_complete?: boolean;
  stop_reason?: "end_turn" | "max_tokens" | "stop_sequence" | "tool_use";
  model?: string;
  usage?: Usage;
  result_summary?: ResultSummary;
  /** 前端流式状态，不属于后端持久化消息字段。 */
  stream_status?: AssistantMessageStatus;
}

interface SystemMessageMetadata extends Record<string, unknown> {
  subtype?: string;
  attempt?: number;
  max_retries?: number;
  retry_delay_ms?: number;
  error_status?: string | number | null;
  error?: string | null;
}

export interface SystemMessage extends BaseMessage {
  role: "system";
  content: string;
  metadata?: SystemMessageMetadata;
}

export type Message = UserMessage | AssistantMessage | SystemMessage;
