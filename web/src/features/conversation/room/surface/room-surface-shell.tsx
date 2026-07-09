"use client";

import { useCallback, useState } from "react";

import { useMediaQuery } from "@/hooks/ui/use-media-query";
import { Agent, AgentIdentityDraft, AgentNameValidationResult, AgentOptions } from "@/types/agent/agent";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import { ConversationSnapshotPayload, RoomConversationView } from "@/types/conversation/conversation";
import { RoomSurfaceTabKey } from "@/types/conversation/room-surface";
import { TodoItem } from "@/types/conversation/todo";
import { UpdateRoomParams } from "@/types/conversation/room";

import { RoomMobileSurface } from "./room-mobile-surface";
import { RoomSurfaceLayout } from "./room-surface-layout";

interface RoomSurfaceShellProps {
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
  currentRoomConversation: RoomConversationView | null;
  currentAgentSessionIdentity: AgentConversationIdentity | null;
  conversationId: string | null;
  currentRoomConversations: RoomConversationView[];
  activeWorkspacePath: string | null;
  initialDraft?: string | null;
  onInitialDraftConsumed?: () => void;
  isEditorOpen: boolean;
  editorWidthPercent: number;
  isResizingEditor: boolean;
  isConversationBusy: boolean;
  currentTodos: TodoItem[];
  workspaceSplitRef: React.RefObject<HTMLElement | null>;
  onReplayTour?: () => void;
  onBackToDirectory: () => void;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onAddRoomMember: (agentId: string) => Promise<void>;
  onRemoveRoomMember: (agentId: string) => Promise<void>;
  onOpenMemberManager: () => Promise<void>;
  onSaveAgentOptions: (agentId: string, title: string, options: AgentOptions, identity: AgentIdentityDraft) => Promise<void>;
  onValidateAgentName: (name: string, agentId?: string) => Promise<AgentNameValidationResult>;
  onUpdateRoom: (roomId: string, params: UpdateRoomParams) => Promise<void>;
  onUpdateConversationTitle: (conversationId: string, title: string) => Promise<void>;
  onOpenWorkspaceFile: (path: string | null, workspaceAgentId?: string | null) => void;
  onStartEditorResize: () => void;
  onLoadingChange: (isLoading: boolean) => void;
  onTodosChange: (todos: TodoItem[]) => void;
  onConversationSnapshotChange: (snapshot: ConversationSnapshotPayload) => void;
  onRoomEvent?: (eventType: string, data: import("@/types/agent/agent-conversation").RoomEventPayload) => void;
}

export function RoomSurfaceShell({
  currentAgent: currentAgent,
  currentRoomType: currentRoomType,
  roomId: roomId,
  roomAvatar: roomAvatar,
  roomMembers: roomMembers,
  availableRoomAgents: availableRoomAgents,
  currentRoomTitle: currentRoomTitle,
  roomSkillNames: roomSkillNames,
  roomHostAgentId: roomHostAgentId,
  roomHostAutoReplyEnabled: roomHostAutoReplyEnabled,
  roomPrivateMessagesEnabled: roomPrivateMessagesEnabled,
  currentRoomConversation: currentRoomConversation,
  currentAgentSessionIdentity: currentAgentSessionIdentity,
  conversationId: conversationId,
  currentRoomConversations: currentRoomConversations,
  activeWorkspacePath: activeWorkspacePath,
  initialDraft: initialDraft,
  onInitialDraftConsumed: onInitialDraftConsumed,
  isEditorOpen: isEditorOpen,
  editorWidthPercent: editorWidthPercent,
  isResizingEditor: isResizingEditor,
  isConversationBusy: isConversationBusy,
  currentTodos: currentTodos,
  workspaceSplitRef: workspaceSplitRef,
  onReplayTour: onReplayTour,
  onBackToDirectory: onBackToDirectory,
  onCreateConversation: onCreateConversation,
  onSelectConversation: onSelectConversation,
  onCloseConversation: onCloseConversation,
  onDeleteConversation: onDeleteConversation,
  onAddRoomMember: onAddRoomMember,
  onRemoveRoomMember: onRemoveRoomMember,
  onOpenMemberManager: onOpenMemberManager,
  onSaveAgentOptions: onSaveAgentOptions,
  onValidateAgentName: onValidateAgentName,
  onUpdateRoom: onUpdateRoom,
  onUpdateConversationTitle: onUpdateConversationTitle,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  onStartEditorResize: onStartEditorResize,
  onLoadingChange: onLoadingChange,
  onTodosChange: onTodosChange,
  onConversationSnapshotChange: onConversationSnapshotChange,
  onRoomEvent: onRoomEvent,
}: RoomSurfaceShellProps) {
  const isMobile = useMediaQuery("(max-width: 767px)");
  const [activeSurfaceTab, setActiveSurfaceTab] = useState<RoomSurfaceTabKey>("chat");

  const handleSelectConversationInShell = useCallback((conversationId: string) => {
    onSelectConversation(conversationId);
  }, [onSelectConversation]);

  const handleChangeSurfaceTab = useCallback((nextTab: RoomSurfaceTabKey) => {
    setActiveSurfaceTab(nextTab);
  }, []);

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
        initialDraft={initialDraft}
        onInitialDraftConsumed={onInitialDraftConsumed}
        onBackToDirectory={onBackToDirectory}
        onConversationSnapshotChange={onConversationSnapshotChange}
        onCreateConversation={handleCreateConversationInShell}
        onLoadingChange={onLoadingChange}
        onRoomEvent={onRoomEvent}
        onSelectConversation={handleSelectConversationInShell}
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
      editorWidthPercent={editorWidthPercent}
      isEditorOpen={isEditorOpen}
      isResizingEditor={isResizingEditor}
      isConversationBusy={isConversationBusy}
      onReplayTour={onReplayTour}
      onAddRoomMember={onAddRoomMember}
      onOpenMemberManager={onOpenMemberManager}
      onRemoveRoomMember={onRemoveRoomMember}
      onSaveAgentOptions={onSaveAgentOptions}
      onValidateAgentName={onValidateAgentName}
      onChangeSurfaceTab={handleChangeSurfaceTab}
      onConversationSnapshotChange={onConversationSnapshotChange}
      onCreateConversation={handleCreateConversationInShell}
      onCloseConversation={onCloseConversation}
      onDeleteConversation={onDeleteConversation}
      onLoadingChange={onLoadingChange}
      onOpenWorkspaceFile={handleOpenWorkspaceFileInShell}
      onUpdateRoom={onUpdateRoom}
      onUpdateConversationTitle={onUpdateConversationTitle}
      onSelectConversation={handleSelectConversationInShell}
      onStartEditorResize={onStartEditorResize}
      onTodosChange={onTodosChange}
      workspaceSplitRef={workspaceSplitRef}
      onRoomEvent={onRoomEvent}
    />
  );
}
