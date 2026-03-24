import { CreateSessionParams, Session, UpdateSessionParams } from "@/types/session";

export type Conversation = Session;
export type CreateConversationParams = CreateSessionParams;
export type UpdateConversationParams = UpdateSessionParams;

export interface ConversationSnapshotPayload {
  conversation_id: string;
  message_count: number;
  last_activity_at: number;
  session_id: string | null;
}
