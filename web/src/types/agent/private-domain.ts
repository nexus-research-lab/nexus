export type AgentPrivateScope = "direct" | "audience" | "self" | string;
export type AgentPrivateDirection = "incoming" | "outgoing" | "self" | string;

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
  last_action_id?: string | null;
  last_action_type?: string | null;
  last_content_preview?: string | null;
  last_timestamp?: number | null;
  action_count: number;
}

export interface AgentPrivateEvent {
  action_id: string;
  thread_id: string;
  direction: AgentPrivateDirection;
  action_type: string;
  request_id?: string | null;
  source_agent_id: string;
  target_agent_id?: string | null;
  audience_agent_ids?: string[] | null;
  content?: string | null;
  visibility: string;
  reply_target: string;
  wake_policy?: string | null;
  delay_seconds?: number | null;
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
