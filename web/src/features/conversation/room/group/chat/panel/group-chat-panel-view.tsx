import type { ComponentProps } from "react";
import { UserRound } from "lucide-react";

import { ComposerPanel } from "@/features/conversation/shared/composer/composer-panel";
import {
  ConversationPanelFloatingControls,
  ConversationPanelLayout,
  ConversationPanelViewport,
  type ConversationScrollToLatestModel,
  type ConversationViewportModel,
} from "@/features/conversation/shared/conversation-panel-layout";
import { ConversationSessionNavigator } from "@/features/conversation/shared/session-navigator/conversation-session-navigator";
import type { Agent } from "@/types/agent/agent";

import { GroupConversationFeed } from "../feed/group-conversation-feed";
import type { GroupConversationFeedProps } from "../feed/group-conversation-feed-model";
import { GroupConversationEmptyState } from "../group-conversation-empty-state";
import { RoomGoalPanel } from "../room-goal-panel";

type NavigatorModel = Omit<
  ComponentProps<typeof ConversationSessionNavigator>,
  "className"
>;
type ComposerModel = Omit<
  ComponentProps<typeof ComposerPanel>,
  "compact" | "goalModeExtra"
>;
type GoalPanelModel = Omit<
  ComponentProps<typeof RoomGoalPanel>,
  "isMobileLayout"
>;
export interface GroupChatPanelViewModel {
  composer: ComposerModel;
  feed: GroupConversationFeedProps;
  goalLead: {
    agentId: string;
    disabled: boolean;
    onChange: (agentId: string) => void;
    roomMembers: Agent[];
  };
  goalPanel: GoalPanelModel;
  isMobileLayout: boolean;
  navigator: NavigatorModel;
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

function RoomGoalLeadControl({
  agentId,
  disabled,
  onChange,
  roomMembers,
}: GroupChatPanelViewModel["goalLead"]) {
  return (
    <label
      className="pointer-events-auto inline-flex h-5 min-w-0 max-w-[190px] items-center gap-1 rounded-[7px] border border-(--surface-canvas-border) bg-(--surface-elevated-background) px-1.5 text-[10px] font-medium text-(--text-muted)"
      title="选择 Room Goal 负责人"
    >
      <UserRound className="h-3 w-3 shrink-0" />
      <select
        className="min-w-0 flex-1 bg-transparent text-[10px] font-semibold text-(--text-default) outline-none disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
        disabled={disabled}
        value={agentId}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">负责人</option>
        {roomMembers.map((agent) => (
          <option key={agent.agent_id} value={agent.agent_id}>
            {agent.name}
          </option>
        ))}
      </select>
    </label>
  );
}
