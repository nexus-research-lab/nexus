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

export interface RoomContextAggregate {
  room: RoomRecord;
  members: RoomMember[];
  conversation: RoomConversationRecord;
  sessions: RoomSessionRecord[];
}

export interface CreateRoomParams {
  agent_ids: string[];
  name?: string;
  description?: string;
  title?: string;
}

export interface UpdateRoomParams {
  name?: string;
  description?: string;
  title?: string;
}

export interface CreateRoomConversationParams {
  title?: string;
}

export interface UpdateRoomConversationParams {
  title?: string;
}
