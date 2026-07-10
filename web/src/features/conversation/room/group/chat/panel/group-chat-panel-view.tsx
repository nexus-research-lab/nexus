import type { ComponentProps, RefObject } from "react";
import { UserRound } from "lucide-react";

import { ComposerPanel } from "@/features/conversation/shared/composer/composer-panel";
import { ConversationErrorBubble } from "@/features/conversation/shared/conversation-error-bubble";
import { ConversationSessionNavigator } from "@/features/conversation/shared/session-navigator/conversation-session-navigator";
import { ProviderUnavailableBanner } from "@/features/conversation/shared/provider-unavailable-banner";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
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
type ScrollViewportEvents = Pick<
  ComponentProps<"div">,
  "onScroll" | "onTouchEnd" | "onTouchMove" | "onTouchStart" | "onWheel"
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
  scrollToLatest: {
    isLoading: boolean;
    onClick: () => void;
    visible: boolean;
  };
  sessionKey: string | null;
  viewport: ScrollViewportEvents & {
    error: string | null;
    isHistoryLoading: boolean;
    scrollRef: RefObject<HTMLDivElement | null>;
  };
}

export function GroupChatPanelView({
  model,
}: {
  model: GroupChatPanelViewModel;
}) {
  const { isMobileLayout } = model;
  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-transparent">
      {!isMobileLayout && model.sessionKey ? (
        <ConversationSessionNavigator
          {...model.navigator}
          className="absolute bottom-[156px] left-3 top-7 z-20"
        />
      ) : null}

      {!model.sessionKey ? (
        <GroupConversationEmptyState
          onCreateConversation={model.onCreateConversation}
        />
      ) : (
        <ActiveGroupConversation model={model} />
      )}
    </div>
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
      <div
        ref={viewport.scrollRef}
        className={
          isMobileLayout
            ? "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-1 py-2"
            : "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-4 py-5 sm:px-6 sm:py-6 xl:px-8 xl:py-7"
        }
        style={{ overflowAnchor: "none" }}
        onScroll={viewport.onScroll}
        onTouchEnd={viewport.onTouchEnd}
        onTouchMove={viewport.onTouchMove}
        onTouchStart={viewport.onTouchStart}
        onWheel={viewport.onWheel}
      >
        {viewport.isHistoryLoading ? (
          <div className="mx-auto mb-3 flex w-full max-w-[980px] items-center justify-center text-xs text-muted-foreground">
            正在加载更早消息...
          </div>
        ) : null}
        <GroupConversationFeed {...model.feed} />
        {viewport.error ? (
          <div
            className={
              isMobileLayout
                ? "mt-4"
                : "mx-auto mt-2 w-full max-w-[980px]"
            }
          >
            <ConversationErrorBubble
              compact={isMobileLayout}
              error={viewport.error}
            />
          </div>
        ) : null}
      </div>

      {model.scrollToLatest.visible ? (
        <ScrollToLatestButton
          isLoading={model.scrollToLatest.isLoading}
          isMobileLayout={isMobileLayout}
          onClick={model.scrollToLatest.onClick}
        />
      ) : null}
      {model.providerWarningVisible ? (
        <ProviderUnavailableBanner compact={isMobileLayout} />
      ) : null}
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
