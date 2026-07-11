import { Agent, ApiAgent } from "@/types/agent/agent";
import type { Message as ChatMessage } from "@/types/conversation/message/entity";

export interface RoomMember {
  id: string;
  room_id: string;
  member_type: string;
  member_user_id?: string | null;
  member_agent_id?: string | null;
  joined_at?: string | null;
}

export interface RoomRecord {
  id: string;
  room_type: string;
  name?: string | null;
  description: string;
  avatar?: string | null;
  skill_names: string[];
  host_agent_id?: string | null;
  host_auto_reply_enabled: boolean;
  private_messages_enabled: boolean;
  created_at?: string | null;
  updated_at?: string | null;
}

export interface RoomAggregate {
  room: RoomRecord;
  members: RoomMember[];
}

export interface RoomConversationRecord {
  id: string;
  room_id: string;
  conversation_type: string;
  title?: string | null;
  message_count?: number | null;
  last_activity_at?: string | null;
  created_at?: string | null;
  updated_at?: string | null;
}

export interface RoomSessionRecord {
  id: string;
  conversation_id: string;
  agent_id: string;
  runtime_id: string;
  version_no: number;
  branch_key: string;
  is_primary: boolean;
  sdk_session_id?: string | null;
  status: string;
  last_activity_at?: string | null;
  created_at?: string | null;
  updated_at?: string | null;
}

export interface RoomSessionSelection {
  value: string;
  session_key: string;
  agent_id: string;
  room_id: string;
  conversation_id: string;
  room_type: RoomRecord["room_type"];
  title: string;
  session: RoomSessionRecord;
  label: string;
}

export interface RoomContextAggregate {
  room: RoomRecord;
  members: RoomMember[];
  member_agents: Agent[];
  conversation: RoomConversationRecord;
  sessions: RoomSessionRecord[];
}

export interface ApiRoomContextAggregate {
  room: RoomRecord;
  members: RoomMember[];
  member_agents?: ApiAgent[] | null;
  conversation: RoomConversationRecord;
  sessions: RoomSessionRecord[];
}

export interface CreateRoomParams {
  agent_ids: string[];
  name?: string;
  description?: string;
  title?: string;
  avatar?: string;
  skill_names?: string[];
  host_agent_id?: string | null;
  host_auto_reply_enabled?: boolean;
  private_messages_enabled?: boolean;
}

export interface UpdateRoomParams {
  name?: string;
  description?: string;
  title?: string;
  avatar?: string;
  skill_names?: string[];
  host_agent_id?: string | null;
  host_auto_reply_enabled?: boolean;
  private_messages_enabled?: boolean;
}

export interface CreateRoomConversationParams {
  title?: string;
}

export interface UpdateRoomConversationParams {
  title?: string;
}

export interface ApiRoomConversationMessagePage {
  items: ChatMessage[];
  has_more: boolean;
  next_before_round_id?: string | null;
  next_before_round_timestamp?: number | null;
}

export interface RoomConversationMessagePage {
  items: ChatMessage[];
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
