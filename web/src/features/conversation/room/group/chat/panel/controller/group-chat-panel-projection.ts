/**
 * INPUT: Group Chat 会话、Room 目录、Goal、Composer 与面板环境。
 * OUTPUT: Feed、交接 mention、Goal、统一回到底部动作和输入区的纯视图模型。
 * POS: Group Chat 控制器状态到纯视图 props 的唯一投影入口。
 */
import type { RefObject } from "react";

import {
  buildExecutionAgentDirectory,
  isExecutionActivityVisible,
} from "@/features/conversation/shared/execution/execution-process-model";
import {
  buildConversationPanelFrameModel,
  type ConversationPanelEnvironment,
  type ConversationPanelSessionSource,
} from "@/features/conversation/shared/conversation-panel-model";
import type {
  ConversationTaskRun,
  ConversationTodoProcess,
} from "@/features/conversation/shared/todos/todo-projection-model";
import { buildGoalActivityKey } from "@/features/conversation/shared/goal/goal-model";
import { coalescePendingPermissions } from "@/lib/conversation/pending-permission-match";
import type { Agent } from "@/types/agent/agent";
import type {
  InputQueueItem,
  UseAgentConversationReturn,
} from "@/types/agent/agent-conversation";
import type { SessionRoundIndexItem } from "@/types/conversation/history";
import type { ExecutionView } from "@/types/conversation/execution";
import type { Goal } from "@/types/conversation/goal";

import type {
  GroupChatComposerModel,
  GroupChatPanelViewModel,
} from "../view/group-chat-panel-view";
import type {
  GroupAgentTimelineProjection,
} from "../../feed/group-agent-timeline-model";
import type { RoomGoalComposerModel } from "./use-room-goal-composer";
import { projectRoomAgentHandoffStatuses } from "./room-handoff-status-model";

export interface RoomAgentDirectory {
  avatars: Record<string, string | null>;
  names: Record<string, string>;
}

export function projectRoomPendingInputQueueItems(
  items: InputQueueItem[],
): InputQueueItem[] {
  return items.filter((item) => item.source === "user");
}

type GroupChatSession = Omit<
  ConversationPanelSessionSource,
  "conversation" | "scroll"
> & {
  conversation: ConversationPanelSessionSource["conversation"] & Pick<
    UseAgentConversationReturn,
    | "live_round_ids"
    | "messages"
    | "input_queue_items"
    | "pending_agent_slots"
    | "pending_permissions"
    | "room_agent_execution_states"
    | "runtime_phase"
    | "send_permission_response"
    | "stop_generation"
    | "stopping_agent_round_ids"
  >;
  roundIndexItems: SessionRoundIndexItem[];
  taskProcesses: ConversationTodoProcess[];
  taskRuns: ConversationTaskRun[];
  scroll: ConversationPanelSessionSource["scroll"] & {
    bottomAnchorRef: RefObject<HTMLDivElement | null>;
    feedRef: RefObject<HTMLDivElement | null>;
    isBottomScrollActive: () => boolean;
    isFollowingLatest: () => boolean;
    isUserScrollActive: () => boolean;
    liveLayoutActive: boolean;
  };
};

interface BuildGroupChatPanelViewModelOptions {
  composer: GroupChatComposerModel;
  currentAgentAvatar: string | null;
  currentAgentName: string | null;
  directory: RoomAgentDirectory;
  environment: ConversationPanelEnvironment;
  execution: {
    dismiss: () => void;
    execution: ExecutionView | null;
  };
  feedTimeline: GroupAgentTimelineProjection;
  goal: RoomGoalComposerModel;
  onCreateConversation: (
    title?: string,
  ) => void | Promise<string | null>;
  onGoalChange: (goal: Goal | null) => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
  onOpenWorkGraph?: () => void;
  onOpenWorkspaceFile?: (path: string) => void;
  roomHostAgentId: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomMembers: Agent[];
  session: GroupChatSession;
}

