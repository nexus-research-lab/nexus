import type { RefObject } from "react";

import {
  buildConversationPanelFrameModel,
  type ConversationPanelEnvironment,
  type ConversationPanelSessionSource,
} from "@/features/conversation/shared/conversation-panel-model";
import { buildGoalActivityKey } from "@/features/conversation/shared/goal/goal-model";
import { buildHomeOnboardingRoundId } from "@/features/onboarding/home-agent-onboarding";
import type {
  HomeOnboardingAgentStyleChoice,
  HomeOnboardingAgentTaskDraft,
} from "@/features/onboarding/home-onboarding-agent-task";
import type {
  HomeOnboardingProviderChoice,
} from "@/features/onboarding/home-onboarding-provider";
import type {
  HomeOnboardingRoomTaskDraft,
} from "@/features/onboarding/home-onboarding-room-task";
import type { UseAgentConversationReturn } from "@/types/agent/agent-conversation";
import type { Message } from "@/types/conversation/message/entity";
import type { SessionRoundIndexItem } from "@/types/conversation/history";
import type { ProviderModelSelection } from "@/types/capability/provider";

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
  scroll: ConversationPanelSessionSource["scroll"] & {
    bottomAnchorRef: RefObject<HTMLDivElement | null>;
    feedRef: RefObject<HTMLDivElement | null>;
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
  goal: DmGoalProjection;
  goalScopeLabel: string;
  onboarding: boolean;
  onboardingAgentConfirmationCardVisible: boolean;
  onboardingAgentIdentityCardVisible: boolean;
  onboardingAgentIsCreating: boolean;
  onboardingAgentStyleCardVisible: boolean;
  onboardingAgentStyleChoices: HomeOnboardingAgentStyleChoice[];
  onboardingAgentTaskDraft: HomeOnboardingAgentTaskDraft;
  onboardingMessages: Message[];
  onboardingDefaultModelCardVisible: boolean;
  onboardingDefaultModelChoices: ProviderModelSelection[];
  onboardingDefaultModelSelection: ProviderModelSelection | null;
  onboardingProviderChoices: HomeOnboardingProviderChoice[];
  onboardingProviderChoicesError: string | null;
  onboardingProviderChoicesLoading: boolean;
  onboardingProviderConfigCardVisible: boolean;
  onboardingProviderSelectionCardVisible: boolean;
  onboardingRoleCardVisible: boolean;
  onboardingRoomCompletionCardVisible: boolean;
  onboardingRoomIsCreating: boolean;
  onboardingRoomLaunchCardVisible: boolean;
  onboardingRoomPlanCardVisible: boolean;
  onboardingRoomStartCardVisible: boolean;
  onboardingRoomTaskDraft: HomeOnboardingRoomTaskDraft;
  onConfirmDefaultModel: () => Promise<void>;
  onConfirmAgentCreation: () => Promise<void>;
  onConfirmRoomPlan: () => Promise<void>;
  onEditLastUserMessage: (messageId: string, content: string) => void;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onFinishOnboarding: () => void;
  onLaunchRoomCollaboration: () => void;
  onRetryProviderChoices: () => void;
  onRestartAgentDraft: () => void;
  onRestartRoomIdea: () => void;
  onSelectAgentStyle: (choice: HomeOnboardingAgentStyleChoice) => void;
  onSelectDefaultModel: (selection: ProviderModelSelection) => void;
  onSelectProvider: (choice: HomeOnboardingProviderChoice) => void;
  onSelectRole: (role: string) => void;
  onStartRoomTask: () => void;
  session: DmChatSession;
  workspaceAgentId: string | null;
}

