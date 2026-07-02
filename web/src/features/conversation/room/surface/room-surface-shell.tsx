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
  current_agent: Agent;
  current_room_type: string;
  room_id: string | null;
  room_avatar?: string | null;
  room_members: Agent[];
  available_room_agents: Agent[];
  current_room_title: string;
  room_skill_names: string[];
  room_host_agent_id?: string | null;
  room_host_auto_reply_enabled: boolean;
  room_private_messages_enabled: boolean;
  current_room_conversation: RoomConversationView | null;
  current_agent_session_identity: AgentConversationIdentity | null;
  conversation_id: string | null;
  current_room_conversations: RoomConversationView[];
  active_workspace_path: string | null;
  initial_draft?: string | null;
  on_initial_draft_consumed?: () => void;
  is_editor_open: boolean;
  editor_width_percent: number;
  is_resizing_editor: boolean;
  is_conversation_busy: boolean;
  current_todos: TodoItem[];
  workspace_split_ref: React.RefObject<HTMLElement | null>;
  on_replay_tour?: () => void;
  on_back_to_directory: () => void;
  on_create_conversation: (title?: string) => Promise<string | null>;
  on_select_conversation: (conversationId: string) => void;
  on_close_conversation: (conversationId: string) => Promise<void>;
  on_delete_conversation: (conversationId: string) => Promise<string | null>;
  on_add_room_member: (agentId: string) => Promise<void>;
  on_remove_room_member: (agentId: string) => Promise<void>;
  on_open_member_manager: () => Promise<void>;
  on_save_agent_options: (agentId: string, title: string, options: AgentOptions, identity: AgentIdentityDraft) => Promise<void>;
  on_validate_agent_name: (name: string, agentId?: string) => Promise<AgentNameValidationResult>;
  on_update_room: (roomId: string, params: UpdateRoomParams) => Promise<void>;
  on_update_conversation_title: (conversationId: string, title: string) => Promise<void>;
  on_open_workspace_file: (path: string | null) => void;
  on_start_editor_resize: () => void;
  on_loading_change: (isLoading: boolean) => void;
  on_todos_change: (todos: TodoItem[]) => void;
  on_conversation_snapshot_change: (snapshot: ConversationSnapshotPayload) => void;
  on_room_event?: (eventType: string, data: import("@/types/agent/agent-conversation").RoomEventPayload) => void;
}

