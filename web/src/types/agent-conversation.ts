/**
 * useAgentConversation Hook 类型定义
 *
 * [INPUT]: 依赖 @/types 的 Message
 * [OUTPUT]: 对外提供 UseAgentConversationOptions, UseAgentConversationReturn
 * [POS]: types 模块的对话交互类型
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { Dispatch, RefObject, SetStateAction } from 'react';

import { Message, StreamMessage } from '@/types';
import { PendingPermission, PermissionDecisionPayload } from '@/types/permission';
import { WebSocketMessage, WebSocketState } from '@/types/websocket';
import { WorkspaceEventPayload } from '@/types/workspace-live';

export interface UseAgentConversationOptions {
  ws_url?: string;
  agent_id?: string | null;
  /** Room context — 传入后消息使用 room 路由 */
  room_id?: string | null;
  conversation_id?: string | null;
  chat_type?: 'dm' | 'group';
  on_error?: (error: Error) => void;
  /** Called when a room-level WS event arrives (member_added/removed/room_deleted) */
  on_room_event?: (event_type: string, data: RoomEventPayload) => void;
}

export interface UseAgentConversationReturn {
  messages: Message[];
  session_key: string | null;
  ws_state: WebSocketState;
  is_loading: boolean;
  error: string | null;
  send_message: (content: string) => Promise<void>;
  bind_conversation_key: (key: string | null) => void;
  start_conversation: () => void;
  load_conversation: (key: string) => Promise<void>;
  clear_conversation: () => void;
  reset_conversation: () => void;
  stop_generation: (msg_id?: string) => void;
  delete_round: (round_id: string) => Promise<void>;
  regenerate: (round_id: string) => Promise<void>;
  pending_permission: PendingPermission | null;
  send_permission_response: (payload: PermissionDecisionPayload) => boolean;
  /** Current agent thinking status (multi-agent room only) */
  agent_thinking: AgentThinkingPayload | null;
}

export interface ConversationSnapshot {
  session_key: string;
  message_count: number;
  last_activity_at: number;
  session_id: string | null;
}

export interface AgentConversationActionContext {
  agent_id?: string | null;
  session_key: string | null;
  /** Room context — 传入后消息使用 room 路由 */
  room_id?: string | null;
  conversation_id?: string | null;
  chat_type?: 'dm' | 'group';
  ws_state: WebSocketState;
  ws_send: (message: WebSocketMessage) => void;
  active_conversation_key_ref: RefObject<string | null>;
  pending_permission: PendingPermission | null;
  messages: Message[];
  set_error: Dispatch<SetStateAction<string | null>>;
  set_is_loading: Dispatch<SetStateAction<boolean>>;
  set_messages: Dispatch<SetStateAction<Message[]>>;
  set_pending_permission: Dispatch<SetStateAction<PendingPermission | null>>;
}

export interface AgentConversationLifecycleContext {
  active_conversation_key_ref: RefObject<string | null>;
  load_request_id_ref: RefObject<number>;
  set_conversation_key: Dispatch<SetStateAction<string | null>>;
  set_messages: Dispatch<SetStateAction<Message[]>>;
  set_pending_permission: Dispatch<SetStateAction<PendingPermission | null>>;
  set_is_loading: Dispatch<SetStateAction<boolean>>;
  set_error: Dispatch<SetStateAction<string | null>>;
  /** Cache of background messages received for non-active sessions */
  bg_message_cache_ref?: RefObject<Map<string, Message[]>>;
}

export interface AgentThinkingPayload {
  agent_id: string;
  agent_name: string;
  round_id: string;
}

export interface RoomEventPayload {
  room_id?: string;
  agent_id?: string;
  agent_name?: string;
  round_id?: string;
}

export interface HandleAgentConversationWebSocketMessageParams {
  backend_message: unknown;
  apply_workspace_event: (payload: WorkspaceEventPayload) => void;
  is_current_session_event: (incoming_session_key?: string | null) => boolean;
  set_error: Dispatch<SetStateAction<string | null>>;
  set_is_loading: Dispatch<SetStateAction<boolean>>;
  set_messages: Dispatch<SetStateAction<Message[]>>;
  set_pending_permission: Dispatch<SetStateAction<PendingPermission | null>>;
  /** Enqueue a stream payload into the rAF batch buffer instead of calling set_messages directly */
  enqueue_stream_payload?: (payload: StreamMessage) => void;
  /** Called when a complete message arrives for a non-active session (for background caching) */
  on_background_message?: (session_key: string, message: Message) => void;
  /** Agent thinking/done status for multi-agent rooms */
  set_agent_thinking?: (payload: AgentThinkingPayload | null) => void;
  /** Room-level events from the server (member add/remove/room deleted) */
  on_room_event?: (event_type: string, data: RoomEventPayload) => void;
  /** Update a single message's stream_status field */
  update_message_status?: (msg_id: string, status: import('@/types/message').AssistantMessageStatus) => void;
}
