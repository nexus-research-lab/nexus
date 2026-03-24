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

export interface SpotlightTokenPileProps {
  tokens: SpotlightToken[];
  current_agent_id: string | null;
  on_select_agent: (agent_id: string) => void;
}

export interface EditableShapeOptions {
  defaultPoints: BlobPoint[];
  storageKey: string;
  viewBoxWidth: number;
  viewBoxHeight: number;
}
