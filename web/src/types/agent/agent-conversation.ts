/**
 * useAgentConversation Hook 类型定义
 *
 * [INPUT]: 依赖 @/types 的 Message
 * [OUTPUT]: 对外提供 UseAgentConversationOptions, UseAgentConversationReturn
 * [POS]: types 模块的对话交互类型
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { Dispatch, RefObject, SetStateAction } from 'react';
import { get_session_key_identity } from '@/lib/conversation/session-key';

import {
  AssistantMessage,
  ChatAckData,
  Message,
  RoundLifecycleStatus,
  SessionStatusEventPayload,
  RoomPendingAgentSlotState,
  StreamMessage,
} from '@/types';
import { PendingPermission, PermissionDecisionPayload } from '@/types/conversation/permission';
import { WebSocketMessage, WebSocketSendResult, WebSocketState } from '@/types/system/websocket';
import { WorkspaceEventPayload } from '@/types/app/workspace-live';

export type AgentConversationChatType = 'dm' | 'group';
export type AgentConversationRuntimePhase =
  | 'idle'
  | 'sending'
  | 'running'
  | 'streaming'
  | 'awaiting_permission';
export type AgentConversationSessionControlState =
  | 'unknown'
  | 'controller'
  | 'observer';

export interface AgentConversationIdentity {
  session_key: string | null;
  agent_id?: string | null;
  room_id?: string | null;
  conversation_id?: string | null;
  room_session_id?: string | null;
  chat_type: AgentConversationChatType;
}

export function get_agent_conversation_identity_key(
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

  const session_identity = get_session_key_identity(identity.session_key);
  return session_identity ? `session:${session_identity}` : null;
}

export function get_session_control_status_text(
  session_control_state: AgentConversationSessionControlState,
  observer_count: number,
): string {
  if (session_control_state === 'controller') {
    return observer_count > 0
      ? `当前窗口是主理人，另有 ${observer_count} 个观察窗口`
      : '当前窗口是主理人';
  }

  if (session_control_state === 'observer') {
    return '当前窗口是观察视图';
  }

  return '正在同步控制权状态';
}

export interface UseAgentConversationOptions {
  ws_url?: string;
  identity?: AgentConversationIdentity | null;
  on_error?: (error: Error) => void;
  /** Called when a room-level WS event arrives (member_added/removed/room_deleted) */
  on_room_event?: (event_type: string, data: RoomEventPayload) => void;
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
  session_control_state: AgentConversationSessionControlState;
  is_session_controller: boolean;
  session_controller_client_id: string | null;
  session_observer_count: number;
  error: string | null;
  pending_agent_slots: RoomPendingAgentSlotState[];
  input_queue_items: InputQueueItem[];
  send_message: (
    content: string,
    options?: AgentConversationSendOptions,
  ) => Promise<void>;
  enqueue_input_queue_message: (
    content: string,
    delivery_policy?: AgentConversationDeliveryPolicy,
  ) => Promise<void>;
  delete_input_queue_message: (item_id: string) => Promise<void>;
  guide_input_queue_message: (item_id: string) => Promise<void>;
  reorder_input_queue_messages: (ordered_ids: string[]) => Promise<void>;
  bind_session_key: (key: string | null) => void;
  start_session: () => void;
  load_session: (key: string) => Promise<void>;
  load_older_messages: () => Promise<boolean>;
  clear_session: () => void;
  reset_session: () => void;
  stop_generation: (msg_id?: string) => void;
  pending_permissions: PendingPermission[];
  send_permission_response: (payload: PermissionDecisionPayload) => boolean;
}

export type AgentConversationDeliveryPolicy = 'queue' | 'guide' | 'interrupt' | 'auto';

export type InputQueueScope = 'dm' | 'room';
export type InputQueueSource = 'user' | 'agent_public_mention';

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
  delivery_policy: AgentConversationDeliveryPolicy;
  owner_user_id?: string;
  root_round_id?: string;
  hop_index?: number;
  created_at: number;
  updated_at: number;
}

export interface InputQueueEventPayload {
  scope: InputQueueScope;
  items: InputQueueItem[];
}

export interface AgentConversationSendOptions {
  delivery_policy?: AgentConversationDeliveryPolicy;
}

export interface ConversationSnapshot {
  session_key: string;
  message_count: number;
  last_activity_at: number;
  session_id: string | null;
}

