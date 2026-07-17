/**
 * 由 go generate ./internal/protocol 自动生成，请勿手改。
 */

export type EventType =
  | 'message'
  | 'stream'
  | 'chat_ack'
  | 'input_queue'
  | 'input_queue_ack'
  | 'round_status'
  | 'agent_round_status'
  | 'session_status'
  | 'runtime_status'
  | 'goal_created'
  | 'goal_updated'
  | 'goal_status_changed'
  | 'goal_progress'
  | 'goal_continuation'
  | 'goal_cleared'
  | 'permission_request'
  | 'permission_request_resolved'
  | 'agent_runtime_event'
  | 'workspace_event'
  | 'directory_changed'
  | 'scheduled_task_changed'
  | 'room_member_added'
  | 'room_member_removed'
  | 'room_deleted'
  | 'room_directed_message'
  | 'room_directed_message_consumed'
  | 'session_resync_required'
  | 'room_resync_required'
  | 'stream_start'
  | 'stream_end'
  | 'stream_cancelled'
  | 'error'
  | 'pong';

export interface EventMessage {
  envelope_id?: string;
  protocol_version: number;
  delivery_mode?: string;
  event_type: EventType;
  session_key?: string;
  session_seq?: number;
  room_id?: string;
  room_seq?: number;
  conversation_id?: string;
  agent_id?: string;
  message_id?: string;
  session_id?: string;
  round_id?: string;
  agent_round_id?: string;
  data: Record<string, unknown>;
  timestamp: number;
}

export interface RoundStatusData {
  round_id: string;
  status: string;
  is_terminal: boolean;
  result_subtype?: string;
}

export interface AgentRoundStatusData {
  round_id: string;
  agent_round_id: string;
  agent_id: string;
  status: string;
  is_terminal: boolean;
}

export interface SessionStatusData {
  is_generating: boolean;
  running_round_ids?: string[];
}

export interface RuntimeStatusData {
  status: 'compacting' | null;
}

export interface ChatAckPendingSlot {
  agent_id: string;
  agent_round_id: string;
  msg_id: string;
  status: string;
  timestamp: number;
  index: number;
}

export interface ChatAckData {
  client_request_id: string;
  client_message_id: string;
  round_id: string;
  user_message_id: string;
  user_message_committed: boolean;
  pending: ChatAckPendingSlot[];
  pending_snapshot: boolean;
  ack_timeout_ms: number;
}

export interface InputQueueAckData {
  accepted: boolean;
  duplicate: boolean;
  action: string;
  item_id: string;
  client_request_id: string;
  client_message_id: string;
  ack_timeout_ms: number;
}

export type ConversationTurnStatus =
  | 'pending'
  | 'running'
  | 'finished'
  | 'interrupted'
  | 'error';

export interface ConversationMessage {
  message_id: string;
  session_key?: string;
  role: 'user' | 'assistant' | 'system';
  round_id: string;
  agent_round_id?: string;
  agent_id?: string;
  parent_id?: string;
  content: unknown;
  timestamp: number;
  display_order?: number;
  stream_status?: 'pending' | 'streaming' | 'done' | 'cancelled' | 'error';
  result_summary?: Record<string, unknown>;
  agent_mentions?: AgentMention[];
}

export interface AgentMention {
  agent_id: string;
  label: string;
  content_block_index: number;
  start_rune: number;
  end_rune: number;
  handoff_id?: string;
}

export interface TurnPendingPermission {
  request_id: string;
  message_id?: string;
  tool_use_id?: string;
  tool_name?: string;
}

export interface AgentTurnSlot {
  agent_id: string;
  agent_round_id: string;
  msg_id?: string;
  status: ConversationTurnStatus;
  assistant_messages: ConversationMessage[];
  pending_permissions: TurnPendingPermission[];
  result_summary?: Record<string, unknown>;
  started_at?: number;
  finished_at?: number;
}

export interface ConversationTurn {
  round_id: string;
  status: ConversationTurnStatus;
  created_at: number;
  updated_at: number;
  user_message: ConversationMessage | null;
  agent_slots: AgentTurnSlot[];
  system_events: ConversationMessage[];
  is_loaded: boolean;
}

export interface ConversationTurnIndexItem {
  round_id: string;
  created_at: number;
  updated_at: number;
  status: ConversationTurnStatus;
  user_preview: string;
  agent_ids: string[];
  loaded: boolean;
}

export interface TurnPage {
  turns: ConversationTurn[];
  next_before_round_id?: string;
  backwards_after_round_id?: string;
}
