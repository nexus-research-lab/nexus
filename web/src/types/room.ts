import { ProtocolRunDetail, ProtocolRunListItem } from "./protocol-room";

export type RoomMode = "protocol" | "open";
export type RoomRuntimeStatus = "created" | "running" | "paused" | "finished" | "error";
export type RoomMemberSource = "existing" | "ephemeral";
export type RoomParticipantStatus =
  | "listening"
  | "speaking"
  | "thinking"
  | "working"
  | "waiting"
  | "blocked"
  | "done";

export interface RoomRecord {
  id: string;
  room_type: string;
  name?: string | null;
  description: string;
  mode: RoomMode;
  runtime_status: RoomRuntimeStatus;
  active_run_id?: string | null;
  orchestrator_agent_id?: string | null;
  ruleset_slug?: string | null;
  goal: string;
  runtime_state: Record<string, any>;
  capabilities: Record<string, any>;
  created_at?: string | null;
  updated_at?: string | null;
}

export interface RoomMemberRecord {
  id: string;
  room_id: string;
  member_type: "agent" | "user";
  member_user_id?: string | null;
  member_agent_id?: string | null;
  member_source: RoomMemberSource;
  member_role?: string | null;
  member_status: RoomParticipantStatus;
  member_visibility_scope: string[];
  workspace_binding: boolean;
  joined_at?: string | null;
}

export interface RoomAggregate {
  room: RoomRecord;
  members: RoomMemberRecord[];
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

export interface RoomConversationContext {
  room: RoomRecord;
  members: RoomMemberRecord[];
  conversation: RoomConversationRecord;
  sessions: RoomSessionRecord[];
}

export interface RoomMemberSpec {
  existing_agent_id?: string;
  create_spec?: Record<string, any>;
  role_hint?: string;
  source?: RoomMemberSource;
  workspace_binding?: boolean;
}

export interface RoomMessageEnvelope {
  scope?: "broadcast" | "direct" | "group" | "system";
  sender_member_id?: string | null;
  target_member_ids?: string[];
  content: string;
  metadata?: Record<string, any>;
}

export interface RoomActionEnvelope {
  action_type: string;
  actor_member_id?: string | null;
  target_member_id?: string | null;
  payload?: Record<string, any>;
}

export interface RoomEventRecord {
  id: string;
  room_id: string;
  run_id?: string | null;
  event_type: string;
  actor_member_id?: string | null;
  actor_agent_id?: string | null;
  channel_id?: string | null;
  visibility: string;
  audience_member_ids: string[];
  title?: string | null;
  body?: string | null;
  metadata: Record<string, any>;
  created_at: string;
}

export interface RoomArtifactRecord {
  id: string;
  owner_member_id?: string | null;
  owner_agent_id?: string | null;
  kind: string;
  title: string;
  summary: string;
  workspace_ref?: string | null;
  source_ref?: string | null;
  created_at: string;
}

export interface RoomTaskRecord {
  id: string;
  title: string;
  summary: string;
  status: string;
  assignee_member_id?: string | null;
  assignee_agent_id?: string | null;
  created_by?: string | null;
  workspace_path?: string | null;
  metadata: Record<string, any>;
  created_at: string;
  updated_at: string;
}

export interface RoomRuntimeView {
  room: RoomRecord;
  members: RoomMemberRecord[];
  events: RoomEventRecord[];
  artifacts: RoomArtifactRecord[];
  tasks: RoomTaskRecord[];
  protocol_runs: ProtocolRunListItem[];
  protocol_detail?: ProtocolRunDetail | null;
  viewer_member_id?: string | null;
}