export interface AgentConversationActionContext {
  identity: AgentConversationIdentity | null;
  session_key: string | null;
  ws_state: WebSocketState;
  session_control_state: AgentConversationSessionControlState;
  ws_send: (message: WebSocketMessage) => WebSocketSendResult;
  active_session_key_ref: RefObject<string | null>;
  pending_permissions: PendingPermission[];
  pending_agent_slots: RoomPendingAgentSlotState[];
  input_queue_items: InputQueueItem[];
  messages: Message[];
  set_error: Dispatch<SetStateAction<string | null>>;
  set_messages: Dispatch<SetStateAction<Message[]>>;
  set_pending_agent_slots: Dispatch<SetStateAction<RoomPendingAgentSlotState[]>>;
  set_input_queue_items: Dispatch<SetStateAction<InputQueueItem[]>>;
  set_pending_permissions: Dispatch<SetStateAction<PendingPermission[]>>;
}

export interface AgentConversationLifecycleContext {
  active_session_key_ref: RefObject<string | null>;
  load_request_id_ref: RefObject<number>;
  identity: AgentConversationIdentity | null;
  set_session_key: Dispatch<SetStateAction<string | null>>;
  set_is_session_loading: Dispatch<SetStateAction<boolean>>;
  set_messages: Dispatch<SetStateAction<Message[]>>;
  set_pending_agent_slots: Dispatch<SetStateAction<RoomPendingAgentSlotState[]>>;
  set_input_queue_items?: Dispatch<SetStateAction<InputQueueItem[]>>;
  set_pending_permissions: Dispatch<SetStateAction<PendingPermission[]>>;
  set_error: Dispatch<SetStateAction<string | null>>;
  /** Cache of background messages received for non-active sessions */
  bg_message_cache_ref?: RefObject<Map<string, Message[]>>;
  /** 恢复当前 session 尚未落入历史快照的进行中轮次。 */
  restore_volatile_session_snapshot?: (session_key: string) => boolean;
  /** Session 快照完成加载后，允许 Hook 对运行时状态做对账 */
  on_session_messages_loaded?: (
    messages: Message[],
    meta: {
      session_key: string;
      is_reload: boolean;
      has_more_history: boolean;
      next_before_round_id: string | null;
      next_before_round_timestamp: number | null;
    },
  ) => void;
}

export interface RoomEventPayload {
  room_id?: string;
  conversation_id?: string;
  agent_id?: string;
  agent_name?: string;
  round_id?: string;
  last_seen_room_seq?: number;
  latest_room_seq?: number;
  buffer_start_room_seq?: number | null;
}

export interface HandleAgentConversationWebSocketMessageParams {
  backend_message: unknown;
  apply_workspace_event: (payload: WorkspaceEventPayload) => void;
  is_current_session_event: (incoming_session_key?: string | null) => boolean;
  set_error: Dispatch<SetStateAction<string | null>>;
  set_messages: Dispatch<SetStateAction<Message[]>>;
  set_pending_agent_slots: Dispatch<SetStateAction<RoomPendingAgentSlotState[]>>;
  set_input_queue_items: Dispatch<SetStateAction<InputQueueItem[]>>;
  set_pending_permissions: Dispatch<SetStateAction<PendingPermission[]>>;
  /** Enqueue a stream payload into the rAF batch buffer instead of calling set_messages directly */
  enqueue_stream_payload?: (payload: StreamMessage) => void;
  /** Called when a complete message arrives for a non-active session (for background caching) */
  on_background_message?: (session_key: string, message: Message) => void;
  /** Room-level events from the server (member add/remove/room deleted) */
  on_room_event?: (event_type: string, data: RoomEventPayload) => void;
  /** Update a single message's stream_status field */
  update_message_status?: (
    msg_id: string,
    status: import('@/types/conversation/message').AssistantMessageStatus,
    round_id?: string | null,
  ) => void;
  /** 后端同步当前 session 的权威运行态 */
  sync_session_status?: (payload: SessionStatusEventPayload) => void;
  /** 后端同步单个 round 的权威生命周期 */
  apply_round_status?: (round_id: string, status: RoundLifecycleStatus) => void;
  /** 记录本轮 chat_ack 预分配的活跃消息槽位 */
  track_chat_ack?: (ack: ChatAckData, session_key: string | null) => void;
  /** 同步 assistant 完整消息的终态 */
  track_assistant_message?: (message: AssistantMessage) => void;
}
