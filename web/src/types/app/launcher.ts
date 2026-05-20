export interface BlobPoint {
  x: number;
  y: number;
}

export interface LauncherTokenSwatch {
  fill: string;
  text: string;
  ring: string;
}

export interface SpotlightToken {
  key: string;
  label: string;
  agent_id: string | null;
  kind: "agent" | "room";
  swatch: LauncherTokenSwatch;
}

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
  title: string;
  status?: string;
  is_active?: boolean;
  last_activity: string;
  message_count?: number;
}

export interface LauncherBootstrapResponse {
  agents: LauncherAgentSummary[];
  rooms: LauncherRoomSummary[];
  conversations: LauncherConversationSummary[];
}
