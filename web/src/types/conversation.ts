import {
  CreateSessionParams,
  InitializeSessionsOptions,
  Session,
  UpdateSessionParams,
} from "@/types/session";

export type Conversation = Session;
export type CreateConversationParams = CreateSessionParams;
export type UpdateConversationParams = UpdateSessionParams;

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
  load_conversations_from_server: InitializeSessionsOptions["load_sessions_from_server"];
  clear_all_conversations: () => void;
}