export function buildDmChatPanelViewModel({
  composer,
  currentAgentAvatar,
  currentAgentName,
  environment,
  goal,
  goalScopeLabel,
  onboarding,
  onboardingAgentConfirmationCardVisible,
  onboardingAgentIdentityCardVisible,
  onboardingAgentIsCreating,
  onboardingAgentStyleCardVisible,
  onboardingAgentStyleChoices,
  onboardingAgentTaskDraft,
  onEditLastUserMessage,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onboardingMessages,
  onboardingDefaultModelCardVisible,
  onboardingDefaultModelChoices,
  onboardingDefaultModelSelection,
  onboardingProviderChoices,
  onboardingProviderChoicesError,
  onboardingProviderChoicesLoading,
  onboardingProviderConfigCardVisible,
  onboardingProviderSelectionCardVisible,
  onboardingRoleCardVisible,
  onboardingRoomCompletionCardVisible,
  onboardingRoomIsCreating,
  onboardingRoomLaunchCardVisible,
  onboardingRoomPlanCardVisible,
  onboardingRoomStartCardVisible,
  onboardingRoomTaskDraft,
  onConfirmDefaultModel,
  onConfirmAgentCreation,
  onConfirmRoomPlan,
  onFinishOnboarding,
  onLaunchRoomCollaboration,
  onRetryProviderChoices,
  onRestartAgentDraft,
  onRestartRoomIdea,
  onSelectAgentStyle,
  onSelectDefaultModel,
  onSelectProvider,
  session,
  workspaceAgentId,
  onSelectRole,
  onStartRoomTask,
}: BuildDmChatPanelViewModelOptions): DmChatPanelViewModel {
  const frameEnvironment = onboarding
    ? { ...environment, providerWarningVisible: false }
    : environment;
  return {
    ...buildConversationPanelFrameModel(session, frameEnvironment),
    composer,
    feed: buildDmFeedModel({
      currentAgentAvatar,
      currentAgentName,
      environment,
      onboarding,
      onboardingAgentConfirmationCardVisible,
      onboardingAgentIdentityCardVisible,
      onboardingAgentStyleCardVisible,
      onboardingDefaultModelCardVisible,
      onboardingProviderConfigCardVisible,
      onboardingProviderSelectionCardVisible,
      onboardingRoleCardVisible,
      onboardingRoomCompletionCardVisible,
      onboardingRoomLaunchCardVisible,
      onboardingRoomPlanCardVisible,
      onboardingRoomStartCardVisible,
      onEditLastUserMessage,
      onOpenAgentContact,
      onOpenWorkspaceFile,
      onboardingMessages,
      session,
      workspaceAgentId,
    }),
    goalPanel: buildDmGoalPanelModel(goal, goalScopeLabel, session),
    onboardingAgentConfirmationCard:
      onboardingAgentConfirmationCardVisible
        ? {
            draft: onboardingAgentTaskDraft,
            isCreating: onboardingAgentIsCreating,
            onConfirm: onConfirmAgentCreation,
            onRestart: onRestartAgentDraft,
          }
        : null,
    onboardingAgentIdentityCard:
      onboardingAgentIdentityCardVisible
        ? { draft: onboardingAgentTaskDraft }
        : null,
    onboardingAgentStyleCard:
      onboardingAgentStyleCardVisible
        ? {
            choices: onboardingAgentStyleChoices,
            onSelect: onSelectAgentStyle,
            role: onboardingAgentTaskDraft.role,
          }
        : null,
    onboardingDefaultModelCard:
      onboardingDefaultModelCardVisible && onboardingDefaultModelSelection
        ? {
            choices: onboardingDefaultModelChoices,
            onConfirm: onConfirmDefaultModel,
            onSelectionChange: onSelectDefaultModel,
            selection: onboardingDefaultModelSelection,
          }
        : null,
    onboardingProviderConfigCardVisible,
    onboardingProviderSelectionCard:
      onboardingProviderSelectionCardVisible
        ? {
            choices: onboardingProviderChoices,
            error: onboardingProviderChoicesError,
            loading: onboardingProviderChoicesLoading,
            onRetry: onRetryProviderChoices,
            onSelect: onSelectProvider,
          }
        : null,
    onboardingRoleCard: onboardingRoleCardVisible
      ? { onSelect: onSelectRole }
      : null,
    onboardingRoomCompletionCard:
      onboardingRoomCompletionCardVisible
        ? {
            draft: onboardingRoomTaskDraft,
            onFinish: onFinishOnboarding,
          }
        : null,
    onboardingRoomLaunchCard:
      onboardingRoomLaunchCardVisible
        ? {
            draft: onboardingRoomTaskDraft,
            onLaunch: onLaunchRoomCollaboration,
            resume: onboardingRoomTaskDraft.phase !== "ready",
          }
        : null,
    onboardingRoomPlanCard:
      onboardingRoomPlanCardVisible
        ? {
            draft: onboardingRoomTaskDraft,
            isCreating: onboardingRoomIsCreating,
            onConfirm: onConfirmRoomPlan,
            onRestart: onRestartRoomIdea,
          }
        : null,
    onboardingRoomStartCard:
      onboardingRoomStartCardVisible
        ? {
            agentDraft: onboardingAgentTaskDraft,
            onStart: onStartRoomTask,
          }
        : null,
  };
}

