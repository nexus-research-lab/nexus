import type {
  AgentConversationIdentity,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import type { SessionSnapshotPayload } from "@/types/conversation/conversation";
import type { TodoItem } from "@/types/conversation/todo";

export interface DmChatPanelProps {
  currentAgentName: string | null;
  currentAgentAvatar: string | null;
  currentAgentPermissionMode: string | null;
  sessionIdentity: AgentConversationIdentity | null;
  layout: "desktop" | "mobile";
  initialDraft?: string | null;
  onInitialDraftConsumed?: () => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onTodosChange?: (todos: TodoItem[]) => void;
  onConversationSnapshotChange?: (snapshot: SessionSnapshotPayload) => void;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
}
