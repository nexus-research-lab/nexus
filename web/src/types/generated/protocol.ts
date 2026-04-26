/**
 * 由 cmd/protocol-tsgen 自动生成，请勿手改。
 */

export type EventType =
  | "message"
  | "stream"
  | "chat_ack"
  | "input_queue"
  | "round_status"
  | "session_status"
  | "permission_request"
  | "permission_request_resolved"
  | "agent_runtime_event"
  | "workspace_event"
  | "room_member_added"
  | "room_member_removed"
  | "room_deleted"
  | "session_resync_required"
  | "room_resync_required"
  | "stream_start"
  | "stream_end"
  | "stream_cancelled"
  | "error"
  | "pong";

export interface EventMessage {
  envelope_id?: string;
  protocol_version: number;
  delivery_mode?: string;
  event_type: EventType;
  session_key?: string;
  session_seq?: number;
  room_id?: string;
  room_seq?: number;
  conversation_id?: string;
  agent_id?: string;
  message_id?: string;
  session_id?: string;
  caused_by?: string;
  data: Record<string, unknown>;
  timestamp: number;
}

export interface RoundStatusData {
  round_id: string;
  status: string;
  is_terminal: boolean;
  result_subtype?: string;
}

export interface SessionStatusData {
  is_generating: boolean;
  running_round_ids?: string[];
  controller_client_id?: string;
  observer_count?: number;
  bound_client_count?: number;
}
