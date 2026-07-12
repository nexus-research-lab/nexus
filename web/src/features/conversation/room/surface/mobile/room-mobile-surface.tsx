"use client";

import { useMemo, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceTaskPanel } from "@/shared/ui/workspace/surface/workspace-task-strip";
import type { Agent } from "@/types/agent/agent";
import type {
  AgentConversationIdentity,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import type {
  ConversationSnapshotPayload,
  RoomConversationView,
} from "@/types/conversation/conversation";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";
import type { TodoItem } from "@/types/conversation/todo";

import { GroupThreadContextProvider } from "../../group/thread/group-thread-context";
import { RoomChatSurface } from "../room-chat-surface";
import { resolveRoomSubagentTaskSource } from "../room-surface-model";
import { RoomMobileConversationSheet } from "./room-mobile-conversation-sheet";
import { RoomMobileHeader } from "./room-mobile-header";
import { RoomMobileSubagentOverlay } from "./room-mobile-subagent-overlay";
import { RoomMobileThreadOverlay } from "./room-mobile-thread-overlay";

interface RoomMobileSurfaceProps {
  conversationId: string | null;
  currentAgent: Agent;
  currentAgentSessionIdentity: AgentConversationIdentity | null;
  currentRoomConversation: RoomConversationView | null;
  currentRoomConversations: RoomConversationView[];
  currentRoomTitle: string;
  currentRoomType: string;
  currentTodos: TodoItem[];
  initialDraft?: string | null;
  onBackToDirectory: () => void;
  onConversationSnapshotChange: (snapshot: ConversationSnapshotPayload) => void;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onInitialDraftConsumed?: () => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
  onSelectConversation: (conversationId: string) => void;
  onTodosChange: (todos: TodoItem[]) => void;
  roomHostAgentId: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomId: string | null;
  roomMembers: Agent[];
}

export function RoomMobileSurface({
  conversationId,
  currentAgent,
  currentAgentSessionIdentity,
  currentRoomConversation,
  currentRoomConversations,
  currentRoomTitle,
  currentRoomType,
  currentTodos,
  initialDraft = null,
  onBackToDirectory,
  onConversationSnapshotChange,
  onCreateConversation,
  onInitialDraftConsumed,
  onOpenWorkspaceFile,
  onRoomEvent,
  onSelectConversation,
  onTodosChange,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomId,
  roomMembers,
}: RoomMobileSurfaceProps) {
  const { t } = useI18n();
  const [isConversationSheetOpen, setIsConversationSheetOpen] = useState(false);
  const [openSubagentSource, setOpenSubagentSource] = useState<SubagentTaskSource | null>(null);
  const isDm = currentRoomType === "dm";
  const subagentTaskSource = useMemo(
    () => resolveRoomSubagentTaskSource({
      conversationId,
      isDm,
      roomId,
      sessionIdentity: currentAgentSessionIdentity,
    }),
    [conversationId, currentAgentSessionIdentity, isDm, roomId],
  );
  const conversationTitle = currentRoomConversation?.title?.trim()
    || t("room.new_conversation");
  const chatSurface = (
    <RoomChatSurface
      conversationId={conversationId}
      currentAgent={currentAgent}
      currentAgentSessionIdentity={currentAgentSessionIdentity}
      currentRoomType={currentRoomType}
      initialDraft={initialDraft}
      layout="mobile"
      onConversationSnapshotChange={onConversationSnapshotChange}
      onCreateConversation={onCreateConversation}
      onInitialDraftConsumed={onInitialDraftConsumed}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
      onRoomEvent={onRoomEvent}
      onTodosChange={onTodosChange}
      roomHostAgentId={roomHostAgentId}
      roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
      roomId={roomId}
      roomMembers={roomMembers}
    />
  );

  return (
    <section className="relative flex min-h-0 flex-1 flex-col overflow-hidden bg-background/90">
      <RoomMobileHeader
        agentAvatar={currentAgent.avatar}
        agentName={currentAgent.name}
        canOpenSubagents={subagentTaskSource !== null}
        conversationTitle={conversationTitle}
        onBack={onBackToDirectory}
        onOpenConversations={() => setIsConversationSheetOpen(true)}
        onOpenSubagents={() => setOpenSubagentSource(subagentTaskSource)}
        roomTitle={currentRoomTitle}
      />

      <div className="relative min-h-0 min-w-0 flex-1 overflow-hidden">
        {isDm ? chatSurface : (
          <GroupThreadContextProvider>
            {chatSurface}
            <RoomMobileThreadOverlay />
          </GroupThreadContextProvider>
        )}
        <WorkspaceTaskPanel
          key={conversationId ?? "mobile-conversation-tasks"}
          todos={currentTodos}
        />
      </div>

      <RoomMobileConversationSheet
        activeConversationId={conversationId}
        conversations={currentRoomConversations}
        isOpen={isConversationSheetOpen}
        onClose={() => setIsConversationSheetOpen(false)}
        onSelect={onSelectConversation}
      />

      <RoomMobileSubagentOverlay
        onClose={() => setOpenSubagentSource(null)}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        source={openSubagentSource === subagentTaskSource
          ? openSubagentSource
          : null}
      />
    </section>
  );
}
