import type { Agent } from "@/types/agent/agent";
import type { ExecutionResource } from "@/features/conversation/shared/execution/use-execution-resource";
import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import type { RoomEventPayload } from "@/types/agent/agent-conversation";
import type { RoomConversationSnapshotPayload } from "@/types/conversation/conversation";
import type { TodoItem } from "@/types/conversation/todo";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

export interface GroupChatPanelProps {
  agentId: string | null;
  conversationId: string | null;
  currentAgentAvatar: string | null;
  currentAgentName: string | null;
  executionResource: ExecutionResource;
  initialDraft?: string | null;
  layout: "desktop" | "mobile";
  onConversationSnapshotChange?: (
    snapshot: RoomConversationSnapshotPayload,
  ) => void;
  onCreateConversation: (title?: string) => void | Promise<string | null>;
  onInitialDraftConsumed?: () => void;
  onExecutionTaskRunsChange?: (runs: ConversationTaskRun[]) => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkGraph?: () => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
  onTodosChange?: (todos: TodoItem[]) => void;
  roomHostAgentId: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomId: string | null;
  roomMembers: Agent[];
  runtimeKind: AgentRuntimeKind;
}
