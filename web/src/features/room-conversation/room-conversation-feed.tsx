import { RefObject } from "react";

import { MessageItem } from "@/features/room-conversation/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/permission";
import { Message } from "@/types/message";

interface RoomConversationFeedProps {
  bottom_anchor_ref: RefObject<HTMLDivElement | null>;
  current_agent_name: string | null;
  is_last_round_pending_permission: PendingPermission | null;
  is_loading: boolean;
  is_mobile_layout: boolean;
  message_groups: Map<string, Message[]>;
  on_delete_round: (round_id: string) => Promise<void>;
  on_open_workspace_file?: (path: string) => void;
  on_permission_response: (payload: PermissionDecisionPayload) => void;
  on_regenerate_round?: (round_id: string) => Promise<void>;
  round_ids: string[];
}

export function RoomConversationFeed({
  bottom_anchor_ref,
  current_agent_name,
  is_last_round_pending_permission,
  is_loading,
  is_mobile_layout,
  message_groups,
  on_delete_round,
  on_open_workspace_file,
  on_permission_response,
  on_regenerate_round,
  round_ids,
}: RoomConversationFeedProps) {
  return (
    <div className={is_mobile_layout ? "space-y-4" : "mx-auto flex w-full max-w-[980px] flex-col gap-6 xl:gap-8"}>
      {round_ids.map((roundId, idx) => {
        const roundMessages = message_groups.get(roundId) || [];
        const isLastRound = idx === round_ids.length - 1;

        return (
          <MessageItem
            key={roundId}
            currentAgentName={current_agent_name}
            roundId={roundId}
            messages={roundMessages}
            isLastRound={isLastRound}
            isLoading={is_loading}
            pendingPermission={isLastRound ? is_last_round_pending_permission : null}
            onPermissionResponse={on_permission_response}
            onOpenWorkspaceFile={on_open_workspace_file}
            onDelete={on_delete_round}
            onRegenerate={isLastRound ? on_regenerate_round : undefined}
          />
        );
      })}
      <div ref={bottom_anchor_ref} className="h-px w-full" />
    </div>
  );
}
