import type { ComponentProps } from "react";

import { ComposerPanel } from "@/features/conversation/shared/composer/composer-panel";
import {
  ConversationPanelFloatingControls,
  ConversationPanelLayout,
  ConversationPanelViewport,
} from "@/features/conversation/shared/conversation-panel-layout";
import type { ConversationPanelFrameModel } from "@/features/conversation/shared/conversation-panel-model";
import { ConversationFeed } from "@/features/conversation/shared/feed/conversation-feed";
import { GoalPanel } from "@/features/conversation/shared/goal/goal-panel";
import { ConversationSessionNavigator } from "@/features/conversation/shared/session-navigator/conversation-session-navigator";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import { HomeOnboardingAgentConfirmationCard } from "@/features/onboarding/home-onboarding-agent-confirmation-card";
import { HomeOnboardingAgentIdentityCard } from "@/features/onboarding/home-onboarding-agent-identity-card";
import { HomeOnboardingAgentStyleCard } from "@/features/onboarding/home-onboarding-agent-style-card";
import { HomeOnboardingDefaultModelCard } from "@/features/onboarding/home-onboarding-default-model-card";
import { HomeOnboardingProviderConfigCard } from "@/features/onboarding/home-onboarding-provider-config-card";
import { HomeOnboardingProviderSelectionCard } from "@/features/onboarding/home-onboarding-provider-selection-card";
import { HomeOnboardingRoleCard } from "@/features/onboarding/home-onboarding-role-card";
import { HomeOnboardingRoomCompletionCard } from "@/features/onboarding/home-onboarding-room-completion-card";
import { HomeOnboardingRoomLaunchCard } from "@/features/onboarding/home-onboarding-room-launch-card";
import { HomeOnboardingRoomPlanCard } from "@/features/onboarding/home-onboarding-room-plan-card";
import { HomeOnboardingRoomStartCard } from "@/features/onboarding/home-onboarding-room-start-card";

export type DmChatComposerModel = Omit<
  ComponentProps<typeof ComposerPanel>,
  "compact"
>;
type FeedModel = ComponentProps<typeof ConversationFeed>;
type GoalPanelModel = Omit<ComponentProps<typeof GoalPanel>, "compact">;

export interface DmChatPanelViewModel extends ConversationPanelFrameModel {
  composer: DmChatComposerModel;
  feed: FeedModel;
  goalPanel: GoalPanelModel;
  onboardingAgentConfirmationCard:
    | ComponentProps<typeof HomeOnboardingAgentConfirmationCard>
    | null;
  onboardingAgentIdentityCard:
    | ComponentProps<typeof HomeOnboardingAgentIdentityCard>
    | null;
  onboardingAgentStyleCard:
    | ComponentProps<typeof HomeOnboardingAgentStyleCard>
    | null;
  onboardingDefaultModelCard:
    | ComponentProps<typeof HomeOnboardingDefaultModelCard>
    | null;
  onboardingProviderConfigCardVisible: boolean;
  onboardingProviderSelectionCard:
    | ComponentProps<typeof HomeOnboardingProviderSelectionCard>
    | null;
  onboardingRoleCard:
    | ComponentProps<typeof HomeOnboardingRoleCard>
    | null;
  onboardingRoomCompletionCard:
    | ComponentProps<typeof HomeOnboardingRoomCompletionCard>
    | null;
  onboardingRoomLaunchCard:
    | ComponentProps<typeof HomeOnboardingRoomLaunchCard>
    | null;
  onboardingRoomPlanCard:
    | ComponentProps<typeof HomeOnboardingRoomPlanCard>
    | null;
  onboardingRoomStartCard:
    | ComponentProps<typeof HomeOnboardingRoomStartCard>
    | null;
}

export function DmChatPanelView({
  model,
}: {
  model: DmChatPanelViewModel;
}) {
  const { isMobileLayout, viewport } = model;
  return (
    <ConversationPanelLayout
      navigator={!isMobileLayout && model.sessionKey ? (
        <ConversationSessionNavigator
          {...model.navigator}
          className="absolute bottom-[156px] left-3 top-7 z-20"
        />
      ) : undefined}
    >
      <ConversationPanelViewport
        isMobileLayout={isMobileLayout}
        tourAnchor={CONVERSATION_TOUR_ANCHORS.feed}
        viewport={viewport}
      >
        <ConversationFeed {...model.feed} />
        {model.onboardingAgentStyleCard ? (
          <HomeOnboardingAgentStyleCard
            {...model.onboardingAgentStyleCard}
          />
        ) : null}
        {model.onboardingAgentConfirmationCard ? (
          <HomeOnboardingAgentConfirmationCard
            {...model.onboardingAgentConfirmationCard}
          />
        ) : null}
        {model.onboardingAgentIdentityCard ? (
          <HomeOnboardingAgentIdentityCard
            {...model.onboardingAgentIdentityCard}
          />
        ) : null}
        {model.onboardingRoomStartCard ? (
          <HomeOnboardingRoomStartCard
            {...model.onboardingRoomStartCard}
          />
        ) : null}
        {model.onboardingRoomPlanCard ? (
          <HomeOnboardingRoomPlanCard
            {...model.onboardingRoomPlanCard}
          />
        ) : null}
        {model.onboardingRoomLaunchCard ? (
          <HomeOnboardingRoomLaunchCard
            {...model.onboardingRoomLaunchCard}
          />
        ) : null}
        {model.onboardingRoomCompletionCard ? (
          <HomeOnboardingRoomCompletionCard
            {...model.onboardingRoomCompletionCard}
          />
        ) : null}
        {model.onboardingDefaultModelCard ? (
          <HomeOnboardingDefaultModelCard
            {...model.onboardingDefaultModelCard}
          />
        ) : null}
        {model.onboardingProviderConfigCardVisible ? (
          <HomeOnboardingProviderConfigCard />
        ) : null}
        {model.onboardingProviderSelectionCard ? (
          <HomeOnboardingProviderSelectionCard
            {...model.onboardingProviderSelectionCard}
          />
        ) : null}
        {model.onboardingRoleCard ? (
          <HomeOnboardingRoleCard {...model.onboardingRoleCard} />
        ) : null}
      </ConversationPanelViewport>
      <ConversationPanelFloatingControls
        isMobileLayout={isMobileLayout}
        providerWarningVisible={model.providerWarningVisible}
        scrollToLatest={model.scrollToLatest}
      />
      <GoalPanel {...model.goalPanel} compact={isMobileLayout} />
      <ComposerPanel {...model.composer} compact={isMobileLayout} />
    </ConversationPanelLayout>
  );
}
