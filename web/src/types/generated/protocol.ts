/**
 * 由 go generate ./internal/protocol 自动生成，请勿手改。
 */

export type KnownFailureCategory =
  | 'validation'
  | 'authentication'
  | 'authorization'
  | 'not_found'
  | 'conflict'
  | 'rate_limited'
  | 'unavailable'
  | 'timeout'
  | 'canceled'
  | 'internal';

export type FailureCategory =
  | KnownFailureCategory
  | (string & Record<never, never>);

export type KnownFailureEffect =
  | 'not_applicable'
  | 'not_applied'
  | 'accepted'
  | 'committed'
  | 'unknown';

export type FailureEffect =
  | KnownFailureEffect
  | (string & Record<never, never>);

export type KnownFailureRecoveryActor =
  | 'user'
  | 'system'
  | 'external'
  | 'none';

export type FailureRecoveryActor =
  | KnownFailureRecoveryActor
  | (string & Record<never, never>);

export interface FailureResolution {
  actor: FailureRecoveryActor;
  action: string;
}

export interface FailureCore {
  version: number;
  code: string;
  category: FailureCategory;
  effect: FailureEffect;
  transport_request_id?: string;
  retry_after_ms?: number;
  resolution?: FailureResolution;
}

export type EventType =
  | 'message'
  | 'stream'
  | 'chat_ack'
  | 'input_queue'
  | 'input_queue_ack'
  | 'interrupt_ack'
  | 'round_status'
  | 'agent_round_status'
  | 'session_status'
  | 'runtime_status'
  | 'command_catalog'
  | 'context_usage'
  | 'goal_created'
  | 'goal_updated'
  | 'goal_status_changed'
  | 'goal_progress'
  | 'goal_continuation'
  | 'goal_cleared'
  | 'execution_invalidated'
  | 'permission_request'
  | 'permission_request_resolved'
  | 'channel_authorization'
  | 'channel_authorization_result'
  | 'agent_runtime_event'
  | 'workspace_event'
  | 'directory_changed'
  | 'scheduled_task_changed'
  | 'subagent_task_changed'
  | 'room_member_added'
  | 'room_member_removed'
  | 'room_member_participation_changed'
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

export type DeliveryMode = 'durable' | 'ephemeral' | 'transient';

export type ConversationFailureCode =
  | 'connection_unavailable'
  | 'delivery_unknown'
  | 'permission_not_sent'
  | 'provider_configuration'
  | 'provider_unavailable'
  | 'request_rejected'
  | 'round_failed'
  | 'safety_rejected'
  | 'session_load_failed'
  | 'usage_limited'
  | 'validation_failed';

export interface EventMessage {
  envelope_id?: string;
  protocol_version: number;
  delivery_mode?: DeliveryMode;
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
  message?: string;
  failure_code?: ConversationFailureCode;
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

export interface ExecutionInvalidationData {
  execution_id: string;
  version: number;
}

export interface SubagentTaskChangedData {
  task_ids: string[];
}

export type ChannelAuthorizationKind = 'qr_code' | 'verification_code';

export interface ChannelAuthorizationData {
  flow_id: string;
  presentation_token: string;
  kind: ChannelAuthorizationKind;
  channel_type: string;
  account_binding: string;
  qr_payload?: string;
  qr_payload_type?: string;
  prompt: string;
  expires_at: string;
}

export interface ChannelAuthorizationResultData {
  flow_id: string;
  accepted: boolean;
  status?: string;
  message: string;
}

export type CommandCatalogStatus = 'cold' | 'ready' | 'unavailable';
export type CommandExecution = 'host' | 'runtime' | 'unsupported';

export interface CommandDescriptor {
  name: string;
  description?: string;
  argument_hint?: string;
  execution: CommandExecution;
  enabled: boolean;
  disabled_reason?: string;
}

export interface CommandCatalogData {
  revision?: string;
  generation?: number;
  runtime_kind?: string;
  status: CommandCatalogStatus;
  agent_id?: string;
  commands: CommandDescriptor[];
}

export interface ContextUsageData {
  total_tokens: number;
  max_tokens: number;
  percentage: number;
  model?: string;
}

export interface ChatAckPendingSlot {
  agent_id: string;
  agent_round_id: string;
  msg_id: string;
  round_id?: string;
  handoff_id?: string;
  hidden_from_user?: boolean;
  status: string;
  timestamp: number;
  index: number;
}

export interface ChatActivitySourceSnapshot {
  session_key: string;
  conversation_id: string;
  running_round_ids: string[];
}

export interface ChatAckData {
  client_request_id: string;
  client_message_id: string;
  round_id: string;
  user_message_id: string;
  user_message_committed: boolean;
  user_message_delivery_mode?: DeliveryMode;
  pending: ChatAckPendingSlot[];
  pending_snapshot: boolean;
  pending_interaction_snapshot?: boolean;
  pending_interaction_request_ids?: string[];
  activity_snapshot?: boolean;
  active_sources?: ChatActivitySourceSnapshot[];
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

export interface InterruptAckData {
  accepted: boolean;
  client_request_id: string;
  round_id?: string;
  agent_round_id?: string;
  ack_timeout_ms: number;
}

export interface AgentMention {
  agent_id: string;
  label: string;
  content_block_index: number;
  start_rune: number;
  end_rune: number;
  handoff_id?: string;
}

export interface PublicHandoffReply {
  handoff_id: string;
  source_message_id: string;
  source_agent_id: string;
}
