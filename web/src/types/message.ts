/**
 * 消息类型定义
 *
 * 本文件定义前端使用的消息数据结构
 */

import { SessionId, ToolInput, ToolOutput } from './sdk';

export type MessageRole = 'user' | 'assistant' | 'system' | 'result';

export interface TextContent {
  type: 'text';
  text: string;
}

export interface ToolUseContent {
  type: 'tool_use';
  id: string;
  name: string;
  input: ToolInput;
}

export interface ToolResultContent {
  type: 'tool_result';
  tool_use_id: string;
  content: string | any[];
  is_error?: boolean;
}

export interface ThinkingContent {
  type: 'thinking';
  thinking: string;
  signature?: string | null;
}

export type ContentBlock =
  | TextContent
  | ToolUseContent
  | ToolResultContent
  | ThinkingContent;

export interface BaseMessage {
  message_id: string;
  session_key: string;
  agent_id: string;
  round_id: string;
  session_id?: SessionId;
  parent_id?: string;
  role: MessageRole;
  timestamp: number;
}

export interface UserMessage extends BaseMessage {
  role: 'user';
  content: string;
  parent_tool_use_id?: string | null;
}

export interface Usage {
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens?: number;
  cache_read_input_tokens?: number;
  [key: string]: any;
}

export interface AssistantMessage extends BaseMessage {
  role: 'assistant';
  content: ContentBlock[];
  stop_reason?: 'end_turn' | 'max_tokens' | 'stop_sequence' | 'tool_use';
  model?: string;
  usage?: Usage;
  parent_tool_use_id?: string | null;
  is_tool_result?: boolean;
  target_message_id?: string | null;
}

export interface SystemMessage extends BaseMessage {
  role: 'system';
  content: string;
  metadata?: Record<string, any>;
}

export interface ResultMessage extends BaseMessage {
  role: 'result';
  subtype: 'success' | 'error' | 'interrupted';
  duration_ms: number;
  duration_api_ms: number;
  num_turns: number;
  total_cost_usd?: number;
  usage?: Usage;
  result?: string;
  is_error: boolean;
}

export type Message = UserMessage | AssistantMessage | SystemMessage | ResultMessage;

export type ToolCallStatus = 'pending' | 'running' | 'success' | 'error';

export interface ToolCall {
  id: string;
  tool_name: string;
  input: ToolInput;
  output?: ToolOutput;
  status: ToolCallStatus;
  start_time: number;
  end_time?: number;
  error?: string;
  parent_tool_use_id?: string | null;
}

export type StreamMessageType =
  | 'message_start'
  | 'content_block_start'
  | 'content_block_delta'
  | 'content_block_stop'
  | 'message_delta'
  | 'message_stop';

export interface StreamMessage {
  message_id: string;
  session_key: string;
  agent_id: string;
  round_id: string;
  session_id?: SessionId;
  type: StreamMessageType;
  index?: number;
  delta?: any;
  content_block?: ContentBlock;
  message?: Partial<AssistantMessage>;
  usage?: Record<string, any>;
  timestamp: number;
}

export interface EventMessage {
  event_type: 'message' | 'stream' | 'permission_request' | 'workspace_event' | 'pong' | 'error';
  session_key?: string | null;
  agent_id?: string | null;
  session_id?: SessionId | null;
  data: any;
  timestamp: number;
}
