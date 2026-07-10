import type { Ref } from "react";

import { ConversationRoundPlaceholder } from "@/features/conversation/shared/timeline/round-placeholder";
import { MessageItem } from "@/features/conversation/shared/message";
import { hasRoomAgentRoundEntries } from "@/features/conversation/shared/utils";
import type { SessionRoundIndexItem } from "@/types/conversation/room";

import { GroupRoundCardGroup } from "../../thread/group-round-card-group";
import {
  resolveRoundAgent,
  type GroupConversationRoundRenderer,
  type GroupConversationRoundState,
} from "./group-conversation-feed-model";

interface GroupConversationRoundProps {
  indexItem?: SessionRoundIndexItem;
  measureRef?: Ref<HTMLDivElement>;
  renderer: GroupConversationRoundRenderer;
  state: GroupConversationRoundState;
}

export function GroupConversationRound({
  indexItem,
  measureRef,
  renderer,
  state,
}: GroupConversationRoundProps) {
  const { index, isLast, isLive, isLoaded, messages, pendingPermissions, pendingSlots, roundId } = state;
  const hasRoomEntries = hasRoomAgentRoundEntries(messages, pendingSlots);

  return (
    <div
      ref={measureRef}
      data-index={measureRef ? index : undefined}
      data-conversation-round-id={roundId}
      data-conversation-round-index={index}
      data-conversation-round-loaded={isLoaded ? "true" : "false"}
    >
      {!isLoaded ? (
        <ConversationRoundPlaceholder indexItem={indexItem} roundId={roundId} />
      ) : hasRoomEntries ? (
        <GroupRoundCardGroup
          agentAvatarMap={renderer.agentAvatarMap}
          agentNameMap={renderer.agentNameMap}
          currentUserAvatar={renderer.currentUserAvatar}
          isLastRound={isLast}
          isLoading={isLive}
          messages={messages}
          onOpenAgentContact={renderer.onOpenAgentContact}
          onOpenWorkspaceFile={renderer.onOpenWorkspaceFile}
          onPermissionResponse={renderer.onPermissionResponse}
          onStopMessage={renderer.onStopMessage}
          pendingPermissions={pendingPermissions}
          pendingSlots={pendingSlots}
          roundId={roundId}
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
      compact={renderer.compact ?? false}
      currentAgentAvatar={agent.avatar}
      currentAgentName={agent.name}
      currentUserAvatar={renderer.currentUserAvatar}
      isLastRound={state.isLast}
      isLoading={state.isLive}
      messages={state.messages}
      onOpenAgentContact={renderer.onOpenAgentContact}
      onOpenWorkspaceFile={renderer.onOpenWorkspaceFile}
      onPermissionResponse={renderer.onPermissionResponse}
      onStopMessage={renderer.onStopMessage}
      pendingPermissions={
        state.isLive ? renderer.isLastRoundPendingPermissions : []
      }
      roundId={state.roundId}
      runtimePhase={state.isLive ? renderer.runtimePhase : null}
      workspaceAgentId={agent.id}
    />
  );
}
