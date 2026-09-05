/**
 * INPUT: Group Chat props、共享 Session/Composer/Goal 资源与 Room Agent 时间线。
 * OUTPUT: 含稳定滚动、Feed、Composer、Goal 与外部权威 WorkGraph 资源的面板模型。
 * POS: Group Chat 有状态装配入口；纯投影与未读队列分别下沉到专属模块。
 */
import { useEffect, useMemo } from "react";

import { useComposerGoalSubmissionReconciliation } from "@/features/conversation/shared/composer/composer-goal-submission-reconciliation";
import { useConversationPanelEnvironment } from "@/features/conversation/shared/use-conversation-panel-environment";
import { buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import type { Agent } from "@/types/agent/agent";

import { projectGroupAgentTimeline } from "../../feed/group-agent-timeline-model";
import { useRoomThreadSource } from "../../../thread/live/use-room-thread-source";
import type { GroupChatPanelProps } from "../group-chat-panel-types";
import type { GroupChatPanelViewModel } from "../view/group-chat-panel-view";
import {
  buildGroupChatPanelViewModel,
  type RoomAgentDirectory,
} from "./group-chat-panel-projection";
import { useGroupChatComposerModel } from "./use-group-chat-composer-model";
import { useGroupChatSessionController } from "./use-group-chat-session-controller";
import { useRoomGoalComposer } from "./use-room-goal-composer";

export function useGroupChatPanelModel({
  agentId,
  conversationId,
  currentAgentAvatar,
  currentAgentName,
  executionResource,
  initialDraft,
  layout,
  onConversationSnapshotChange,
  onCreateConversation,
  onExecutionTaskRunsChange,
  onInitialDraftConsumed,
  onOpenAgentContact,
  onOpenSubagentTask,
  onOpenWorkGraph,
  onOpenWorkspaceFile,
  onRoomEvent,
  onTodosChange,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomId,
  roomMembers,
  runtimeKind,
}: GroupChatPanelProps): GroupChatPanelViewModel {
  const environment = useConversationPanelEnvironment(layout);
  const sessionKey = conversationId
    ? buildRoomSharedSessionKey(conversationId)
    : null;
  const goal = useRoomGoalComposer({
    roomId,
    roomHostAgentId,
    roomMembers,
    sessionKey,
  });
  const session = useGroupChatSessionController({
    agentId,
    conversationId,
    onConversationSnapshotChange,
    onGoalEvent: goal.refresh,
    onRoomEvent,
    onTodosChange,
    roomId,
    sessionKey,
  });
  useEffect(() => {
    onExecutionTaskRunsChange?.(session.taskRuns);
  }, [onExecutionTaskRunsChange, session.taskRuns]);
  const feedTimeline = useMemo(
    () => projectGroupAgentTimeline({
      messageGroups: session.timeline.message_groups,
      pendingPermissionGroups: session.timeline.pending_permission_groups,
      pendingSlotGroups: session.timeline.pending_slot_groups,
      roomAgentExecutionStateGroups:
        session.timeline.room_agent_execution_state_groups,
      roundIds: session.timeline.feed_round_ids,
    }),
    [session.timeline],
  );
  const directory = useRoomAgentDirectory(roomMembers);
  const composer = useGroupChatComposerModel({
    agentId,
    conversation: session.conversation,
    conversationId,
    goal,
    initialDraft: initialDraft ?? null,
    onInitialDraftConsumed,
    roomId,
    roomMembers,
    scrollToBottom: session.scroll.scrollToBottom,
    sessionKey: session.sessionKey,
    runtimeKind,
  });
  const reconcileGoalSubmission = useComposerGoalSubmissionReconciliation(
    composer.draftScopeKey,
    session.conversation.messages,
  );

  useRoomThreadSource({
    agentAvatarMap: directory.avatars,
    agentNameMap: directory.names,
    conversationId,
    messageGroups: session.timeline.message_groups,
    onOpenWorkspaceFile,
    pendingPermissionGroups: session.timeline.pending_permission_groups,
    pendingSlotGroups: session.timeline.pending_slot_groups,
    roomAgentExecutionStateGroups:
      session.timeline.room_agent_execution_state_groups,
    sendPermissionResponse: session.conversation.send_permission_response,
  });

  return buildGroupChatPanelViewModel({
    composer,
    currentAgentAvatar,
    currentAgentName,
    directory,
    environment,
    execution: executionResource,
    feedTimeline,
    goal,
    onGoalChange: reconcileGoalSubmission,
    onCreateConversation,
    onOpenAgentContact,
    onOpenSubagentTask,
    onOpenWorkGraph,
    onOpenWorkspaceFile,
    roomHostAgentId,
    roomHostAutoReplyEnabled,
    roomMembers,
    session,
  });
}

function useRoomAgentDirectory(roomMembers: Agent[]): RoomAgentDirectory {
  return useMemo(() => buildRoomAgentDirectory(roomMembers), [roomMembers]);
}

function buildRoomAgentDirectory(roomMembers: Agent[]): RoomAgentDirectory {
  const avatars: Record<string, string | null> = {};
  const names: Record<string, string> = {};
  for (const member of roomMembers) {
    avatars[member.agent_id] = member.avatar ?? null;
    names[member.agent_id] = member.name;
  }
  return { avatars, names };
}
