import { memo, useEffect, useMemo, useRef } from "react";
import type { RefObject } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { MessageItem } from "@/features/room-conversation/message";
import { Message } from "@/types/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/permission";
import { estimateRoundHeights } from "@/hooks/use-message-height";

interface RoomConversationFeedProps {
  bottom_anchor_ref: React.RefObject<HTMLDivElement | null>;
  feed_ref?: RefObject<HTMLDivElement | null>;
  /** The scrollable container — needed by the virtualizer */
  scroll_ref?: RefObject<HTMLDivElement | null>;
  compact?: boolean;
  current_agent_name: string | null;
  is_last_round_pending_permission: PendingPermission | null;
  is_loading: boolean;
  is_mobile_layout: boolean;
  message_groups: Map<string, Message[]>;
  on_delete_round: (round_id: string) => Promise<void>;
  on_open_workspace_file?: (path: string) => void;
  on_permission_response: (payload: PermissionDecisionPayload) => boolean;
  on_regenerate_round?: (round_id: string) => Promise<void>;
  round_ids: string[];
}

// Minimum rounds before we enable virtualization — below this threshold the
// overhead is not worth it and scroll behaviour is simpler without it.
const VIRTUAL_THRESHOLD = 20;

export const RoomConversationFeed = memo(function RoomConversationFeed({
  bottom_anchor_ref,
  feed_ref,
  scroll_ref,
  compact = false,
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
  const use_virtual = round_ids.length >= VIRTUAL_THRESHOLD;

  if (use_virtual && scroll_ref) {
    return (
      <VirtualFeed
        bottom_anchor_ref={bottom_anchor_ref}
        feed_ref={feed_ref}
        scroll_ref={scroll_ref}
        compact={compact}
        current_agent_name={current_agent_name}
        is_last_round_pending_permission={is_last_round_pending_permission}
        is_loading={is_loading}
        is_mobile_layout={is_mobile_layout}
        message_groups={message_groups}
        on_delete_round={on_delete_round}
        on_open_workspace_file={on_open_workspace_file}
        on_permission_response={on_permission_response}
        on_regenerate_round={on_regenerate_round}
        round_ids={round_ids}
      />
    );
  }

  return (
    <div
      ref={feed_ref}
      className={is_mobile_layout ? "space-y-4" : "mx-auto flex w-full max-w-[980px] flex-col gap-1"}
    >
      {round_ids.map((roundId, idx) => {
        const roundMessages = message_groups.get(roundId) || [];
        const isLastRound = idx === round_ids.length - 1;

        return (
          <MessageItem
            key={roundId}
            compact={compact}
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
});

// ─── VirtualFeed ──────────────────────────────────────────────────────────────

function VirtualFeed({
  bottom_anchor_ref,
  feed_ref,
  scroll_ref,
  compact,
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
}: Omit<RoomConversationFeedProps, "scroll_ref"> & { scroll_ref: RefObject<HTMLDivElement | null> }) {
  const container_ref = useRef<HTMLDivElement>(null);

  // Measure scroll container width for pretext height estimation
  const container_width_ref = useRef(680);
  useEffect(() => {
    const el = scroll_ref.current;
    if (!el) return;
    container_width_ref.current = el.clientWidth || 680;
    const observer = new ResizeObserver(() => {
      container_width_ref.current = el.clientWidth || 680;
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, [scroll_ref]);

  // Pretext-based height estimates (recomputed when round count changes)
  const height_map = useMemo(
    () => estimateRoundHeights(round_ids, message_groups, container_width_ref.current),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [round_ids.length, message_groups],
  );

  const virtualizer = useVirtualizer({
    count: round_ids.length,
    getScrollElement: () => scroll_ref.current,
    estimateSize: (i) => height_map.get(round_ids[i]) ?? 200,
    overscan: 5,
    // Allow measured sizes to override estimates as items render
    measureElement: (el) => el.getBoundingClientRect().height,
  });

  const virtual_items = virtualizer.getVirtualItems();
  const total_size = virtualizer.getTotalSize();

  return (
    <div
      ref={(el) => {
        // Merge feed_ref with container_ref
        container_ref.current = el;
        if (feed_ref) (feed_ref as React.MutableRefObject<HTMLDivElement | null>).current = el;
      }}
      className={is_mobile_layout ? "relative" : "relative mx-auto w-full max-w-[980px]"}
      style={{ height: total_size }}
    >
      <div
        style={{
          position: "absolute",
          top: 0,
          left: 0,
          width: "100%",
          transform: `translateY(${virtual_items[0]?.start ?? 0}px)`,
        }}
      >
        {virtual_items.map((virtual_item) => {
          const roundId = round_ids[virtual_item.index];
          const roundMessages = message_groups.get(roundId) || [];
          const isLastRound = virtual_item.index === round_ids.length - 1;

          return (
            <div
              key={roundId}
              data-index={virtual_item.index}
              ref={virtualizer.measureElement}
            >
              <MessageItem
                compact={compact}
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
            </div>
          );
        })}
      </div>
      <div ref={bottom_anchor_ref} className="absolute bottom-0 h-px w-full" />
    </div>
  );
}
