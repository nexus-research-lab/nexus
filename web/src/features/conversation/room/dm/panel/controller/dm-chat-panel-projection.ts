/**
 * INPUT: DM 会话、当前 Agent、Goal、Composer 与面板环境。
 * OUTPUT: Feed、Goal、带来源的进程和输入区纯视图模型。
 * POS: DM Chat 控制器状态到纯视图 props 的唯一投影入口。
 */
import type { RefObject } from "react";

import {
  isExecutionActivityVisible,
  type ExecutionAgentDirectory,
} from "@/features/conversation/shared/execution/execution-process-model";
import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import {
  buildConversationPanelFrameModel,
  type ConversationPanelEnvironment,
  type ConversationPanelSessionSource,
} from "@/features/conversation/shared/conversation-panel-model";
import { buildGoalActivityKey } from "@/features/conversation/shared/goal/goal-model";
import { coalescePendingPermissions } from "@/lib/conversation/pending-permission-match";
import type { UseAgentConversationReturn } from "@/types/agent/agent-conversation";
import type { SessionRoundIndexItem } from "@/types/conversation/history";
import type { TodoItem } from "@/types/conversation/todo";
import type { ExecutionView } from "@/types/conversation/execution";
import type { Goal } from "@/types/conversation/goal";

import type {
  DmChatComposerModel,
  DmChatPanelViewModel,
} from "../view/dm-chat-panel-view";
import type { DmGoalControllerModel } from "./use-dm-goal-controller";

type DmChatSession = Omit<
  ConversationPanelSessionSource,
  "conversation" | "scroll"
