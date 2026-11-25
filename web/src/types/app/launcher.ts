export interface LauncherAgentSummary {
  id: string;
  name: string;
  avatar?: string;
  description?: string | null;
}

export interface LauncherRoomMemberSummary {
  id: string;
  name: string;
  avatar?: string;
}

export interface LauncherRoomSummary {
  id: string;
  room_type: "dm" | "room";
  name?: string;
  avatar?: string;
  dm_target_agent_id?: string;
  created_at?: string;
  updated_at?: string;
  members?: LauncherRoomMemberSummary[];
}

export interface LauncherConversationSummary {
  session_key: string;
  agent_id?: string;
  room_id?: string;
  conversation_id?: string;
  room_type: "dm" | "room";
  channel_type?: string;
  title: string;
  last_reply_preview?: string;
  last_activity: string;
  message_count?: number;
}

export interface LauncherBootstrapResponse {
  agents: LauncherAgentSummary[];
  rooms: LauncherRoomSummary[];
  conversations: LauncherConversationSummary[];
  /** 服务端读取目录前的已提交版本；不晚于它的目录失效通知可跳过重读。 */
  directory_revision?: number;
}
