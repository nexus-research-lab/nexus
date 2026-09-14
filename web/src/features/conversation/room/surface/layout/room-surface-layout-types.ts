// INPUT: Room 桌面装配需要的数据、身份和已绑定命令。
// OUTPUT: 窄 Surface 参数合同，包括交给原 owner 的鼠标/键盘尺寸请求。
// POS: Room 布局消费侧类型；不定义业务真相或新的宽度状态。

import type { RefObject } from "react";

import type { ExecutionResource } from "@/features/conversation/shared/execution/use-execution-resource";
import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import type {
  Agent,
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions,
} from "@/types/agent/agent";
import type {
  AgentConversationIdentity,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import type {
  ConversationSnapshotPayload,
  RoomConversationView,
} from "@/types/conversation/conversation";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";
import type { FinalConversationReplacementHandler } from "@/features/navigation/conversation-tabs/final-conversation-replacement";
import type { TodoItem } from "@/types/conversation/todo";
import type { AgentRuntimeKind } from "@/types/settings/preferences";
import type { ResourceFailure } from "@/lib/error-message";

export interface RoomExternalSessionsReliability {
  failure: ResourceFailure | null;
  isLoading: boolean;
  isStale: boolean;
  refresh: () => void;
}

export interface RoomSurfaceLayoutProps {
  currentAgent: Agent;
  currentRoomType: string;
  roomId: string | null;
  roomAvatar?: string | null;
  roomMembers: Agent[];
  availableRoomAgents: Agent[];
  currentRoomTitle: string;
  runtimeKind: AgentRuntimeKind;
  roomSkillNames: string[];
  roomHostAgentId: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomPrivateMessagesEnabled: boolean;
  currentAgentSessionIdentity: AgentConversationIdentity | null;
  conversationId: string | null;
  currentRoomConversations: RoomConversationView[];
  activeWorkspacePath: string | null;
  activeSurfaceTab: RoomSurfaceTabKey;
  initialDraft?: string | null;
  onInitialDraftConsumed?: () => void;
  executionResource: ExecutionResource;
  executionTaskRuns: ConversationTaskRun[];
  externalSessionsReliability: RoomExternalSessionsReliability;
  sidePanelWidthPercent: number;
  isResizingSidePanel: boolean;
  currentTodos: TodoItem[];
  surfaceSplitRef: RefObject<HTMLElement | null>;
  onExecutionTaskRunsChange: (runs: ConversationTaskRun[]) => void;
  onChangeSurfaceTab: (tab: RoomSurfaceTabKey) => void;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onReplaceFinalConversation: FinalConversationReplacementHandler;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onForkConversation: (roundId: string) => Promise<void>;
  onManageRoom: (submission: RoomDialogSubmission) => Promise<void>;
  onOpenMemberManager: () => Promise<void>;
  onSaveAgentOptions: (
    agentId: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => Promise<void>;
  onValidateAgentName: (
    name: string,
    agentId?: string,
  ) => Promise<AgentNameValidationResult>;
  onUpdateConversationTitle: (
    conversationId: string,
    title: string,
  ) => Promise<void>;
  onOpenWorkspaceFile: (
    path: string | null,
    workspaceAgentId?: string | null,
  ) => void;
  onStartSidePanelResize: () => void;
  onSidePanelWidthChange: (percent: number) => void;
  onTodosChange: (todos: TodoItem[]) => void;
  onConversationSnapshotChange: (
    snapshot: ConversationSnapshotPayload,
  ) => void;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
}

export type RoomAgentAboutRequest = {
  agent_id: string | null;
  tab: "identity" | "private_domain";
  key: number;
};
