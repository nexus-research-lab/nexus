/**
 * INPUT: Room 会话 Frame、Feed、Goal、Task 与 Composer 视图模型。
 * OUTPUT: 共享 viewport 与从 Composer 向上堆叠 Goal、Task、滚动入口的 Room 对话布局。
 * POS: Group Chat 面板的纯视图层。
 */

import type { ComponentProps } from "react";

import type { ConversationTodoProcess } from "@/features/conversation/shared/todos/todo-projection-model";
import { ExecutionProcessPanel } from "@/features/conversation/shared/execution/execution-process-panel";
import {
  ComposerInteractionSurface,
  type ComposerInteractionSurfaceProps,
} from "@/features/conversation/shared/composer/components/interaction/composer-interaction-surface";
import { ComposerPanel } from "@/features/conversation/shared/composer/composer-panel";
import {
  AgentHandoffStatusProvider,
  type AgentHandoffStatusMap,
} from "@/features/conversation/shared/message/agent-handoff-status-context";
import {
  ConversationPanelBottomArea,
  ConversationPanelLayout,
  ConversationPanelViewport,
  ConversationPanelViewportArea,
} from "@/features/conversation/shared/conversation-panel-layout";
import type { ConversationPanelFrameModel } from "@/features/conversation/shared/conversation-panel-model";
import { ConversationSessionNavigator } from "@/features/conversation/shared/session-navigator/conversation-session-navigator";
import type { Agent } from "@/types/agent/agent";

import { GroupConversationFeed } from "../../feed/group-conversation-feed";
import type { GroupConversationFeedProps } from "../../feed/group-conversation-feed-model";
import { GroupConversationEmptyState } from "../../group-conversation-empty-state";
import { RoomGoalPanel } from "../../room-goal-panel";
import {
  RoomGoalLeadControl,
  type RoomGoalLeadControlProps,
} from "./room-goal-lead-control";
import { RoomWorkspaceTaskPanel } from "./room-workspace-task-panel";

export type GroupChatComposerModel = Omit<
  ComponentProps<typeof ComposerPanel>,
  "compact" | "goalModeExtra"
>;
type GoalPanelModel = Omit<
  ComponentProps<typeof RoomGoalPanel>,
  "isMobileLayout"
>;

export interface GroupChatPanelViewModel extends ConversationPanelFrameModel {
  composer: GroupChatComposerModel;
  composerInteraction: ComposerInteractionSurfaceProps;
  feed: GroupConversationFeedProps;
  executionPanel: ComponentProps<typeof ExecutionProcessPanel> | null;
  goalLead: RoomGoalLeadControlProps;
  goalPanel: GoalPanelModel;
  handoffStatuses: AgentHandoffStatusMap;
  onCreateConversation: (title?: string) => void | Promise<string | null>;
  taskProcesses: ConversationTodoProcess[];
  taskProcessMembers: Agent[];
}

export function GroupChatPanelView({
  model,
}: {
  model: GroupChatPanelViewModel;
}) {
  return (
    <ConversationPanelLayout>
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
  const currentInteraction = model.composerInteraction.permissions[0] ?? null;
  const interactionSurface = currentInteraction ? (
    <ComposerInteractionSurface {...model.composerInteraction} />
  ) : undefined;
  return (
    <>
      <ConversationPanelViewportArea
        navigator={!isMobileLayout && model.sessionKey ? (
          <ConversationSessionNavigator
            {...model.navigator}
            className="absolute inset-y-0 left-3 z-20"
          />
        ) : undefined}
      >
        <ConversationPanelViewport
          floatingDockOccupied={
            model.executionPanel !== null
            || model.taskProcesses.length > 0
            || model.scrollToLatest.visible
          }
          isMobileLayout={isMobileLayout}
          viewport={viewport}
        >
          <AgentHandoffStatusProvider statuses={model.handoffStatuses}>
            <GroupConversationFeed {...model.feed} />
          </AgentHandoffStatusProvider>
        </ConversationPanelViewport>
      </ConversationPanelViewportArea>
      <ConversationPanelBottomArea
        activity={
          model.executionPanel
            ? <ExecutionProcessPanel {...model.executionPanel} />
            : model.taskProcesses.length > 0 && model.sessionKey
            ? (
                <RoomWorkspaceTaskPanel
                  processes={model.taskProcesses}
                  roomMembers={model.taskProcessMembers}
                  scopeKey={model.sessionKey}
                />
              )
            : undefined
        }
        goal={(
          <RoomGoalPanel
            {...model.goalPanel}
            isMobileLayout={isMobileLayout}
          />
        )}
        isMobileLayout={isMobileLayout}
        providerWarningVisible={model.providerWarningVisible}
        scrollToLatest={model.scrollToLatest}
      >
        <ComposerPanel
          {...model.composer}
          compact={isMobileLayout}
          goalModeExtra={<RoomGoalLeadControl {...model.goalLead} />}
          interactionIdentity={currentInteraction?.request_id ?? null}
          interactionSurface={interactionSurface}
        />
      </ConversationPanelBottomArea>
    </>
  );
}