export function RoomSurfaceShell({
  current_agent: currentAgent,
  current_room_type: currentRoomType,
  room_id: roomId,
  room_avatar: roomAvatar,
  room_members: roomMembers,
  available_room_agents: availableRoomAgents,
  current_room_title: currentRoomTitle,
  room_skill_names: roomSkillNames,
  room_host_agent_id: roomHostAgentId,
  room_host_auto_reply_enabled: roomHostAutoReplyEnabled,
  room_private_messages_enabled: roomPrivateMessagesEnabled,
  current_room_conversation: currentRoomConversation,
  current_agent_session_identity: currentAgentSessionIdentity,
  conversation_id: conversationId,
  current_room_conversations: currentRoomConversations,
  active_workspace_path: activeWorkspacePath,
  initial_draft: initialDraft,
  on_initial_draft_consumed: onInitialDraftConsumed,
  is_editor_open: isEditorOpen,
  editor_width_percent: editorWidthPercent,
  is_resizing_editor: isResizingEditor,
  is_conversation_busy: isConversationBusy,
  current_todos: currentTodos,
  workspace_split_ref: workspaceSplitRef,
  on_replay_tour: onReplayTour,
  on_back_to_directory: onBackToDirectory,
  on_create_conversation: onCreateConversation,
  on_select_conversation: onSelectConversation,
  on_close_conversation: onCloseConversation,
  on_delete_conversation: onDeleteConversation,
  on_add_room_member: onAddRoomMember,
  on_remove_room_member: onRemoveRoomMember,
  on_open_member_manager: onOpenMemberManager,
  on_save_agent_options: onSaveAgentOptions,
  on_validate_agent_name: onValidateAgentName,
  on_update_room: onUpdateRoom,
  on_update_conversation_title: onUpdateConversationTitle,
  on_open_workspace_file: onOpenWorkspaceFile,
  on_start_editor_resize: onStartEditorResize,
  on_loading_change: onLoadingChange,
  on_todos_change: onTodosChange,
  on_conversation_snapshot_change: onConversationSnapshotChange,
  on_room_event: onRoomEvent,
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

  const handleOpenWorkspaceFileInShell = useCallback((path: string | null) => {
    onOpenWorkspaceFile(path);
    if (path) {
      setActiveSurfaceTab("workspace");
    }
  }, [onOpenWorkspaceFile]);

  if (isMobile) {
    return (
      <RoomMobileSurface
        current_agent={currentAgent}
        current_room_type={currentRoomType}
        room_id={roomId}
        room_members={roomMembers}
        room_host_agent_id={roomHostAgentId}
        room_host_auto_reply_enabled={roomHostAutoReplyEnabled}
        current_room_conversation={currentRoomConversation}
        current_agent_session_identity={currentAgentSessionIdentity}
        conversation_id={conversationId}
        current_room_conversations={currentRoomConversations}
        current_room_title={currentRoomTitle}
        initial_draft={initialDraft}
        on_initial_draft_consumed={onInitialDraftConsumed}
        on_back_to_directory={onBackToDirectory}
        on_conversation_snapshot_change={onConversationSnapshotChange}
        on_create_conversation={handleCreateConversationInShell}
        on_loading_change={onLoadingChange}
        on_room_event={onRoomEvent}
        on_select_conversation={handleSelectConversationInShell}
      />
    );
  }

  return (
    <RoomSurfaceLayout
      active_workspace_path={activeWorkspacePath}
      active_surface_tab={activeSurfaceTab}
      available_room_agents={availableRoomAgents}
      current_agent={currentAgent}
      current_room_type={currentRoomType}
      room_id={roomId}
      room_avatar={roomAvatar}
      room_members={roomMembers}
      current_room_title={currentRoomTitle}
      room_skill_names={roomSkillNames}
      room_host_agent_id={roomHostAgentId}
      room_host_auto_reply_enabled={roomHostAutoReplyEnabled}
      room_private_messages_enabled={roomPrivateMessagesEnabled}
      current_agent_session_identity={currentAgentSessionIdentity}
      conversation_id={conversationId}
      current_room_conversations={currentRoomConversations}
      initial_draft={initialDraft}
      on_initial_draft_consumed={onInitialDraftConsumed}
      current_todos={currentTodos}
      editor_width_percent={editorWidthPercent}
      is_editor_open={isEditorOpen}
      is_resizing_editor={isResizingEditor}
      is_conversation_busy={isConversationBusy}
      on_replay_tour={onReplayTour}
      on_add_room_member={onAddRoomMember}
      on_open_member_manager={onOpenMemberManager}
      on_remove_room_member={onRemoveRoomMember}
      on_save_agent_options={onSaveAgentOptions}
      on_validate_agent_name={onValidateAgentName}
      on_change_surface_tab={handleChangeSurfaceTab}
      on_conversation_snapshot_change={onConversationSnapshotChange}
      on_create_conversation={handleCreateConversationInShell}
      on_close_conversation={onCloseConversation}
      on_delete_conversation={onDeleteConversation}
      on_loading_change={onLoadingChange}
      on_open_workspace_file={handleOpenWorkspaceFileInShell}
      on_update_room={onUpdateRoom}
      on_update_conversation_title={onUpdateConversationTitle}
      on_select_conversation={handleSelectConversationInShell}
      on_start_editor_resize={onStartEditorResize}
      on_todos_change={onTodosChange}
      workspace_split_ref={workspaceSplitRef}
      on_room_event={onRoomEvent}
    />
  );
}
