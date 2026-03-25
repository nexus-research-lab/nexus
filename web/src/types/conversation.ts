import { SessionId } from "@/types/sdk";

export interface Conversation {
  session_key: string;
  agent_id?: string;
  session_id: SessionId | null;
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

export interface ApiConversation {
  session_key: string;
  agent_id: string;
  session_id: string | null;
  room_id?: string | null;
  conversation_id?: string | null;
  created_at: string;
  last_activity: string;
  is_active: boolean;
  title: string | null;
  message_count: number;
  options: Record<string, unknown> | null;
}

export interface CreateConversationParams {
  title?: string;
  agent_id?: string;
}

export interface UpdateConversationParams {
  title?: string;
}

export interface ConversationSnapshotPayload {
  conversation_id: string;
  message_count: number;
  last_activity_at: number;
  session_id: string | null;
}

export interface InitializeConversationsOptions {
  load_conversations_from_server: () => Promise<void>;
  set_current_conversation: (key: string) => void;
  auto_select_first?: boolean;
  debug_name?: string;
}

export interface ConversationLoaderOptions {
  conversation_id: string | null;
  load_conversation: (key: string) => void;
  debug_name?: string;
}

export interface ConversationStoreState {
  conversations: Conversation[];
  current_conversation_id: string | null;
  loading: boolean;
  error: string | null;
  create_conversation: (params?: CreateConversationParams) => Promise<string>;
  delete_conversation: (key: string) => void;
  update_conversation: (key: string, params: UpdateConversationParams) => void;
  set_current_conversation: (key: string | null) => void;
  sync_conversation_snapshot: (
    key: string,
    patch: Partial<Pick<Conversation, "message_count" | "last_activity_at" | "session_id">>,
  ) => void;
  get_conversation: (key: string) => Conversation | undefined;
  load_conversations_from_server: () => Promise<void>;
  clear_all_conversations: () => void;
}
