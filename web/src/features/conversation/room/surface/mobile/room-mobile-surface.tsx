"use client";

/**
 * INPUT: 移动端 Room 会话、任务快照、导航与 Overlay 命令。
 * OUTPUT: 将任务快照交给聊天 Bottom Dock，并常驻暴露可打开统一空态的移动端工作图。
 * POS: Room 移动端 Surface 的主装配层。
 */

import { useMemo, useState } from "react";

import { buildComposerDraftScopeKey } from "@/features/conversation/shared/composer/composer-draft-scope";
import type { ExecutionResource } from "@/features/conversation/shared/execution/use-execution-resource";
import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import { RoomMemberManagerDialog } from "@/features/conversation/room/members/room-member-manager-dialog";
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
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";
import type { TodoItem } from "@/types/conversation/todo";
import type { AgentRuntimeKind } from "@/types/settings/preferences";
import { buildRoomSharedSessionKey } from "@/lib/conversation/session-key";

import { GroupThreadContextProvider } from "../../group/thread/group-thread-context";
import { RoomHistoryMenu } from "../history/room-history-menu";
import { RoomChatSurface } from "../room-chat-surface";
import { resolveRoomSubagentTaskSource } from "../room-surface-model";
import { RoomMobileActionsMenu } from "./room-mobile-actions-menu";
import {
  RoomMobileAuxiliaryOverlay,
  type RoomMobileAuxiliaryTab,
} from "./room-mobile-auxiliary-overlay";
import { RoomMobileConversationSwitcher } from "./room-mobile-conversation-switcher";
import { RoomMobileHeader } from "./room-mobile-header";
import { RoomMobileSubagentOverlay } from "./room-mobile-subagent-overlay";
import { RoomMobileThreadOverlay } from "./room-mobile-thread-overlay";
import type { RoomExternalSessionsReliability } from "../layout/room-surface-layout-types";
import { ReadResourceReliabilityNotice } from "@/features/conversation/shared/read-resource-reliability-notice";

