import type { Message } from "./message/entity";

export interface ConversationMessagesQuery {
  around_limit?: number | null;
  around_round_id?: string | null;
  before_round_id?: string | null;
  before_round_timestamp?: number | null;
  limit?: number;
}

export interface ApiConversationMessagePage {
  items: Message[];
  has_more: boolean;
  next_before_round_id?: string | null;
  next_before_round_timestamp?: number | null;
}

export interface ConversationMessagePage {
  items: Message[];
  has_more: boolean;
  next_before_round_id: string | null;
  next_before_round_timestamp: number | null;
}

export interface ApiSessionRoundIndexItem {
  round_id: string;
  title?: string;
  timestamp?: number;
  status?: string;
  duration_ms?: number | null;
  is_live?: boolean;
  has_user_message?: boolean;
  agent_ids?: string[] | null;
}

export interface ApiSessionRoundIndex {
  items?: ApiSessionRoundIndexItem[];
}

export interface SessionRoundIndexItem {
  roundId: string;
  title: string;
  timestamp: number | null;
  status: string | null;
  durationMs: number | null;
  isLive: boolean;
  hasUserMessage: boolean;
  agentIds: string[];
}
