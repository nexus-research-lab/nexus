import type { Agent } from "@/types/agent/agent";
import type { RoomEventPayload } from "@/types/agent/agent-conversation";
import type { RoomConversationSnapshotPayload } from "@/types/conversation/conversation";
import type { TodoItem } from "@/types/conversation/todo";

export interface GroupChatPanelProps {
  agentId: string | null;
  conversationId: string | null;
  currentAgentAvatar: string | null;
  currentAgentName: string | null;
  initialDraft?: string | null;
  layout: "desktop" | "mobile";
  onConversationSnapshotChange?: (
    snapshot: RoomConversationSnapshotPayload,
  ) => void;
  onCreateConversation: (title?: string) => void | Promise<string | null>;
  onInitialDraftConsumed?: () => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
  onTodosChange?: (todos: TodoItem[]) => void;
  roomHostAgentId: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomId: string | null;
  roomMembers: Agent[];
}
