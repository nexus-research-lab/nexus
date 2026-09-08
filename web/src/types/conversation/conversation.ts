// INPUT: 当前会话 API、导航、Store 与快照同步调用。
// OUTPUT: 已接入的会话视图、API 响应和快照类型。
// POS: 会话身份与读模型契约；不镜像未使用的创建/更新请求。

import { SessionId } from "@/types/system/sdk";

export interface BaseConversation {
  session_key: string;
  agent_id?: string;
  session_id: SessionId | null;
  room_session_id?: string | null;
  room_id?: string | null;
  conversation_id?: string | null;
  conversation_type?: string;
  title: string;
  options: Record<string, unknown>;
  created_at: number;
  last_activity_at: number;
  is_active?: boolean;
  message_count?: number;
}

export interface Conversation extends BaseConversation {}

export interface RoomConversationView extends BaseConversation {
  room_id: string;
  conversation_id: string;
  is_draft?: boolean;
}

export interface ApiConversation {
  session_key: string;
  agent_id: string;
  session_id: string | null;
  room_session_id?: string | null;
  room_id?: string | null;
  conversation_id?: string | null;
  created_at: string;
  last_activity: string;
  is_active: boolean;
  title: string | null;
  message_count: number;
  options: Record<string, unknown> | null;
}

interface BaseSnapshotPayload {
  has_user_input?: boolean;
  last_activity_at?: number;
  message_count?: number;
  session_id: string | null;
}

export interface SessionSnapshotPayload extends BaseSnapshotPayload {
  session_key: string;
  agent_id?: string | null;
  room_id?: string | null;
  conversation_id?: string | null;
  room_session_id?: string | null;
}

export interface RoomConversationSnapshotPayload extends BaseSnapshotPayload {
  conversation_id: string;
}

export type ConversationSnapshotPayload = SessionSnapshotPayload | RoomConversationSnapshotPayload;

export interface SessionLoaderOptions {
  session_key: string | null;
  load_session: (key: string) => void;
  debug_name?: string;
}

export interface ConversationStoreState {
  conversations: Conversation[];
  loading: boolean;
  error: string | null;
  sync_conversation_snapshot: (
    key: string,
    patch: Partial<Pick<Conversation, "last_activity_at" | "session_id">>,
  ) => void;
  load_conversations_from_server: () => Promise<void>;
  clear_all_conversations: () => void;
}
