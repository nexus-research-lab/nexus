/**
 * INPUT: DM 会话 Frame、Feed、Goal、Task 与 Composer 视图模型。
 * OUTPUT: 共享 viewport 与从 Composer 向上堆叠 Goal、Task、滚动入口的 DM 对话布局。
 * POS: DM 面板的纯视图层。
 */

import type { ComponentProps } from "react";

import {
  ComposerInteractionSurface,
  type ComposerInteractionSurfaceProps,
} from "@/features/conversation/shared/composer/components/interaction/composer-interaction-surface";
import { ComposerPanel } from "@/features/conversation/shared/composer/composer-panel";
import {
  ConversationPanelBottomArea,
  ConversationPanelLayout,
  ConversationPanelViewport,
  ConversationPanelViewportArea,
} from "@/features/conversation/shared/conversation-panel-layout";
import type { ConversationPanelFrameModel } from "@/features/conversation/shared/conversation-panel-model";
import { ConversationFeed } from "@/features/conversation/shared/feed/conversation-feed";
import { ExecutionProcessPanel } from "@/features/conversation/shared/execution/execution-process-panel";
import { GoalPanel } from "@/features/conversation/shared/goal/goal-panel";
import { ConversationSessionNavigator } from "@/features/conversation/shared/session-navigator/conversation-session-navigator";
import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import {
  WorkspaceTaskPanel,
  type WorkspaceTaskSource,
} from "@/shared/ui/workspace/surface/workspace-task-strip";
import type { TodoItem } from "@/types/conversation/todo";

export type DmChatComposerModel = Omit<
  ComponentProps<typeof ComposerPanel>,
  "compact"
>;
type FeedModel = ComponentProps<typeof ConversationFeed>;
type GoalPanelModel = Omit<ComponentProps<typeof GoalPanel>, "compact">;

export interface DmChatPanelViewModel extends ConversationPanelFrameModel {
  embedded: boolean;
  composer: DmChatComposerModel;
  composerInteraction: ComposerInteractionSurfaceProps;
  feed: FeedModel;
  goalPanel: GoalPanelModel;
  executionPanel: ComponentProps<typeof ExecutionProcessPanel> | null;
  taskSource?: WorkspaceTaskSource;
  todos: TodoItem[];
}

export function DmChatPanelView({
  model,
}: {
  model: DmChatPanelViewModel;
}) {
  const { isMobileLayout, viewport } = model;
  const currentInteraction = model.composerInteraction.permissions[0] ?? null;
  const interactionSurface = currentInteraction ? (
    <ComposerInteractionSurface {...model.composerInteraction} />
  ) : undefined;
  return (
    <ConversationPanelLayout>
      <ConversationPanelViewportArea
        navigator={!model.embedded && !isMobileLayout && model.sessionKey ? (
          <ConversationSessionNavigator
            {...model.navigator}
            className="absolute inset-y-0 left-3 z-20"
          />
        ) : undefined}
      >
        <ConversationPanelViewport
          floatingDockOccupied={model.scrollToLatest.visible || (!model.embedded && (
            model.executionPanel !== null || model.todos.length > 0
          ))}
          isMobileLayout={isMobileLayout}
          tourAnchor={CONVERSATION_TOUR_ANCHORS.feed}
          viewport={viewport}
        >
          <ConversationFeed {...model.feed} />
        </ConversationPanelViewport>
      </ConversationPanelViewportArea>
      <ConversationPanelBottomArea
        activity={!model.embedded ? (
          model.executionPanel
            ? <ExecutionProcessPanel {...model.executionPanel} />
            : model.todos.length > 0
            ? (
                <WorkspaceTaskPanel
                  source={model.taskSource}
                  todos={model.todos}
                />
              )
            : undefined
        ) : undefined}
        goal={!model.embedded ? <GoalPanel {...model.goalPanel} compact={isMobileLayout} /> : undefined}
        isMobileLayout={isMobileLayout}
        providerWarningVisible={model.providerWarningVisible}
        scrollToLatest={model.scrollToLatest}
      >
        <ComposerPanel
          {...model.composer}
          compact={isMobileLayout}
          interactionIdentity={currentInteraction?.request_id ?? null}
          interactionSurface={interactionSurface}
        />
      </ConversationPanelBottomArea>
    </ConversationPanelLayout>
  );
}
