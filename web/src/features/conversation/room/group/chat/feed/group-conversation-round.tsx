import type { Ref } from "react";

import { ConversationRoundPlaceholder } from "@/features/conversation/shared/timeline/round-placeholder";
import { MessageItem } from "@/features/conversation/shared/message/item/message-item";

import { hasRoomAgentRoundEntries } from "../../round/round-agent-model";
import { GroupRoundCardGroup } from "../../thread/round-card/group-round-card-group";
import {
  resolveRoundAgent,
  type GroupConversationRoundRenderer,
  type GroupConversationRoundState,
} from "./group-conversation-feed-model";

interface GroupConversationRoundProps {
  measureRef?: Ref<HTMLDivElement>;
  renderer: GroupConversationRoundRenderer;
  state: GroupConversationRoundState;
}

export function GroupConversationRound({
  measureRef,
  renderer,
  state,
}: GroupConversationRoundProps) {
  const { index, isLoaded, messages, pendingPermissions, pendingSlots, roundId } = state;
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
        <ConversationRoundPlaceholder />
      ) : hasRoomEntries ? (
        <GroupRoundCardGroup
          agentAvatarMap={renderer.agentAvatarMap}
          agentNameMap={renderer.agentNameMap}
          currentUserAvatar={renderer.currentUserAvatar}
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
      agentMentionDirectory={{ avatars: renderer.agentAvatarMap, names: renderer.agentNameMap }}
      isLastRound={state.isLast}
      messages={state.messages}
      onOpenAgentContact={renderer.onOpenAgentContact}
      onOpenWorkspaceFile={renderer.onOpenWorkspaceFile}
      onPermissionResponse={renderer.onPermissionResponse}
      onStopMessage={renderer.onStopMessage}
      roundId={state.roundId}
      workspaceAgentId={agent.id}
    />
  );
}
