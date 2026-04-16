/**
 * 消息类型定义
 *
 * 本文件定义前端使用的消息数据结构
 */
import { SessionId, ToolInput } from '../system/sdk';

export type MessageRole = 'user' | 'assistant' | 'system' | 'result' | 'agent';

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
  error_code?: string | null;
}

export interface ThinkingContent {
  type: 'thinking';
  thinking: string;
  signature?: string | null;
}

export interface TaskProgressContent {
  type: 'task_progress';
  task_id: string;
  description: string;
  tool_use_id?: string | null;
  last_tool_name?: string | null;
  usage?: Record<string, any>;
}

export type ContentBlock =
  | TextContent
  | ToolUseContent
  | ToolResultContent
  | ThinkingContent
  | TaskProgressContent;

export interface BaseMessage {
  message_id: string;
  session_key: string;
  room_id?: string | null;
  conversation_id?: string | null;
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
}

export interface AgentMessage {
  message_id: string;
  session_key: string;
  room_id: string;
  conversation_id: string;
  sender_agent_id: string;
  content: string;
  timestamp: number;
}

export interface Usage {
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens?: number;
  cache_read_input_tokens?: number;
  [key: string]: any;
}

/** Status for assistant messages in Room multi-agent scenarios. */
export type AssistantMessageStatus = 'pending' | 'streaming' | 'done' | 'cancelled' | 'error';

export interface AssistantMessage extends BaseMessage {
  role: 'assistant';
  content: ContentBlock[];
  is_complete?: boolean;
  stop_reason?: 'end_turn' | 'max_tokens' | 'stop_sequence' | 'tool_use';
  model?: string;
  usage?: Usage;
  /** Room 并发场景下气泡的生命周期状态 */
  stream_status?: AssistantMessageStatus;
}

export interface SystemMessageMetadata extends Record<string, any> {
  subtype?: string;
  attempt?: number;
  max_retries?: number;
  retry_delay_ms?: number;
  error_status?: string | null;
  error?: string | null;
}

export interface SystemMessage extends BaseMessage {
  role: 'system';
  content: string;
  metadata?: SystemMessageMetadata;
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

export type RoundLifecycleStatus = 'running' | 'finished' | 'interrupted' | 'error';

export interface RoundStatusEventPayload {
  round_id: string;
  status: RoundLifecycleStatus;
  is_terminal: boolean;
  result_subtype?: ResultMessage['subtype'] | null;
}

export interface SessionStatusEventPayload {
  is_generating: boolean;
  running_round_ids?: string[];
  controller_client_id?: string | null;
  observer_count?: number;
  bound_client_count?: number;
}

export type StreamMessageType =
  | 'message_start'
  | 'content_block_start'
  | 'content_block_delta'
  | 'message_delta'
  | 'message_stop';

export interface StreamMessage {
  message_id: string;
  session_key: string;
  room_id?: string | null;
  conversation_id?: string | null;
  agent_id: string;
  round_id: string;
  session_id?: SessionId;
  type: StreamMessageType;
  index?: number;
  content_block?: ContentBlock;
  message?: {
    model?: string;
    stop_reason?: AssistantMessage['stop_reason'];
  };
  usage?: Usage;
  timestamp: number;
}

export interface EventMessage {
  envelope_id?: string;
  protocol_version?: number;
  delivery_mode?: 'durable' | 'ephemeral';
  session_seq?: number;
  room_seq?: number;
  event_type:
  | 'message'
  | 'stream'
  | 'permission_request'
  | 'agent_runtime_event'
  | 'workspace_event'
  | 'pong'
  | 'error'
  | 'room_collaboration'
  | 'agent_thinking'
  | 'agent_done'
  | 'room_member_added'
  | 'room_member_removed'
  | 'room_deleted'
  | 'room_resync_required'
  | 'session_resync_required'
  | 'chat_ack'
  | 'round_status'
  | 'stream_start'
  | 'stream_end'
  | 'stream_cancelled'
  | 'session_status';
  session_key?: string | null;
  room_id?: string | null;
  conversation_id?: string | null;
  agent_id?: string | null;
  message_id?: string | null;
  session_id?: SessionId | null;
  caused_by?: string | null;
  data: any;
  timestamp: number;
}

/** Pending agent slot from chat_ack */
export interface PendingAgentSlot {
  agent_id: string;
  msg_id: string;
  round_id?: string;
  status?: AssistantMessageStatus;
  timestamp?: number;
}

/** Room 前端占位槽位状态。 */
export interface RoomPendingAgentSlotState extends PendingAgentSlot {
  round_id: string;
  status: AssistantMessageStatus;
  timestamp: number;
}

/** chat_ack event data */
export interface ChatAckData {
  req_id: string;
  round_id: string;
  pending: PendingAgentSlot[];
}

export type RoomCollaborationEventType = 'agent_message' | 'room_broadcast';

export interface RoomCollaborationEvent {
  event_type: 'room_collaboration';
  data: {
    room_id: string;
    conversation_id: string;
    message_type: RoomCollaborationEventType;
    sender_agent_id?: string;
    content?: string;
  };
  timestamp: number;
}

export interface SystemMessageDisplayMeta {
  label: string;
  tone: 'neutral' | 'warning';
}

export function get_system_message_display_meta(
  message: SystemMessage,
): SystemMessageDisplayMeta {
  const subtype = message.metadata?.subtype;
  if (subtype === 'api_retry') {
    return {
      label: '自动重试',
      tone: 'warning',
    };
  }

  if (subtype === 'task_started') {
    return {
      label: '任务启动',
      tone: 'neutral',
    };
  }

  return {
    label: '系统事件',
    tone: 'neutral',
  };
}
