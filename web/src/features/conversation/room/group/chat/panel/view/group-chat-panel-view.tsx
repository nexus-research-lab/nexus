import type { ComponentProps } from "react";

import { ComposerPanel } from "@/features/conversation/shared/composer/composer-panel";
import {
  ConversationPanelFloatingControls,
  ConversationPanelLayout,
  ConversationPanelViewport,
  type ConversationScrollToLatestModel,
  type ConversationViewportModel,
} from "@/features/conversation/shared/conversation-panel-layout";
import type { ConversationNavigatorModel } from "@/features/conversation/shared/conversation-panel-model";
import { ConversationSessionNavigator } from "@/features/conversation/shared/session-navigator/conversation-session-navigator";

import { GroupConversationFeed } from "../../feed/group-conversation-feed";
import type { GroupConversationFeedProps } from "../../feed/group-conversation-feed-model";
import { GroupConversationEmptyState } from "../../group-conversation-empty-state";
import { RoomGoalPanel } from "../../room-goal-panel";
import {
  RoomGoalLeadControl,
  type RoomGoalLeadControlProps,
} from "./room-goal-lead-control";

export type GroupChatComposerModel = Omit<
  ComponentProps<typeof ComposerPanel>,
  "compact" | "goalModeExtra"
>;
type GoalPanelModel = Omit<
  ComponentProps<typeof RoomGoalPanel>,
  "isMobileLayout"
>;

export interface GroupChatPanelViewModel {
  composer: GroupChatComposerModel;
  feed: GroupConversationFeedProps;
  goalLead: RoomGoalLeadControlProps;
  goalPanel: GoalPanelModel;
  isMobileLayout: boolean;
  navigator: ConversationNavigatorModel;
  onCreateConversation: (title?: string) => void | Promise<string | null>;
  providerWarningVisible: boolean;
  scrollToLatest: ConversationScrollToLatestModel;
  sessionKey: string | null;
  viewport: ConversationViewportModel;
}

export function GroupChatPanelView({
  model,
}: {
  model: GroupChatPanelViewModel;
}) {
  const { isMobileLayout } = model;
  return (
    <ConversationPanelLayout
      navigator={!isMobileLayout && model.sessionKey ? (
        <ConversationSessionNavigator
          {...model.navigator}
          className="absolute bottom-[156px] left-3 top-7 z-20"
        />
      ) : undefined}
    >
      {!model.sessionKey ? (
        <GroupConversationEmptyState
          onCreateConversation={model.onCreateConversation}
        />
      ) : (
        <ActiveGroupConversation model={model} />
      )}
    </ConversationPanelLayout>
  );
}

function ActiveGroupConversation({
  model,
}: {
  model: GroupChatPanelViewModel;
}) {
  const { isMobileLayout, viewport } = model;
  return (
    <>
      <ConversationPanelViewport
        isMobileLayout={isMobileLayout}
        viewport={viewport}
      >
        <GroupConversationFeed {...model.feed} />
      </ConversationPanelViewport>
      <ConversationPanelFloatingControls
        isMobileLayout={isMobileLayout}
        providerWarningVisible={model.providerWarningVisible}
        scrollToLatest={model.scrollToLatest}
      />
      <RoomGoalPanel {...model.goalPanel} isMobileLayout={isMobileLayout} />
      <ComposerPanel
        {...model.composer}
        compact={isMobileLayout}
        goalModeExtra={<RoomGoalLeadControl {...model.goalLead} />}
      />
    </>
  );
}
