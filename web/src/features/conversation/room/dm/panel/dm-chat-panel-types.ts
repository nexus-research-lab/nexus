import type {
  AgentConversationIdentity,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import type { ExecutionResource } from "@/features/conversation/shared/execution/use-execution-resource";
import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import type { Agent } from "@/types/agent/agent";
import type { SessionSnapshotPayload } from "@/types/conversation/conversation";
import type { TodoItem } from "@/types/conversation/todo";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

export interface DmChatPanelProps {
  currentAgent: Agent;
  executionResource: ExecutionResource;
  sessionIdentity: AgentConversationIdentity | null;
  runtimeKind: AgentRuntimeKind;
  todos: TodoItem[];
  layout: "desktop" | "mobile";
  embeddedEditor?: {
    placeholder: string;
    visibleAfterUnixMilli: number;
  };
  initialDraft?: string | null;
  onInitialDraftConsumed?: () => void;
  onExecutionTaskRunsChange?: (runs: ConversationTaskRun[]) => void;
  onBusyChange?: (busy: boolean) => void;
  onForkConversation?: (roundId: string) => Promise<void>;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
  onOpenWorkGraph?: () => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onTodosChange?: (todos: TodoItem[]) => void;
  onConversationSnapshotChange?: (snapshot: SessionSnapshotPayload) => void;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
}
