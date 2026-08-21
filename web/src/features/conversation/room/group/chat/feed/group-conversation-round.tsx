/**
 * INPUT: Room feed 节点、Agent 目录、权限与交互回调。
 * OUTPUT: Agent 执行卡或普通 root 轮次，并暴露稳定轮次身份与测量边界。
 * POS: Group feed 单节点的唯一渲染分派入口。
 */
import type { Ref } from "react";

import { MessageItem } from "@/features/conversation/shared/message/item/message-item";

import { hasRoomAgentRoundEntries } from "../../round/round-agent-model";
import { GroupRoundCardGroup } from "../../thread/round-card/group-round-card-group";
import {
  resolveRoundAgent,
  type GroupConversationRoundRenderer,
  type GroupConversationRoundState,
} from "./group-conversation-feed-model";

interface GroupConversationRoundProps {
  isMobileLayout: boolean;
  measureRef?: Ref<HTMLDivElement>;
  renderer: GroupConversationRoundRenderer;
  state: GroupConversationRoundState;
}

export function GroupConversationRound({
  isMobileLayout,
  measureRef,
  renderer,
  state,
}: GroupConversationRoundProps) {
  const {
    index,
    isLoaded,
    messages,
    pendingPermissions,
    pendingSlots,
    roomAgentExecutionStates,
    rootRoundId,
    roundId,
  } = state;
  const hasRoomEntries = hasRoomAgentRoundEntries(
    messages,
    pendingSlots,
    pendingPermissions,
    roomAgentExecutionStates,
  );

  return (
    <div
      ref={measureRef}
      className={`relative ${isMobileLayout ? "pb-4" : "pb-1"}`}
      data-index={measureRef ? index : undefined}
      data-conversation-round-id={roundId}
      data-conversation-root-round-id={
        rootRoundId === roundId ? undefined : rootRoundId
      }
      data-conversation-round-index={index}
      data-conversation-round-loaded={isLoaded ? "true" : "false"}
    >
      {!isLoaded ? null : hasRoomEntries ? (
        <GroupRoundCardGroup
          agentAvatarMap={renderer.agentAvatarMap}
          agentNameMap={renderer.agentNameMap}
          messages={messages}
          onOpenAgentContact={renderer.onOpenAgentContact}
          onOpenSubagentTask={renderer.onOpenSubagentTask}
          onOpenWorkspaceFile={renderer.onOpenWorkspaceFile}
          onPermissionResponse={renderer.onPermissionResponse}
          onStopAgentRound={renderer.onStopAgentRound}
          pendingPermissions={pendingPermissions}
          pendingSlots={pendingSlots}
          roomAgentExecutionStates={roomAgentExecutionStates}
          roundId={rootRoundId}
          stoppingAgentRoundIds={renderer.stoppingAgentRoundIds}
        />
      ) : (
        <StandaloneConversationRound renderer={renderer} state={state} />
      )}
    </div>
  );
}

function StandaloneConversationRound({
  renderer,
  state,
}: Pick<GroupConversationRoundProps, "renderer" | "state">) {
  const agent = resolveRoundAgent(state.messages, renderer);
  return (
    <MessageItem
      animateEntry={false}
      compact={renderer.compact ?? false}
      currentAgentAvatar={agent.avatar}
      currentAgentName={agent.name}
      agentMentionDirectory={{ avatars: renderer.agentAvatarMap, names: renderer.agentNameMap }}
      isLastRound={state.isLast}
      messages={state.messages}
      onOpenAgentContact={renderer.onOpenAgentContact}
      onOpenSubagentTask={renderer.onOpenSubagentTask}
      onOpenWorkspaceFile={renderer.onOpenWorkspaceFile}
      onPermissionResponse={renderer.onPermissionResponse}
      pendingPermissions={state.pendingPermissions}
      roundId={state.rootRoundId}
      workspaceAgentId={agent.id}
    />
  );
}
