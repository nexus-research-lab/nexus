/**
 * INPUT: DM 单轮状态、消息 source 与渲染动作。
 * OUTPUT: 通用 MessageItem，或保持索引估高的未加载轮次测量边界。
 * POS: DM 静态与虚拟 Feed 共用的轮次展示节点。
 */
import type { Ref } from "react";

import { MessageItem } from "@/features/conversation/shared/message/item/message-item";

import {
  resolveRoundWorkspaceAgentId,
  type ConversationRoundRenderer,
  type ConversationRoundState,
  type ConversationRoundSource,
} from "./conversation-feed-model";
import { resolveConversationVirtualPlaceholderHeight } from "./use-conversation-virtual-scroll-policy";

interface ConversationRoundProps {
  isMobileLayout: boolean;
  measureRef?: Ref<HTMLDivElement>;
  placeholderHeight?: number;
  renderer: ConversationRoundRenderer;
  source: ConversationRoundSource;
  state: ConversationRoundState;
}

export function ConversationRound({
  isMobileLayout,
  measureRef,
  placeholderHeight,
  renderer,
  source,
  state,
}: ConversationRoundProps) {
  const workspaceAgentId = resolveRoundWorkspaceAgentId(
    state.messages,
    renderer.workspaceAgentId,
  );
  const resolvedPlaceholderHeight = resolveConversationVirtualPlaceholderHeight(
    state.isLoaded,
    placeholderHeight,
  );
  const showHistoryDivider = state.index > 0 && state.messages.some(
    (message) => message.role === "assistant"
      && message.metadata?.source === "echo",
  );

  return (
    <div
      ref={measureRef}
      className={isMobileLayout ? "pb-4" : "pb-1"}
      style={resolvedPlaceholderHeight
        ? { minHeight: resolvedPlaceholderHeight }
        : undefined}
      data-index={measureRef ? state.index : undefined}
      data-conversation-round-id={state.roundId}
      data-conversation-round-index={state.index}
      data-conversation-round-loaded={state.isLoaded ? "true" : "false"}
    >
      {showHistoryDivider ? (
        <div
          aria-label={renderer.historyDividerLabel}
          className="flex items-center gap-3 px-2 pb-2 pt-1 text-xs font-medium text-(--text-soft)"
          role="separator"
        >
          <span className="h-px flex-1 bg-(--content-divider-color)" />
          {renderer.historyDividerLabel ? (
            <span className="shrink-0">{renderer.historyDividerLabel}</span>
          ) : null}
          <span className="h-px flex-1 bg-(--content-divider-color)" />
        </div>
      ) : null}
      {state.isLoaded ? (
        <MessageItem
          animateEntry={false}
          assistantContentMode={state.isLive ? "dm_live" : "dm_archived"}
          compact={renderer.compact ?? false}
          currentAgentAvatar={renderer.currentAgentAvatar}
          currentAgentName={renderer.currentAgentName}
          isLastRound={state.isLast}
          isLoading={state.isLive}
          messages={state.messages}
          onEditUserMessage={
            state.isLast && !state.isLive
              ? renderer.onEditLastUserMessage
              : undefined
          }
          onOpenAgentContact={renderer.onOpenAgentContact}
          onForkConversation={
            !state.isLive && renderer.onForkRound
              ? () => renderer.onForkRound?.(state.roundId) ?? Promise.resolve()
              : undefined
          }
          onOpenSubagentTask={renderer.onOpenSubagentTask}
          onOpenWorkspaceFile={renderer.onOpenWorkspaceFile}
          onPermissionResponse={renderer.onPermissionResponse}
          pendingPermissions={state.isLive ? source.pendingPermissions : []}
          roundId={state.roundId}
          runtimePhase={state.isLive ? source.runtimePhase : null}
          workspaceAgentId={workspaceAgentId}
        />
      ) : null}
    </div>
  );
}
