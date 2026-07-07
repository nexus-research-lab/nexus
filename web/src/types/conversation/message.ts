/**
 * 消息类型定义
 *
 * 本文件定义前端使用的消息数据结构
 */
import { SessionId, ToolInput } from "../system/sdk";

export type MessageRole = "user" | "assistant" | "system" | "agent";

export interface TextContent {
  type: "text";
  text: string;
}

export interface ToolUseErrorContent {
  type: "tool_use_error";
  content: string;
}

export interface ToolUseContent {
  type: "tool_use";
  id: string;
  name: string;
  input: ToolInput;
}

export interface ToolResultContent {
  type: "tool_result";
  tool_use_id: string;
  content: string | any[];
  is_error?: boolean;
  error_code?: string | null;
}

export interface ThinkingContent {
  type: "thinking";
  thinking: string;
  signature?: string | null;
}

export interface ImageContent {
  type: "image";
  data?: string;
  mime_type?: string | null;
  alt?: string | null;
  path?: string | null;
  url?: string | null;
  uri?: string | null;
  source?: {
    type?: string;
    data?: string;
    media_type?: string;
    mime_type?: string;
    url?: string;
    uri?: string;
    path?: string;
  } | null;
}

export interface TaskProgressContent {
  type: "task_progress";
  task_id: string;
  description: string;
  tool_use_id?: string | null;
  last_tool_name?: string | null;
  usage?: Record<string, any>;
}

export interface WorkspaceFileArtifactContent {
  type: "workspace_file_artifact";
  id?: string;
  path: string;
  display_path?: string | null;
  label?: string | null;
  title?: string | null;
  artifact_kind?: string | null;
  mime_type?: string | null;
  operation?: string | null;
  scope?: MessageAttachmentScope;
  workspace_agent_id?: string | null;
  source_tool_use_id?: string | null;
  source_tool_name?: string | null;
}

export type SystemEventTone = "neutral" | "warning";
export type SystemEventIcon = "retry" | "progress" | "status" | "guide";

export interface SystemEventContent {
  type: "system_event";
  content: string;
  label: string;
  tone: SystemEventTone;
  icon: SystemEventIcon;
  source_message_id: string;
  timestamp: number;
  subtype?: string;
  tool_use_id?: string | null;
  attempt?: number;
  max_retries?: number;
  retry_delay_ms?: number;
  error_status?: string | number | null;
  error?: string | null;
}

export type ContentBlock =
  | TextContent
  | ToolUseErrorContent
  | ToolUseContent
  | ToolResultContent
  | ThinkingContent
  | ImageContent
  | TaskProgressContent
  | WorkspaceFileArtifactContent
  | SystemEventContent;

export interface BaseMessage {
  message_id: string;
  session_key: string;
  room_id?: string | null;
  conversation_id?: string | null;
  agent_id: string;
  round_id: string;
  /** Room slot / agent 私有执行轮次 id；round_id 永远是 root round。 */
  agent_round_id?: string | null;
  session_id?: SessionId;
  parent_id?: string;
  role: MessageRole;
  timestamp: number;
  /** 运行态投影字段；用于区分 durable / ephemeral，不来自历史回放。 */
  delivery_mode?: "durable" | "ephemeral";
}

export type MessageAttachmentKind = "text" | "image" | "file";
export type MessageAttachmentScope = "agentWorkspace" | "roomConversation";

export interface MessageAttachment {
  file_name: string;
  workspace_path: string;
  workspace_agent_id?: string;
  room_id?: string;
  conversation_id?: string;
  scope?: MessageAttachmentScope;
  kind: MessageAttachmentKind;
  mime_type?: string | null;
  size?: number;
}

export interface UserMessage extends BaseMessage {
  role: "user";
  content: string;
  delivery_policy?: "queue" | "guide" | "interrupt" | "auto";
  attachments?: MessageAttachment[];
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

/** Status for assistant messages in Room multi-agent scenarios. */
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
  /** UI-only；不是后端协议字段。 */
  stream_status?: AssistantMessageStatus;
}

