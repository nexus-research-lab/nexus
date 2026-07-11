import type { RefObject } from "react";

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
import type { TodoItem } from "@/types/conversation/todo";

export interface RoomSurfaceLayoutProps {
  currentAgent: Agent;
  currentRoomType: string;
  roomId: string | null;
  roomAvatar?: string | null;
  roomMembers: Agent[];
  availableRoomAgents: Agent[];
  currentRoomTitle: string;
  roomSkillNames: string[];
  roomHostAgentId?: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomPrivateMessagesEnabled: boolean;
  currentAgentSessionIdentity: AgentConversationIdentity | null;
  conversationId: string | null;
  currentRoomConversations: RoomConversationView[];
  activeWorkspacePath: string | null;
  activeSurfaceTab: RoomSurfaceTabKey;
  initialDraft?: string | null;
  onInitialDraftConsumed?: () => void;
  isEditorOpen: boolean;
  editorWidthPercent: number;
  isResizingEditor: boolean;
  currentTodos: TodoItem[];
  workspaceSplitRef: RefObject<HTMLElement | null>;
  onReplayTour?: () => void;
  onChangeSurfaceTab: (tab: RoomSurfaceTabKey) => void;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
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
  onStartEditorResize: () => void;
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
