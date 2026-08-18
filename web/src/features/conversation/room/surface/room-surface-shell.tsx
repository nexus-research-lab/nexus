"use client";

/**
 * INPUT: 当前 Room/DM conversation identity、共享聊天 surface 与 session-scoped realtime events。
 * OUTPUT: 聊天/工作区/WorkGraph 共用布局，以及只由 execution_invalidated 驱动的 ExecutionResource revision。
 * POS: Room 页面桌面与移动 Surface 的资源组合根；不从 message/round/Goal 活动猜测图变化。
 */
import { useCallback, useState } from "react";

import { useExecutionResource } from "@/features/conversation/shared/execution/use-execution-resource";
import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { useMediaQuery } from "@/hooks/ui/use-media-query";
import { useDefaultAgentRuntimeKind } from "@/hooks/settings/use-default-agent-runtime-kind";
import { CONVERSATION_FOCUS_MEDIA_QUERY } from "@/lib/layout/home-layout";
import { buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import type { FinalConversationReplacementHandler } from "@/shared/ui/workspace/controls/conversation-tabs/final-conversation-replacement";
import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import { Agent, AgentIdentityDraft, AgentNameValidationResult, AgentOptions } from "@/types/agent/agent";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import { ConversationSnapshotPayload, RoomConversationView } from "@/types/conversation/conversation";
import { normalizeAgentRuntimeKind } from "@/types/settings/preferences";
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
  onBackToDirectory: () => void;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onReplaceFinalConversation: FinalConversationReplacementHandler;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onForkConversation: (
    conversationId: string,
    roundId: string,
  ) => Promise<string | null>;
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
  onBackToDirectory,
  onCreateConversation,
  onReplaceFinalConversation,
  onSelectConversation,
  onCloseConversation,
  onDeleteConversation,
  onForkConversation,
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
  const isConversationFocusMode = useMediaQuery(
    CONVERSATION_FOCUS_MEDIA_QUERY,
  );
  const defaultRuntimeKind = useDefaultAgentRuntimeKind();
  const [activeSurfaceTab, setActiveSurfaceTab] = useState<RoomSurfaceTabKey>("chat");
  const [executionEventRevision, setExecutionEventRevision] = useState(0);
  const handleRoomEvent = useCallback<NonNullable<RoomSurfaceShellProps["onRoomEvent"]>>(
    (eventType, data) => {
      if (eventType === "execution_invalidated") {
        setExecutionEventRevision((current) => current + 1);
      }
      onRoomEvent?.(eventType, data);
    },
    [onRoomEvent],
  );
  const storedRuntimeKind = currentRoomConversation?.options.runtime_kind;
  const runtimeKind = typeof storedRuntimeKind === "string"
    ? normalizeAgentRuntimeKind(storedRuntimeKind)
    : defaultRuntimeKind;
  const executionSessionKey = currentRoomType === "dm"
    ? currentAgentSessionIdentity?.session_key ?? null
    : conversationId
    ? buildRoomSharedSessionKey(conversationId)
    : null;
  const [executionTaskRunState, setExecutionTaskRunState] = useState<{
    sessionKey: string | null;
    runs: ConversationTaskRun[];
  }>({ sessionKey: null, runs: [] });
  const executionTaskRuns = executionTaskRunState.sessionKey === executionSessionKey
    ? executionTaskRunState.runs
    : [];
  const executionResource = useExecutionResource({
    invalidationKey: executionEventRevision,
    sessionKey: executionSessionKey,
  });
  const handleExecutionTaskRunsChange = useCallback((runs: ConversationTaskRun[]) => {
    setExecutionTaskRunState({ sessionKey: executionSessionKey, runs });
  }, [executionSessionKey]);

  const handleCreateConversationInShell = useCallback(async (title?: string) => {
    const nextConversationId = await onCreateConversation(title);
    setActiveSurfaceTab("chat");
    return nextConversationId;
  }, [onCreateConversation]);

  const handleReplaceFinalConversationInShell = useCallback<FinalConversationReplacementHandler>((
    conversation,
    commitConversation,
  ) => onReplaceFinalConversation(
    conversation,
    (conversationId) => {
      setActiveSurfaceTab("chat");
      commitConversation(conversationId);
    },
  ), [onReplaceFinalConversation]);

  const handleForkConversationInShell = useCallback(async (roundId: string) => {
    if (!conversationId) {
      return;
    }
    const nextConversationId = await onForkConversation(conversationId, roundId);
    if (!nextConversationId) {
      return;
    }
    setActiveSurfaceTab("chat");
    onSelectConversation(nextConversationId);
  }, [conversationId, onForkConversation, onSelectConversation]);

  const handleOpenWorkspaceFileInShell = useCallback((path: string | null, workspaceAgentId?: string | null) => {
    onOpenWorkspaceFile(path, workspaceAgentId);
    if (path) {
      setActiveSurfaceTab("workspace");
    }
  }, [onOpenWorkspaceFile]);

  if (isConversationFocusMode) {
    return (
      <RoomMobileSurface
        activeWorkspacePath={activeWorkspacePath}
        availableRoomAgents={availableRoomAgents}
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
        runtimeKind={runtimeKind}
        currentTodos={currentTodos}
        executionResource={executionResource}
        executionTaskRuns={executionTaskRuns}
        initialDraft={initialDraft}
        onExecutionTaskRunsChange={handleExecutionTaskRunsChange}
        onInitialDraftConsumed={onInitialDraftConsumed}
        onManageRoom={onManageRoom}
        onOpenMemberManager={onOpenMemberManager}
        onBackToDirectory={onBackToDirectory}
        onConversationSnapshotChange={onConversationSnapshotChange}
        onCreateConversation={handleCreateConversationInShell}
        onDeleteConversation={onDeleteConversation}
        onForkConversation={handleForkConversationInShell}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        onRoomEvent={handleRoomEvent}
        onSaveAgentOptions={onSaveAgentOptions}
        onSelectConversation={onSelectConversation}
        onTodosChange={onTodosChange}
        onUpdateConversationTitle={onUpdateConversationTitle}
        onValidateAgentName={onValidateAgentName}
        roomAvatar={roomAvatar}
        roomPrivateMessagesEnabled={roomPrivateMessagesEnabled}
        roomSkillNames={roomSkillNames}
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
      runtimeKind={runtimeKind}
      roomSkillNames={roomSkillNames}
      roomHostAgentId={roomHostAgentId}
      roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
      roomPrivateMessagesEnabled={roomPrivateMessagesEnabled}
      currentAgentSessionIdentity={currentAgentSessionIdentity}
      conversationId={conversationId}
      currentRoomConversations={currentRoomConversations}
      executionResource={executionResource}
      executionTaskRuns={executionTaskRuns}
      initialDraft={initialDraft}
      onExecutionTaskRunsChange={handleExecutionTaskRunsChange}
      onInitialDraftConsumed={onInitialDraftConsumed}
      currentTodos={currentTodos}
      sidePanelWidthPercent={sidePanelWidthPercent}
      isResizingSidePanel={isResizingSidePanel}
      onManageRoom={onManageRoom}
      onOpenMemberManager={onOpenMemberManager}
      onSaveAgentOptions={onSaveAgentOptions}
      onValidateAgentName={onValidateAgentName}
      onChangeSurfaceTab={setActiveSurfaceTab}
      onConversationSnapshotChange={onConversationSnapshotChange}
      onCreateConversation={handleCreateConversationInShell}
      onReplaceFinalConversation={handleReplaceFinalConversationInShell}
      onCloseConversation={onCloseConversation}
      onDeleteConversation={onDeleteConversation}
      onForkConversation={handleForkConversationInShell}
      onOpenWorkspaceFile={handleOpenWorkspaceFileInShell}
      onUpdateConversationTitle={onUpdateConversationTitle}
      onSelectConversation={onSelectConversation}
      onStartSidePanelResize={onStartSidePanelResize}
      onTodosChange={onTodosChange}
      surfaceSplitRef={surfaceSplitRef}
      onRoomEvent={handleRoomEvent}
    />
  );
}
