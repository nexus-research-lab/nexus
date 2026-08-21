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

  return (
    <div
      ref={measureRef}
      className={isMobileLayout ? "pb-4" : "pb-1"}
      style={placeholderHeight ? { minHeight: placeholderHeight } : undefined}
      data-index={measureRef ? state.index : undefined}
      data-conversation-round-id={state.roundId}
      data-conversation-round-index={state.index}
      data-conversation-round-loaded={state.isLoaded ? "true" : "false"}
    >
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