interface RoomMobileSurfaceProps {
  activeWorkspacePath: string | null;
  availableRoomAgents: Agent[];
  conversationId: string | null;
  currentAgent: Agent;
  currentAgentSessionIdentity: AgentConversationIdentity | null;
  currentRoomConversation: RoomConversationView | null;
  currentRoomConversations: RoomConversationView[];
  currentRoomTitle: string;
  currentRoomType: string;
  currentTodos: TodoItem[];
  executionResource: ExecutionResource;
  executionTaskRuns: ConversationTaskRun[];
  externalSessionsReliability: RoomExternalSessionsReliability;
  initialDraft?: string | null;
  onBackToDirectory: () => void;
  onConversationSnapshotChange: (snapshot: ConversationSnapshotPayload) => void;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onExecutionTaskRunsChange: (runs: ConversationTaskRun[]) => void;
  onForkConversation: (roundId: string) => Promise<void>;
  onInitialDraftConsumed?: () => void;
  onManageRoom: (submission: RoomDialogSubmission) => Promise<void>;
  onOpenMemberManager: () => Promise<void>;
  onOpenWorkspaceFile: (
    path: string | null,
    workspaceAgentId?: string | null,
  ) => void;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
  onSaveAgentOptions: (
    agentId: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => Promise<void>;
  onSelectConversation: (conversationId: string) => void;
  onTodosChange: (todos: TodoItem[]) => void;
  onUpdateConversationTitle: (
    conversationId: string,
    title: string,
  ) => Promise<void>;
  onValidateAgentName: (
    name: string,
    agentId?: string,
  ) => Promise<AgentNameValidationResult>;
  roomHostAgentId: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomAvatar?: string | null;
  roomId: string | null;
  roomMembers: Agent[];
  roomPrivateMessagesEnabled: boolean;
  roomSkillNames: string[];
  runtimeKind: AgentRuntimeKind;
}

export function RoomMobileSurface({
  activeWorkspacePath,
  availableRoomAgents,
  conversationId,
  currentAgent,
  currentAgentSessionIdentity,
  currentRoomConversation,
  currentRoomConversations,
  currentRoomTitle,
  currentRoomType,
  currentTodos,
  executionResource,
  executionTaskRuns,
  externalSessionsReliability,
  initialDraft = null,
  onBackToDirectory,
  onConversationSnapshotChange,
  onCreateConversation,
  onDeleteConversation,
  onExecutionTaskRunsChange,
  onForkConversation,
  onInitialDraftConsumed,
  onManageRoom,
  onOpenMemberManager,
  onOpenWorkspaceFile,
  onRoomEvent,
  onSaveAgentOptions,
  onSelectConversation,
  onTodosChange,
  onUpdateConversationTitle,
  onValidateAgentName,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomAvatar,
  roomId,
  roomMembers,
  roomPrivateMessagesEnabled,
  roomSkillNames,
  runtimeKind,
}: RoomMobileSurfaceProps) {
  const { t } = useI18n();
  const [activeAuxiliaryTab, setActiveAuxiliaryTab] = useState<RoomMobileAuxiliaryTab | null>(null);
  const [isConversationSwitcherOpen, setIsConversationSwitcherOpen] = useState(false);
  const [memberDialogRoomId, setMemberDialogRoomId] = useState<string | null>(null);
  const [openSubagentSource, setOpenSubagentSource] = useState<SubagentTaskSource | null>(null);
  const [subagentRequest, setSubagentRequest] = useState({
    hostAgentId: null as string | null,
    key: 0,
    toolUseId: null as string | null,
  });
  const isDm = currentRoomType === "dm";
  const composerSessionKey = isDm
    ? currentAgentSessionIdentity?.session_key?.trim() || null
    : conversationId
    ? buildRoomSharedSessionKey(conversationId)
    : null;
  const composerDraftScopeKey = buildComposerDraftScopeKey({
    agentId: isDm ? currentAgent.agent_id : null,
    roomId: isDm ? null : roomId,
    sessionKey: composerSessionKey,
  });
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
  const handleOpenMemberList = async () => {
    const scopeRoomId = roomId;
    if (!scopeRoomId || isDm) {
      return;
    }
    await onOpenMemberManager();
    setMemberDialogRoomId(scopeRoomId);
  };
  const handleOpenAuxiliaryTab = (
    tab: "about" | "subagents" | "workgraph" | "workspace",
  ) => {
    if (tab === "subagents") {
      setActiveAuxiliaryTab(null);
      setSubagentRequest((current) => ({
        hostAgentId: null,
        key: current.key + 1,
        toolUseId: null,
      }));
      setOpenSubagentSource(subagentTaskSource);
      return;
    }
    setOpenSubagentSource(null);
    setActiveAuxiliaryTab(tab);
  };
  const handleOpenSubagentTask = (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => {
    const normalizedToolUseId = toolUseId.trim();
    if (!normalizedToolUseId || !subagentTaskSource) {
      return;
    }
    setActiveAuxiliaryTab(null);
    setSubagentRequest((current) => ({
      hostAgentId: hostAgentId?.trim() || null,
      key: current.key + 1,
      toolUseId: normalizedToolUseId,
    }));
    setOpenSubagentSource(subagentTaskSource);
  };
  const handleOpenWorkspaceFile = (
    path: string | null,
    workspaceAgentId?: string | null,
  ) => {
    onOpenWorkspaceFile(path, workspaceAgentId);
    if (path) {
      setOpenSubagentSource(null);
      setActiveAuxiliaryTab("workspace");
    }
  };
  const chatSurface = (
    <RoomChatSurface
      conversationId={conversationId}
      currentAgent={currentAgent}
      currentAgentSessionIdentity={currentAgentSessionIdentity}
      currentRoomType={currentRoomType}
      executionResource={executionResource}
      initialDraft={initialDraft}
      layout="mobile"
      onConversationSnapshotChange={onConversationSnapshotChange}
      onCreateConversation={onCreateConversation}
      onExecutionTaskRunsChange={onExecutionTaskRunsChange}
      onForkConversation={onForkConversation}
      onInitialDraftConsumed={onInitialDraftConsumed}
      onOpenSubagentTask={subagentTaskSource
        ? handleOpenSubagentTask
        : undefined}
      onOpenWorkGraph={() => handleOpenAuxiliaryTab("workgraph")}
      onOpenWorkspaceFile={handleOpenWorkspaceFile}
      onRoomEvent={onRoomEvent}
      onTodosChange={onTodosChange}
      roomHostAgentId={roomHostAgentId}
      roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
      roomId={roomId}
      roomMembers={roomMembers}
      runtimeKind={runtimeKind}
      todos={currentTodos}
    />
  );

  return (
    <section className="relative flex min-h-0 flex-1 flex-col overflow-hidden bg-background/90">
      <RoomMobileHeader
        conversationTitle={conversationTitle}
        isConversationSwitcherOpen={isConversationSwitcherOpen}
        onBack={onBackToDirectory}
        onOpenConversations={() => {
          setIsConversationSwitcherOpen((isOpen) => !isOpen);
        }}
        roomTitle={currentRoomTitle}
        trailing={(
          <>
            <RoomHistoryMenu
              conversationId={conversationId}
              conversations={currentRoomConversations}
              onCreateConversation={onCreateConversation}
              onDeleteConversation={onDeleteConversation}
              onSelectConversation={onSelectConversation}
              onUpdateConversationTitle={onUpdateConversationTitle}
              triggerVariant="history"
            />
            <RoomMobileActionsMenu
              canOpenSubagents={subagentTaskSource !== null}
              onCreateConversation={onCreateConversation}
              onManageMembers={!isDm && roomId
                ? () => void handleOpenMemberList()
                : undefined}
              onOpenAuxiliaryTab={handleOpenAuxiliaryTab}
            />
          </>
        )}
      />
      {isDm && externalSessionsReliability.failure ? (
        <ReadResourceReliabilityNotice
          impact={t(externalSessionsReliability.failure.access
            ? "conversation.external_sessions_access_impact"
            : externalSessionsReliability.isStale
            ? "conversation.external_sessions_stale_impact"
            : "conversation.external_sessions_unavailable_impact")}
          isRefreshing={externalSessionsReliability.isLoading}
          onRefresh={externalSessionsReliability.refresh}
          problem={t("conversation.external_sessions_refresh_failed")}
          resource="room-external-sessions"
          stale={externalSessionsReliability.isStale}
        />
      ) : null}

      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <div className="relative min-h-0 min-w-0 flex-1 overflow-hidden">
          {isDm ? chatSurface : (
            <GroupThreadContextProvider>
              {chatSurface}
              <RoomMobileThreadOverlay />
            </GroupThreadContextProvider>
          )}
        </div>
      </div>

      <RoomMobileConversationSwitcher
        activeConversationId={conversationId}
        conversations={currentRoomConversations}
        isOpen={isConversationSwitcherOpen}
        onClose={() => setIsConversationSwitcherOpen(false)}
        onSelect={onSelectConversation}
      />

      <RoomMobileSubagentOverlay
        currentAgentId={currentAgent.agent_id}
        onClose={() => setOpenSubagentSource(null)}
        onOpenWorkspaceFile={handleOpenWorkspaceFile}
        requestKey={subagentRequest.key}
        requestedHostAgentId={subagentRequest.hostAgentId}
        requestedTaskToolUseId={subagentRequest.toolUseId}
        roomMembers={roomMembers}
        source={openSubagentSource === subagentTaskSource
          ? openSubagentSource
          : null}
      />

      <RoomMobileAuxiliaryOverlay
        activeTab={activeAuxiliaryTab}
        activeWorkspacePath={activeWorkspacePath}
        composerDraftScopeKey={composerDraftScopeKey}
        conversationId={conversationId}
        currentAgent={currentAgent}
        executionResource={executionResource}
        executionTaskRuns={executionTaskRuns}
        isDm={isDm}
        onClose={() => setActiveAuxiliaryTab(null)}
        onOpenWorkspaceFile={handleOpenWorkspaceFile}
        onSaveAgentOptions={onSaveAgentOptions}
        onValidateAgentName={onValidateAgentName}
        roomId={roomId}
        roomMembers={roomMembers}
      />

      {!isDm ? (
        <RoomMemberManagerDialog
          availableRoomAgents={availableRoomAgents}
          initialAvatar={roomAvatar}
          initialHostAgentId={roomHostAgentId}
          initialHostAutoReplyEnabled={roomHostAutoReplyEnabled}
          initialName={currentRoomTitle}
          initialPrivateMessagesEnabled={roomPrivateMessagesEnabled}
          initialRoomSkillNames={roomSkillNames}
          isOpen={roomId !== null && memberDialogRoomId === roomId}
          onClose={() => setMemberDialogRoomId(null)}
          onManageRoom={onManageRoom}
          roomMembers={roomMembers}
        />
      ) : null}
    </section>
  );
}
