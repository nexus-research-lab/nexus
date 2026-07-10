import type { Ref } from "react";

import { MessageItem } from "@/features/conversation/shared/message";
import type { SessionRoundIndexItem } from "@/types/conversation/room";

import { ConversationRoundPlaceholder } from "../conversation-round-placeholder";
import {
  resolveRoundWorkspaceAgentId,
  type ConversationRoundRenderer,
  type ConversationRoundState,
  type ConversationRoundSource,
} from "./conversation-feed-model";

interface ConversationRoundProps {
  indexItem?: SessionRoundIndexItem;
  measureRef?: Ref<HTMLDivElement>;
  renderer: ConversationRoundRenderer;
  source: ConversationRoundSource;
  state: ConversationRoundState;
}

export function ConversationRound({
  indexItem,
  measureRef,
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
      data-index={measureRef ? state.index : undefined}
      data-conversation-round-id={state.roundId}
      data-conversation-round-index={state.index}
      data-conversation-round-loaded={state.isLoaded ? "true" : "false"}
    >
      {state.isLoaded ? (
        <MessageItem
          assistantContentMode={state.isLive ? "dm_live" : "dm_archived"}
          compact={renderer.compact ?? false}
          currentAgentAvatar={renderer.currentAgentAvatar}
          currentAgentName={renderer.currentAgentName}
          currentUserAvatar={renderer.currentUserAvatar}
          isLastRound={state.isLast}
          isLoading={state.isLive}
          messages={state.messages}
          onEditUserMessage={
            state.isLast && !state.isLive
              ? renderer.onEditLastUserMessage
              : undefined
          }
          onOpenAgentContact={renderer.onOpenAgentContact}
          onOpenWorkspaceFile={renderer.onOpenWorkspaceFile}
          onPermissionResponse={renderer.onPermissionResponse}
          pendingPermissions={state.isLive ? source.pendingPermissions : []}
          roundId={state.roundId}
          runtimePhase={state.isLive ? source.runtimePhase : null}
          workspaceAgentId={workspaceAgentId}
        />
      ) : (
        <ConversationRoundPlaceholder
          indexItem={indexItem}
          roundId={state.roundId}
        />
      )}
    </div>
  );
}
