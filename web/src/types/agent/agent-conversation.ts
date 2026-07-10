/**
 * useAgentConversation Hook 类型定义
 *
 * [INPUT]: 依赖 @/types 的 Message
 * [OUTPUT]: 对外提供 UseAgentConversationOptions, UseAgentConversationReturn
 * [POS]: types 模块的对话交互类型
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { getSessionKeyIdentity } from '@/lib/conversation/session-key';

import {
  MessageAttachment,
  Message,
  RoomPendingAgentSlotState,
} from '@/types';
import { PendingPermission, PermissionDecisionPayload } from '@/types/conversation/permission';
import { WebSocketState } from '@/types/system/websocket';

export type AgentConversationChatType = 'dm' | 'group';
export type AgentConversationRuntimePhase =
  | 'idle'
  | 'sending'
  | 'running'
  | 'streaming'
  | 'awaiting_permission';
export interface AgentConversationIdentity {
  session_key: string | null;
  agent_id?: string | null;
  room_id?: string | null;
  conversation_id?: string | null;
  room_session_id?: string | null;
  chat_type: AgentConversationChatType;
}

export function getAgentConversationIdentityKey(
  identity: AgentConversationIdentity | null | undefined,
): string | null {
  if (!identity) {
    return null;
  }

  if (identity.room_session_id) {
    return `room-session:${identity.room_session_id}`;
  }

  if (identity.chat_type === 'group' && identity.conversation_id) {
    return `room-conversation:${identity.conversation_id}`;
  }

  const sessionIdentity = getSessionKeyIdentity(identity.session_key);
  return sessionIdentity ? `session:${sessionIdentity}` : null;
}

export interface UseAgentConversationOptions {
  ws_url?: string;
  identity?: AgentConversationIdentity | null;
  on_error?: (error: Error) => void;
  /** Called when a room-level WS event arrives for member or room changes. */
  on_room_event?: (eventType: string, data: RoomEventPayload) => void;
}

export interface UseAgentConversationReturn {
  messages: Message[];
  session_key: string | null;
  ws_state: WebSocketState;
  is_loading: boolean;
  live_round_ids: string[];
  is_session_loading: boolean;
  is_history_loading: boolean;
  has_more_history: boolean;
  history_prepend_token: number;
  runtime_phase: AgentConversationRuntimePhase;
  error: string | null;
  pending_agent_slots: RoomPendingAgentSlotState[];
  input_queue_items: InputQueueItem[];
  send_message: (
    content: string,
    options?: AgentConversationSendOptions,
  ) => Promise<void>;
  rewrite_last_user_message: (
    targetRoundId: string,
    content: string,
  ) => Promise<void>;
  enqueue_input_queue_message: (
    content: string,
    deliveryPolicy?: AgentConversationDeliveryPolicy,
    attachments?: MessageAttachment[],
  ) => Promise<void>;
  delete_input_queue_message: (itemId: string) => Promise<void>;
  guide_input_queue_message: (itemId: string) => Promise<void>;
  reorder_input_queue_messages: (orderedIds: string[]) => Promise<void>;
  bind_session_key: (key: string | null) => void;
  start_session: () => void;
  load_session: (key: string) => Promise<void>;
  load_older_messages: () => Promise<boolean>;
  load_round_window: (roundId: string) => Promise<boolean>;
  clear_session: () => void;
  reset_session: () => void;
  stop_generation: (agentRoundId?: string) => void;
  pending_permissions: PendingPermission[];
  send_permission_response: (payload: PermissionDecisionPayload) => boolean;
}

export type AgentConversationDeliveryPolicy = 'queue' | 'guide' | 'interrupt' | 'auto';
export type AgentConversationDefaultDeliveryPolicy = 'queue' | 'interrupt';

export type InputQueueScope = 'dm' | 'room';
export type InputQueueSource = 'user' | 'agent_public_mention' | 'agent_room_directed_message';
export type RoomWakePolicy = 'none' | 'immediate' | 'delayed';
export type RoomReplyRouteMode = 'public' | 'private' | 'none';

export interface RoomReplyRoute {
  mode: RoomReplyRouteMode;
  recipients?: string[];
  wake_policy?: RoomWakePolicy;
  next_reply_route?: RoomReplyRoute;
}

export interface InputQueueItem {
  id: string;
  scope: InputQueueScope;
  session_key: string;
  room_id?: string;
  conversation_id?: string;
  agent_id?: string;
  source_agent_id?: string;
  source_message_id?: string;
  target_agent_ids?: string[];
  source: InputQueueSource;
  content: string;
  attachments?: MessageAttachment[];
  delivery_policy: AgentConversationDeliveryPolicy;
  owner_user_id?: string;
  root_round_id?: string;
  hop_index?: number;
  reply_route?: RoomReplyRoute;
  created_at: number;
  updated_at: number;
}

export interface InputQueueEventPayload {
  scope: InputQueueScope;
  items: InputQueueItem[];
}

export interface AgentConversationSendOptions {
  delivery_policy?: AgentConversationDeliveryPolicy;
  attachments?: MessageAttachment[];
}

export interface ConversationSnapshot {
  session_key: string;
  message_count: number;
  last_activity_at: number;
  session_id: string | null;
}

export interface RoomEventPayload {
  room_id?: string;
  conversation_id?: string;
  agent_id?: string;
  agent_name?: string;
  message_id?: string;
  event_kind?: "created" | "wake_scheduled" | "wake_started" | "wake_queued";
  source_agent_id?: string;
  target_agent_id?: string;
  recipients?: string[];
  reply_route?: RoomReplyRoute;
  wake_policy?: RoomWakePolicy;
  delay_seconds?: number;
  content_chars?: number;
  content?: string;
  correlation_id?: string;
  round_id?: string;
  last_message_id?: string;
  last_message_timestamp?: number;
  last_seen_room_seq?: number;
  latest_room_seq?: number;
  buffer_start_room_seq?: number | null;
}
