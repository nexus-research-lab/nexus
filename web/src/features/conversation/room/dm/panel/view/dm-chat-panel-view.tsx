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
