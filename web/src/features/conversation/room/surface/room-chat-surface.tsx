"use client";

/**
 * INPUT: 当前 Room/DM 身份、任务快照、runtime 与会话命令。
 * OUTPUT: 绑定错误边界后把空选择或任务快照投影到共享新会话入口及对应 DM/Group 聊天面板。
 * POS: Room Surface 到两类聊天面板之间的窄适配层。
 */

import { DmChatPanel } from "@/features/conversation/room/dm/panel/dm-chat-panel";
import { GroupChatPanel } from "@/features/conversation/room/group/chat/panel/group-chat-panel";
import type { ExecutionResource } from "@/features/conversation/shared/execution/use-execution-resource";
import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { getAgentConversationIdentityKey } from "@/lib/conversation/agent-conversation-identity";
import type { Agent } from "@/types/agent/agent";
import type {
  AgentConversationIdentity,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import type { ConversationSnapshotPayload } from "@/types/conversation/conversation";
import type { TodoItem } from "@/types/conversation/todo";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

import { RoomChatErrorBoundary } from "./room-chat-error-boundary";
import { RoomConversationEmptyState } from "./room-conversation-empty-state";

interface RoomChatSurfaceProps {
  currentAgent: Agent;
  currentRoomType: string;
  currentAgentSessionIdentity: AgentConversationIdentity | null;
  conversationId: string | null;
  executionResource: ExecutionResource;
  initialDraft?: string | null;
  layout: "desktop" | "mobile";
  onInitialDraftConsumed?: () => void;
  onConversationSnapshotChange: (snapshot: ConversationSnapshotPayload) => void;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onExecutionTaskRunsChange: (runs: ConversationTaskRun[]) => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
  onOpenWorkGraph?: () => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
  onTodosChange: (todos: TodoItem[]) => void;
  roomHostAgentId: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomId: string | null;
  roomMembers: Agent[];
  runtimeKind: AgentRuntimeKind;
  todos: TodoItem[];
}

export function RoomChatSurface({
  currentAgent: currentAgent,
  currentRoomType: currentRoomType,
  currentAgentSessionIdentity: currentAgentSessionIdentity,
  conversationId: conversationId,
  executionResource,
  initialDraft: initialDraft,
  layout: layout,
  onInitialDraftConsumed: onInitialDraftConsumed,
  onConversationSnapshotChange: onConversationSnapshotChange,
  onCreateConversation: onCreateConversation,
  onExecutionTaskRunsChange,
  onOpenAgentContact: onOpenAgentContact,
  onOpenSubagentTask,
  onOpenWorkGraph,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  onRoomEvent: onRoomEvent,
  onTodosChange: onTodosChange,
  roomHostAgentId: roomHostAgentId,
  roomHostAutoReplyEnabled: roomHostAutoReplyEnabled,
  roomId: roomId,
  roomMembers: roomMembers,
  runtimeKind: runtimeKind,
  todos,
}: RoomChatSurfaceProps) {
  const isDm = currentRoomType === "dm";
  const identityKey = getAgentConversationIdentityKey(currentAgentSessionIdentity)
    ?? `${roomId ?? "room"}:${conversationId ?? "conversation"}`;

  return (
    <RoomChatErrorBoundary resetKey={`${currentRoomType}:${identityKey}`}>
      {!conversationId ? (
        <RoomConversationEmptyState
          isDm={isDm}
          onCreateConversation={onCreateConversation}
        />
      ) : isDm ? (
        <DmChatPanel
          currentAgent={currentAgent}
          executionResource={executionResource}
          initialDraft={initialDraft}
          layout={layout}
          onInitialDraftConsumed={onInitialDraftConsumed}
          onConversationSnapshotChange={onConversationSnapshotChange}
          onExecutionTaskRunsChange={onExecutionTaskRunsChange}
          onOpenAgentContact={onOpenAgentContact}
          onOpenSubagentTask={onOpenSubagentTask}
          onOpenWorkGraph={onOpenWorkGraph}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          onRoomEvent={onRoomEvent}
          onTodosChange={onTodosChange}
          sessionIdentity={currentAgentSessionIdentity}
          runtimeKind={runtimeKind}
          todos={todos}
        />
      ) : (
        <GroupChatPanel
          agentId={currentAgent.agent_id}
          conversationId={conversationId}
          currentAgentName={currentAgent.name}
          currentAgentAvatar={currentAgent.avatar ?? null}
          executionResource={executionResource}
          initialDraft={initialDraft}
          layout={layout}
          onInitialDraftConsumed={onInitialDraftConsumed}
          onConversationSnapshotChange={onConversationSnapshotChange}
          onCreateConversation={onCreateConversation}
          onExecutionTaskRunsChange={onExecutionTaskRunsChange}
          onOpenAgentContact={onOpenAgentContact}
          onOpenSubagentTask={onOpenSubagentTask}
          onOpenWorkGraph={onOpenWorkGraph}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          onRoomEvent={onRoomEvent}
          onTodosChange={onTodosChange}
          roomHostAgentId={roomHostAgentId}
          roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
          roomId={roomId}
          roomMembers={roomMembers}
          runtimeKind={runtimeKind}
        />
      )}
    </RoomChatErrorBoundary>
  );
}
