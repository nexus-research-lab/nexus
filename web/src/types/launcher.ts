import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";

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

export interface ConversationWithOwner {
  owner: Agent | null;
  conversation: Conversation;
}

export interface LauncherQueryRequest {
  query: string;
}

export interface LauncherQueryResponse {
  action_type: 'open_agent_dm' | 'open_room';
  target_id: string;
  initial_message?: string;
}

export interface LauncherSuggestion {
  type: 'agent' | 'room';
  id: string;
  name: string;
  avatar?: string;
  last_activity?: string;
}

export interface LauncherSuggestionsResponse {
  agents: LauncherSuggestion[];
  rooms: LauncherSuggestion[];
}
