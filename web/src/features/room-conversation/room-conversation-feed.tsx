import { MessageItem } from "@/features/room-conversation/message";
import { RoomConversationFeedProps } from "@/types/room-conversation";

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
            current_agent_name={current_agent_name}
            round_id={roundId}
            messages={roundMessages}
            is_last_round={isLastRound}
            is_loading={is_loading}
            pending_permission={isLastRound ? is_last_round_pending_permission : null}
            on_permission_response={on_permission_response}
            on_open_workspace_file={on_open_workspace_file}
            on_delete={on_delete_round}
            on_regenerate={isLastRound ? on_regenerate_round : undefined}
          />
        );
      })}
      <div ref={bottom_anchor_ref} className="h-px w-full" />
    </div>
  );
}