export function buildGroupChatPanelViewModel({
  composer,
  currentAgentAvatar,
  currentAgentName,
  directory,
  environment,
  execution,
  feedTimeline,
  goal,
  onCreateConversation,
  onGoalChange,
  onOpenAgentContact,
  onOpenSubagentTask,
  onOpenWorkGraph,
  onOpenWorkspaceFile,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomMembers,
  session,
}: BuildGroupChatPanelViewModelOptions): GroupChatPanelViewModel {
  const frame = buildConversationPanelFrameModel(session, environment);
  return {
    ...frame,
    composer,
    composerInteraction: {
      agentAvatarMap: directory.avatars,
      agentNameMap: directory.names,
      onResponse: session.conversation.send_permission_response,
      permissions: coalescePendingPermissions(
        session.conversation.pending_permissions,
      ),
    },
    handoffStatuses: projectRoomAgentHandoffStatuses({
      executionStates: session.conversation.room_agent_execution_states,
      inputQueueItems: session.conversation.input_queue_items,
      messages: session.conversation.messages,
      pendingSlots: session.conversation.pending_agent_slots,
    }),
    feed: buildFeedModel({
      currentAgentAvatar,
      currentAgentName,
      directory,
      environment,
      feedTimeline,
      onOpenAgentContact,
      onOpenSubagentTask,
      onOpenWorkspaceFile,
      session,
    }),
    executionPanel: isExecutionActivityVisible(execution.execution)
      ? {
          directory: buildExecutionAgentDirectory(roomMembers),
          execution: execution.execution,
          onNavigateToRound: (roundId: string) => {
            session.roundScrollRef.current?.scrollToRoundId(roundId, {
              align: "focus",
              behavior: "smooth",
              target: "round",
            });
          },
          onOpenGraph: onOpenWorkGraph,
        }
      : null,
    goalLead: buildGoalLeadModel({ goal, roomMembers, session }),
    goalPanel: buildGoalPanelModel({
      goal,
      onGoalChange,
      roomHostAgentId,
      roomHostAutoReplyEnabled,
      roomMembers,
      session,
    }),
    onCreateConversation,
    scrollToLatest: frame.scrollToLatest,
    taskProcesses: session.taskProcesses,
    taskProcessMembers: roomMembers,
  };
}

function buildFeedModel({
  currentAgentAvatar,
  currentAgentName,
  directory,
  environment,
  feedTimeline,
  onOpenAgentContact,
  onOpenSubagentTask,
  onOpenWorkspaceFile,
  session,
}: Pick<
  BuildGroupChatPanelViewModelOptions,
  | "currentAgentAvatar"
  | "currentAgentName"
  | "directory"
  | "environment"
  | "feedTimeline"
  | "onOpenAgentContact"
  | "onOpenSubagentTask"
  | "onOpenWorkspaceFile"
  | "session"
>): GroupChatPanelViewModel["feed"] {
  const { conversation, roundIndexItems, roundScrollRef, scroll } = session;
  return {
    isMobileLayout: environment.isMobileLayout,
    refs: {
      bottomAnchorRef: scroll.bottomAnchorRef,
      feedRef: scroll.feedRef,
      isBottomScrollActive: scroll.isBottomScrollActive,
      isFollowingLatest: scroll.isFollowingLatest,
      isUserScrollActive: scroll.isUserScrollActive,
      roundScrollRef,
      scrollRef: scroll.scrollRef,
    },
    renderer: {
      agentAvatarMap: directory.avatars,
      agentNameMap: directory.names,
      currentAgentAvatar,
      currentAgentName,
      isLastRoundPendingPermissions: conversation.pending_permissions,
      onOpenAgentContact,
      onOpenSubagentTask,
      onOpenWorkspaceFile,
      onPermissionResponse: conversation.send_permission_response,
      onStopAgentRound: conversation.stop_generation,
      runtimePhase: conversation.runtime_phase,
      stoppingAgentRoundIds: conversation.stopping_agent_round_ids,
    },
    source: {
      liveLayoutActive: scroll.liveLayoutActive,
      liveRoundIds: conversation.live_round_ids,
      messageGroups: feedTimeline.messageGroups,
      pendingPermissionGroups: feedTimeline.pendingPermissionGroups,
      pendingSlotGroups: feedTimeline.pendingSlotGroups,
      roomAgentExecutionStateGroups:
        feedTimeline.roomAgentExecutionStateGroups,
      rootRoundIds: feedTimeline.rootRoundIds,
      roundIds: feedTimeline.roundIds,
      roundIndexItems,
      scopeKey: session.sessionKey,
    },
  };
}

function buildGoalLeadModel({
  goal,
  roomMembers,
  session,
}: Pick<
  BuildGroupChatPanelViewModelOptions,
  "goal" | "roomMembers" | "session"
>): GroupChatPanelViewModel["goalLead"] {
  return {
    agentId: goal.leadAgentId,
    disabled: session.conversation.is_loading || roomMembers.length === 0,
    onChange: goal.setLeadAgentId,
    roomMembers,
  };
}

function buildGoalPanelModel({
  goal,
  onGoalChange,
  roomHostAgentId,
  roomHostAutoReplyEnabled,
  roomMembers,
  session,
}: Pick<
  BuildGroupChatPanelViewModelOptions,
  | "goal"
  | "onGoalChange"
  | "roomHostAgentId"
  | "roomHostAutoReplyEnabled"
  | "roomMembers"
  | "session"
>): GroupChatPanelViewModel["goalPanel"] {
  const { conversation, sessionKey } = session;
  return {
    activityKey: buildGoalActivityKey(
      conversation.messages.length,
      conversation.is_loading,
      goal.refreshSequence,
    ),
    isLoading: conversation.is_loading,
    onGoalChange,
    roomHostAgentId,
    roomHostAutoReplyEnabled,
    roomMembers,
    sessionKey,
  };
}
