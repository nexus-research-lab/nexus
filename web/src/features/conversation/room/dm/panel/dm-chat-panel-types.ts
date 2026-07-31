import type {
  AgentConversationIdentity,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import type { Agent } from "@/types/agent/agent";
import type { SessionSnapshotPayload } from "@/types/conversation/conversation";
import type { TodoItem } from "@/types/conversation/todo";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

export interface DmChatPanelProps {
  currentAgent: Agent;
  sessionIdentity: AgentConversationIdentity | null;
  runtimeKind: AgentRuntimeKind;
  todos: TodoItem[];
  layout: "desktop" | "mobile";
  initialDraft?: string | null;
  onInitialDraftConsumed?: () => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onTodosChange?: (todos: TodoItem[]) => void;
  onConversationSnapshotChange?: (snapshot: SessionSnapshotPayload) => void;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
}