> & {
  conversation: ConversationPanelSessionSource["conversation"] & Pick<
    UseAgentConversationReturn,
    | "live_round_ids"
    | "messages"
    | "pending_permissions"
    | "runtime_phase"
    | "send_permission_response"
  >;
  roundIndexItems: SessionRoundIndexItem[];
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
type DmGoalProjection = Pick<
  DmGoalControllerModel,
  "continuationHold" | "refreshSequence"
>;

interface BuildDmChatPanelViewModelOptions {
  composer: DmChatComposerModel;
  currentAgentAvatar: string | null;
  currentAgentName: string | null;
  environment: ConversationPanelEnvironment;
  execution: {
    dismiss: () => void;
    execution: ExecutionView | null;
  };
  goal: DmGoalProjection;
  goalScopeLabel: string;
  onEditLastUserMessage: (messageId: string, content: string) => void;
  onForkConversation?: (roundId: string) => Promise<void>;
  onGoalChange: (goal: Goal | null) => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
  onOpenWorkGraph?: () => void;
  onOpenWorkspaceFile?: (path: string) => void;
  session: DmChatSession;
  todos: TodoItem[];
  workspaceAgentId: string | null;
}

export function buildDmChatPanelViewModel({
  composer,
  currentAgentAvatar,
  currentAgentName,
  environment,
  execution,
  goal,
  goalScopeLabel,
  onEditLastUserMessage,
  onForkConversation,
  onGoalChange,
  onOpenAgentContact,
  onOpenSubagentTask,
  onOpenWorkGraph,
  onOpenWorkspaceFile,
  session,
  todos,
  workspaceAgentId,
}: BuildDmChatPanelViewModelOptions): DmChatPanelViewModel {
  return {
    ...buildConversationPanelFrameModel(session, environment),
    composer,
    composerInteraction: {
      agentAvatarMap: workspaceAgentId
        ? { [workspaceAgentId]: currentAgentAvatar }
        : undefined,
      agentNameMap: workspaceAgentId && currentAgentName
        ? { [workspaceAgentId]: currentAgentName }
        : undefined,
      fallbackAgentId: workspaceAgentId,
      onResponse: session.conversation.send_permission_response,
      permissions: coalescePendingPermissions(
        session.conversation.pending_permissions,
      ),
    },
    feed: buildDmFeedModel({
      currentAgentAvatar,
      currentAgentName,
      environment,
      onEditLastUserMessage,
      onForkConversation,
      onOpenAgentContact,
      onOpenSubagentTask,
      onOpenWorkspaceFile,
      session,
      workspaceAgentId,
    }),
    executionPanel: buildDmExecutionPanelModel({
      currentAgentAvatar,
      currentAgentName,
      execution: execution.execution,
      onNavigateToRound: (roundId: string) => {
        session.roundScrollRef.current?.scrollToRoundId(roundId, {
          align: "focus",
          behavior: "smooth",
          target: "round",
        });
      },
      onOpenGraph: onOpenWorkGraph,
      workspaceAgentId,
    }),
    goalPanel: buildDmGoalPanelModel(
      goal,
      goalScopeLabel,
      session,
      onGoalChange,
    ),
    taskSource: workspaceAgentId && currentAgentName
      ? {
          agentId: workspaceAgentId,
          avatar: currentAgentAvatar,
          name: currentAgentName,
        }
      : undefined,
    todos,
  };
}

function buildDmExecutionPanelModel({
  currentAgentAvatar,
  currentAgentName,
  execution,
  onNavigateToRound,
  onOpenGraph,
  workspaceAgentId,
}: {
  currentAgentAvatar: string | null;
  currentAgentName: string | null;
  execution: ExecutionView | null;
  onNavigateToRound: (roundId: string) => void;
  onOpenGraph?: () => void;
  workspaceAgentId: string | null;
}): DmChatPanelViewModel["executionPanel"] {
  if (!isExecutionActivityVisible(execution)) {
    return null;
  }
  const directory: ExecutionAgentDirectory = {};
  if (workspaceAgentId) {
    directory[workspaceAgentId] = {
      avatar: currentAgentAvatar,
      id: workspaceAgentId,
      name: currentAgentName || workspaceAgentId,
    };
  }
  return {
    directory,
    execution,
    onNavigateToRound,
    onOpenGraph,
  };
}

function buildDmFeedModel({
  currentAgentAvatar,
  currentAgentName,
  environment,
  onEditLastUserMessage,
  onForkConversation,
  onOpenAgentContact,
  onOpenSubagentTask,
  onOpenWorkspaceFile,
  session,
  workspaceAgentId,
}: Pick<
  BuildDmChatPanelViewModelOptions,
  | "currentAgentAvatar"
  | "currentAgentName"
  | "environment"
  | "onEditLastUserMessage"
  | "onForkConversation"
  | "onOpenAgentContact"
  | "onOpenSubagentTask"
  | "onOpenWorkspaceFile"
  | "session"
  | "workspaceAgentId"
>): DmChatPanelViewModel["feed"] {
  const { conversation, roundIndexItems, roundScrollRef, scroll, timeline } =
    session;
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
      currentAgentAvatar,
      currentAgentName,
      onEditLastUserMessage,
      onForkRound: onForkConversation,
      onOpenAgentContact,
      onOpenSubagentTask,
      onOpenWorkspaceFile,
      onPermissionResponse: conversation.send_permission_response,
      workspaceAgentId,
    },
    source: {
      liveLayoutActive: scroll.liveLayoutActive,
      liveRoundIds: conversation.live_round_ids,
      messageGroups: timeline.message_groups,
      pendingPermissions: conversation.pending_permissions,
      roundIds: timeline.feed_round_ids,
      roundIndexItems,
      runtimePhase: conversation.runtime_phase,
      scopeKey: session.sessionKey,
    },
  };
}

function buildDmGoalPanelModel(
  goal: DmGoalProjection,
  scopeLabel: string,
  session: DmChatSession,
  onGoalChange: (goal: Goal | null) => void,
): DmChatPanelViewModel["goalPanel"] {
  const { conversation, sessionKey } = session;
  return {
    activityKey: buildGoalActivityKey(
      conversation.messages.length,
      conversation.is_loading,
      goal.refreshSequence,
    ),
    continuationHold: goal.continuationHold,
    isGenerating: conversation.is_loading,
    onGoalChange,
    scopeLabel,
    sessionKey,
  };
}
