import type { RoomReplyRoute, RoomWakePolicy } from "./agent-conversation";

export type AgentPrivateScope = "direct" | "audience" | "self";
export type AgentPrivateDirection = "incoming" | "outgoing" | "self";

export interface AgentPrivateParticipant {
  agent_id: string;
  name?: string | null;
  avatar?: string | null;
}

export interface AgentPrivateThread {
  thread_id: string;
  agent_id: string;
  scope: AgentPrivateScope;
  participant_agent_ids: string[];
  peer_agent_ids: string[];
  participants: AgentPrivateParticipant[];
  room_id?: string | null;
  room_name?: string | null;
  room_type?: string | null;
  conversation_id?: string | null;
  conversation_title?: string | null;
  last_message_id?: string | null;
  last_content_preview?: string | null;
  last_timestamp?: number | null;
  message_count: number;
}

export interface AgentPrivateEvent {
  message_id: string;
  thread_id: string;
  direction: AgentPrivateDirection;
  source_agent_id: string;
  recipients: string[];
  content?: string | null;
  reply_route: RoomReplyRoute;
  wake_policy?: RoomWakePolicy | null;
  delay_seconds?: number | null;
  correlation_id?: string | null;
  room_id?: string | null;
  room_name?: string | null;
  room_type?: string | null;
  conversation_id?: string | null;
  conversation_title?: string | null;
  participants: AgentPrivateParticipant[];
  timestamp: number;
}

export interface AgentPrivateThreadPage {
  items: AgentPrivateThread[];
}

export interface AgentPrivateEventPage {
  thread: AgentPrivateThread;
  items: AgentPrivateEvent[];
}
