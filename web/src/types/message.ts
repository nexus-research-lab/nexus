/**
 * 消息类型定义
 *
 * 本文件定义前端使用的消息数据结构
 */

import { SessionId, ToolInput, ToolOutput } from './sdk';

// ==================== 消息角色 ====================

/** 消息角色 */
export type MessageRole = 'user' | 'assistant' | 'system' | 'result';

// ==================== 内容块类型 ====================

/** 文本内容块 */
export interface TextContent {
  type: 'text';
  text: string;
}

/** 工具使用内容块 */
export interface ToolUseContent {
  type: 'tool_use';
  id: string;
  name: string;
  input: ToolInput;
}

/** 工具结果内容块 */
export interface ToolResultContent {
  type: 'tool_result';
  tool_use_id: string;
  content: string | any[];
  is_error?: boolean;
}

/** 思考内容块 */
export interface ThinkingContent {
  type: 'thinking';
  thinking: string;
}

/** 内容块联合类型 */
export type ContentBlock =
  | TextContent
  | ToolUseContent
  | ToolResultContent
  | ThinkingContent;

// ==================== 消息类型 ====================

/** 基础消息接口 */
export interface BaseMessage {
  message_id: string;
  round_id: string;            // 轮次ID
  agent_id: string;            // session 路由键
  session_id?: SessionId;      // SDK Session ID (可选，由后端返回)
  parent_id?: string;          // 父消息ID (可选，由后端返回)
  role: MessageRole;
  timestamp: number;
}

/** 用户消息 */
export interface UserMessage extends BaseMessage {
  role: 'user';
  content: string;
  parent_tool_use_id?: string | null;
}


/** token使用消息 */
export interface Usage {
  input_tokens: number;
  output_tokens: number;
  cache_creation_tokens?: number;
  cache_read_input_tokens?: number;

  [key: string]: any;
}

/** 助手消息 */
export interface AssistantMessage extends BaseMessage {
  role: 'assistant';
  content: ContentBlock[];
  stop_reason?: 'end_turn' | 'max_tokens' | 'stop_sequence' | 'tool_use';
  model?: string;
  parent_tool_use_id?: string | null;
  is_tool_result?: boolean;
}

/** 系统消息 */
export interface SystemMessage extends BaseMessage {
  role: 'system';
  content: string;
  metadata?: Record<string, any>;
}

/** 执行结果消息 */
export interface ResultMessage extends BaseMessage {
  role: 'result';
  subtype: 'success' | 'error';
  duration_ms: number;
  duration_api_ms: number;
  num_turns: number;
  total_cost_usd?: number;
  usage?: Usage;
  result?: string;
  is_error: boolean;
}

/** 消息联合类型 */
export type Message = UserMessage | AssistantMessage | SystemMessage | ResultMessage;

// ==================== 工具调用类型 ====================

/** 工具调用状态 */
export type ToolCallStatus = 'pending' | 'running' | 'success' | 'error';

/** 工具调用记录 */
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

// ==================== 消息流事件 ====================

/** 流式消息事件类型 */
export type StreamEventType =
  | 'message_start'
  | 'content_block_start'
  | 'content_block_delta'
  | 'content_block_stop'
  | 'message_delta'
  | 'message_stop';

/** 流式消息事件 */
export interface StreamEvent {
  type: StreamEventType;
  index?: number;
  delta?: any;
  content_block?: ContentBlock;
  message?: Partial<AssistantMessage>;
  message_id?: string;
}
