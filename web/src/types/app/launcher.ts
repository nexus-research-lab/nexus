import { Agent } from "@/types/agent/agent";
import { Conversation } from "@/types/conversation/conversation";

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

