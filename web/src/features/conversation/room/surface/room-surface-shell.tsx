"use client";

import { useCallback, useState } from "react";

import { useMediaQuery } from "@/hooks/ui/use-media-query";
import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import { Agent, AgentIdentityDraft, AgentNameValidationResult, AgentOptions } from "@/types/agent/agent";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import { ConversationSnapshotPayload, RoomConversationView } from "@/types/conversation/conversation";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";
import { TodoItem } from "@/types/conversation/todo";

import { RoomMobileSurface } from "./mobile/room-mobile-surface";
import { RoomSurfaceLayout } from "./layout/room-surface-layout";

interface RoomSurfaceShellProps {
  currentAgent: Agent;
  currentRoomType: string;
  roomId: string | null;
  roomAvatar?: string | null;
  roomMembers: Agent[];
  availableRoomAgents: Agent[];
  currentRoomTitle: string;
  roomSkillNames: string[];
  roomHostAgentId: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomPrivateMessagesEnabled: boolean;
  currentRoomConversation: RoomConversationView | null;
  currentAgentSessionIdentity: AgentConversationIdentity | null;
  conversationId: string | null;
  currentRoomConversations: RoomConversationView[];
  activeWorkspacePath: string | null;
  initialDraft?: string | null;
  onInitialDraftConsumed?: () => void;
  sidePanelWidthPercent: number;
  isResizingSidePanel: boolean;
  currentTodos: TodoItem[];
  surfaceSplitRef: React.RefObject<HTMLElement | null>;
  onReplayTour?: () => void;
  onBackToDirectory: () => void;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onManageRoom: (submission: RoomDialogSubmission) => Promise<void>;
  onOpenMemberManager: () => Promise<void>;
  onSaveAgentOptions: (agentId: string, title: string, options: AgentOptions, identity: AgentIdentityDraft) => Promise<void>;
  onValidateAgentName: (name: string, agentId?: string) => Promise<AgentNameValidationResult>;
  onUpdateConversationTitle: (conversationId: string, title: string) => Promise<void>;
  onOpenWorkspaceFile: (path: string | null, workspaceAgentId?: string | null) => void;
  onStartSidePanelResize: () => void;
  onTodosChange: (todos: TodoItem[]) => void;
  onConversationSnapshotChange: (snapshot: ConversationSnapshotPayload) => void;
  onRoomEvent?: (eventType: string, data: import("@/types/agent/agent-conversation").RoomEventPayload) => void;
}

export function RoomSurfaceShell({
  currentAgent,
  currentRoomType,
  roomId,
  roomAvatar,
  roomMembers,
  availableRoomAgents,
  currentRoomTitle,
  roomSkillNames,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomPrivateMessagesEnabled,
  currentRoomConversation,
  currentAgentSessionIdentity,
  conversationId,
  currentRoomConversations,
  activeWorkspacePath,
  initialDraft,
  onInitialDraftConsumed,
  sidePanelWidthPercent,
  isResizingSidePanel,
  currentTodos,
  surfaceSplitRef,
  onReplayTour,
  onBackToDirectory,
  onCreateConversation,
  onSelectConversation,
  onCloseConversation,
  onDeleteConversation,
  onManageRoom,
  onOpenMemberManager,
  onSaveAgentOptions,
  onValidateAgentName,
  onUpdateConversationTitle,
  onOpenWorkspaceFile,
  onStartSidePanelResize,
  onTodosChange,
  onConversationSnapshotChange,
  onRoomEvent,
}: RoomSurfaceShellProps) {
  const isMobile = useMediaQuery("(max-width: 767px)");
  const [activeSurfaceTab, setActiveSurfaceTab] = useState<RoomSurfaceTabKey>("chat");

  const handleCreateConversationInShell = useCallback(async (title?: string) => {
    const nextConversationId = await onCreateConversation(title);
    setActiveSurfaceTab("chat");
    return nextConversationId;
  }, [onCreateConversation]);

  const handleOpenWorkspaceFileInShell = useCallback((path: string | null, workspaceAgentId?: string | null) => {
    onOpenWorkspaceFile(path, workspaceAgentId);
    if (path) {
      setActiveSurfaceTab("workspace");
    }
  }, [onOpenWorkspaceFile]);

  if (isMobile) {
    return (
      <RoomMobileSurface
        key={roomId ?? currentAgent.agent_id}
        currentAgent={currentAgent}
        currentRoomType={currentRoomType}
        roomId={roomId}
        roomMembers={roomMembers}
        roomHostAgentId={roomHostAgentId}
        roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
        currentRoomConversation={currentRoomConversation}
        currentAgentSessionIdentity={currentAgentSessionIdentity}
        conversationId={conversationId}
        currentRoomConversations={currentRoomConversations}
        currentRoomTitle={currentRoomTitle}
        currentTodos={currentTodos}
        initialDraft={initialDraft}
        onInitialDraftConsumed={onInitialDraftConsumed}
        onBackToDirectory={onBackToDirectory}
        onConversationSnapshotChange={onConversationSnapshotChange}
        onCreateConversation={handleCreateConversationInShell}
        onOpenWorkspaceFile={(path, workspaceAgentId) =>
          onOpenWorkspaceFile(path, workspaceAgentId)}
        onRoomEvent={onRoomEvent}
        onSelectConversation={onSelectConversation}
        onTodosChange={onTodosChange}
      />
    );
  }

  return (
    <RoomSurfaceLayout
      activeWorkspacePath={activeWorkspacePath}
      activeSurfaceTab={activeSurfaceTab}
      availableRoomAgents={availableRoomAgents}
      currentAgent={currentAgent}
      currentRoomType={currentRoomType}
      roomId={roomId}
      roomAvatar={roomAvatar}
      roomMembers={roomMembers}
      currentRoomTitle={currentRoomTitle}
      roomSkillNames={roomSkillNames}
      roomHostAgentId={roomHostAgentId}
      roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
      roomPrivateMessagesEnabled={roomPrivateMessagesEnabled}
      currentAgentSessionIdentity={currentAgentSessionIdentity}
      conversationId={conversationId}
      currentRoomConversations={currentRoomConversations}
      initialDraft={initialDraft}
      onInitialDraftConsumed={onInitialDraftConsumed}
      currentTodos={currentTodos}
      sidePanelWidthPercent={sidePanelWidthPercent}
      isResizingSidePanel={isResizingSidePanel}
      onReplayTour={onReplayTour}
      onManageRoom={onManageRoom}
      onOpenMemberManager={onOpenMemberManager}
      onSaveAgentOptions={onSaveAgentOptions}
      onValidateAgentName={onValidateAgentName}
      onChangeSurfaceTab={setActiveSurfaceTab}
      onConversationSnapshotChange={onConversationSnapshotChange}
      onCreateConversation={handleCreateConversationInShell}
      onCloseConversation={onCloseConversation}
      onDeleteConversation={onDeleteConversation}
      onOpenWorkspaceFile={handleOpenWorkspaceFileInShell}
      onUpdateConversationTitle={onUpdateConversationTitle}
      onSelectConversation={onSelectConversation}
      onStartSidePanelResize={onStartSidePanelResize}
      onTodosChange={onTodosChange}
      surfaceSplitRef={surfaceSplitRef}
      onRoomEvent={onRoomEvent}
    />
  );
}