function buildDmFeedModel({
  currentAgentAvatar,
  currentAgentName,
  environment,
  onboarding,
  onboardingAgentConfirmationCardVisible,
  onboardingAgentIdentityCardVisible,
  onboardingAgentStyleCardVisible,
  onboardingDefaultModelCardVisible,
  onboardingProviderConfigCardVisible,
  onboardingProviderSelectionCardVisible,
  onboardingRoleCardVisible,
  onboardingRoomCompletionCardVisible,
  onboardingRoomLaunchCardVisible,
  onboardingRoomPlanCardVisible,
  onboardingRoomStartCardVisible,
  onEditLastUserMessage,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onboardingMessages,
  session,
  workspaceAgentId,
}: Pick<
  BuildDmChatPanelViewModelOptions,
  | "currentAgentAvatar"
  | "currentAgentName"
  | "environment"
  | "onEditLastUserMessage"
  | "onOpenAgentContact"
  | "onOpenWorkspaceFile"
  | "onboarding"
  | "onboardingAgentConfirmationCardVisible"
  | "onboardingAgentIdentityCardVisible"
  | "onboardingAgentStyleCardVisible"
  | "onboardingDefaultModelCardVisible"
  | "onboardingMessages"
  | "onboardingProviderConfigCardVisible"
  | "onboardingProviderSelectionCardVisible"
  | "onboardingRoleCardVisible"
  | "onboardingRoomCompletionCardVisible"
  | "onboardingRoomLaunchCardVisible"
  | "onboardingRoomPlanCardVisible"
  | "onboardingRoomStartCardVisible"
  | "session"
  | "workspaceAgentId"
>): DmChatPanelViewModel["feed"] {
  const { conversation, roundIndexItems, roundScrollRef, scroll, timeline } =
    session;
  const messageGroups = new Map(timeline.message_groups);
  const onboardingRoundIds: string[] = [];
  onboardingMessages.forEach((message, index) => {
    const roundId = buildHomeOnboardingRoundId(index);
    messageGroups.set(roundId, [message]);
    onboardingRoundIds.push(roundId);
  });
  const roundIds = mergeOnboardingRoundIdsByTimestamp(
    timeline.feed_round_ids,
    onboardingRoundIds,
    messageGroups,
  );
  return {
    isMobileLayout: environment.isMobileLayout,
    refs: {
      bottomAnchorRef: scroll.bottomAnchorRef,
      feedRef: scroll.feedRef,
      roundScrollRef,
      scrollRef: scroll.scrollRef,
    },
    renderer: {
      currentAgentAvatar,
      currentAgentName,
      currentUserAvatar: environment.currentUserAvatar,
      onboarding,
      onboardingCardVisible:
        onboardingAgentConfirmationCardVisible
        || onboardingAgentIdentityCardVisible
        || onboardingAgentStyleCardVisible
        || onboardingDefaultModelCardVisible
        || onboardingProviderConfigCardVisible
        || onboardingProviderSelectionCardVisible
        || onboardingRoleCardVisible
        || onboardingRoomCompletionCardVisible
        || onboardingRoomLaunchCardVisible
        || onboardingRoomPlanCardVisible
        || onboardingRoomStartCardVisible,
      onEditLastUserMessage,
      onOpenAgentContact,
      onOpenWorkspaceFile,
      onPermissionResponse: conversation.send_permission_response,
      workspaceAgentId,
    },
    source: {
      liveRoundIds: conversation.live_round_ids,
      messageGroups,
      pendingPermissions: conversation.pending_permissions,
      roundIds,
      roundIndexItems,
      runtimePhase: conversation.runtime_phase,
    },
  };
}

export function mergeOnboardingRoundIdsByTimestamp(
  feedRoundIds: string[],
  onboardingRoundIds: string[],
  messageGroups: Map<string, Message[]>,
): string[] {
  const merged = [...feedRoundIds];
  onboardingRoundIds.forEach((roundId) => {
    const timestamp = messageGroups.get(roundId)?.[0]?.timestamp;
    if (typeof timestamp !== "number") {
      merged.push(roundId);
      return;
    }
    const nextRoundIndex = merged.findIndex((candidateRoundId) => {
      const candidateTimestamp = messageGroups.get(candidateRoundId)?.[0]?.timestamp;
      return typeof candidateTimestamp === "number"
        && candidateTimestamp > timestamp;
    });
    if (nextRoundIndex < 0) {
      merged.push(roundId);
      return;
    }
    merged.splice(nextRoundIndex, 0, roundId);
  });
  return merged;
}

function buildDmGoalPanelModel(
  goal: DmGoalProjection,
  scopeLabel: string,
  session: DmChatSession,
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
    scopeLabel,
    sessionKey,
  };
}