export interface SystemMessageMetadata extends Record<string, any> {
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

export type RoundLifecycleStatus =
  | "running"
  | "finished"
  | "interrupted"
  | "error";

export interface RoundStatusEventPayload {
  round_id: string;
  status: RoundLifecycleStatus;
  is_terminal: boolean;
  result_subtype?: ResultSummary["subtype"] | null;
}

export interface SessionStatusEventPayload {
  is_generating: boolean;
  running_round_ids?: string[];
}

export type StreamMessageType =
  | "message_start"
  | "content_block_start"
  | "content_block_delta"
  | "message_delta"
  | "message_stop";

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
    stop_reason?: AssistantMessage["stop_reason"];
  };
  usage?: Usage;
  timestamp: number;
}

export interface EventMessage {
  envelope_id?: string;
  protocol_version?: number;
  delivery_mode?: "durable" | "ephemeral";
  session_seq?: number;
  room_seq?: number;
  event_type:
    | "message"
    | "stream"
    | "permission_request"
    | "agent_runtime_event"
    | "workspace_event"
    | "directory_changed"
    | "scheduled_task_changed"
    | "pong"
    | "error"
    | "room_collaboration"
    | "room_member_added"
    | "room_member_removed"
    | "room_deleted"
    | "room_directed_message"
    | "room_directed_message_consumed"
    | "room_resync_required"
    | "session_resync_required"
    | "chat_ack"
    | "input_queue"
    | "round_status"
    | "agent_round_status"
    | "stream_start"
    | "stream_end"
    | "stream_cancelled"
    | "permission_request_resolved"
    | "goal_created"
    | "goal_updated"
    | "goal_status_changed"
    | "goal_progress"
    | "goal_continuation"
    | "goal_cleared"
    | "session_status";
  session_key?: string | null;
  room_id?: string | null;
  conversation_id?: string | null;
  agent_id?: string | null;
  message_id?: string | null;
  session_id?: SessionId | null;
  round_id?: string | null;
  agent_round_id?: string | null;
  data: any;
  timestamp: number;
}

/** Pending agent slot from chatAck */
export interface PendingAgentSlot {
  agent_id: string;
  agent_round_id: string;
  msg_id: string;
  status?: AssistantMessageStatus;
  timestamp?: number;
  index?: number;
}

/** Room 前端占位槽位状态。round_id 是 root round。 */
export interface RoomPendingAgentSlotState extends PendingAgentSlot {
  round_id: string;
  status: AssistantMessageStatus;
  timestamp: number;
}

/** chatAck event data */
export interface ChatAckData {
  client_request_id: string;
  client_message_id: string;
  round_id: string;
  user_message_id: string;
  pending: PendingAgentSlot[];
  ack_timeout_ms?: number;
}

/** agent_round_status event data（Room slot 生命周期）。 */
export interface AgentRoundStatusEventPayload {
  round_id: string;
  agent_round_id: string;
  agent_id: string;
  status: RoundLifecycleStatus;
  is_terminal: boolean;
}

export type RoomCollaborationEventType = "agent_message" | "room_broadcast";

export interface RoomCollaborationEvent {
  event_type: "room_collaboration";
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
  tone: "neutral" | "warning";
  icon: SystemEventIcon;
}

export function getSystemMessageDisplayMeta(
  message: SystemMessage,
): SystemMessageDisplayMeta {
  const subtype = message.metadata?.subtype;
  if (subtype === "api_retry") {
    return {
      label: "自动重试",
      tone: "warning",
      icon: "retry",
    };
  }

  if (subtype === "task_started" || subtype === "task_progress") {
    return {
      label: "执行状态",
      tone: "neutral",
      icon: "progress",
    };
  }

  if (subtype === "task_notification" || subtype === "task_updated" || subtype === "status") {
    return {
      label: "状态更新",
      tone: "neutral",
      icon: "status",
    };
  }

  if (subtype === "compact_boundary") {
    return {
      label: "上下文压缩",
      tone: "neutral",
      icon: "status",
    };
  }

  if (subtype === "guided_input") {
    return {
      label: "已引导对话",
      tone: "neutral",
      icon: "guide",
    };
  }

  return {
    label: "系统事件",
    tone: "neutral",
    icon: "status",
  };
}
