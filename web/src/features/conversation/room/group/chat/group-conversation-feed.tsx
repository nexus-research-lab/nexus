import { memo, useEffect, useMemo, useRef } from "react";
import type { RefObject } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { MessageItem } from "@/features/conversation/shared/message";
import { has_room_agent_round_entries } from "@/features/conversation/shared/utils";
import { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import { Message, RoomPendingAgentSlotState } from "@/types/conversation/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/conversation/permission";
import { estimate_round_heights } from "@/hooks/conversation/use-message-height";
import { GroupRoundCardGroup } from "../thread/group-round-card-group";

interface GroupConversationFeedProps {
  bottom_anchor_ref: React.RefObject<HTMLDivElement | null>;
  feed_ref?: RefObject<HTMLDivElement | null>;
  /** The scrollable container — needed by the virtualizer */
  scroll_ref?: RefObject<HTMLDivElement | null>;
  compact?: boolean;
  current_agent_name: string | null;
  current_agent_avatar?: string | null;
  current_user_avatar?: string | null;
  /** Room 模式下的 agent_id → name 映射（用于多 Agent 显示） */
  agent_name_map?: Record<string, string>;
  /** Room 模式下的 agent_id → avatar 映射（用于多 Agent 显示） */
  agent_avatar_map?: Record<string, string | null>;
  is_last_round_pending_permissions: PendingPermission[];
  is_loading: boolean;
  runtime_phase?: AgentConversationRuntimePhase | null;
  live_round_ids: string[];
  is_mobile_layout: boolean;
  message_groups: Map<string, Message[]>;
  pending_permission_groups: Map<string, PendingPermission[]>;
  pending_slot_groups: Map<string, RoomPendingAgentSlotState[]>;
  on_open_workspace_file?: (path: string) => void;
  on_permission_response: (payload: PermissionDecisionPayload) => boolean;
  can_respond_to_permissions?: boolean;
  permission_read_only_reason?: string;
  /** Room 并发模式：停止单条消息生成 */
  on_stop_message?: (msg_id: string) => void;
  round_ids: string[];
}

// Minimum rounds before we enable virtualization — below this threshold the
// overhead is not worth it and scroll behaviour is simpler without it.
const VIRTUAL_THRESHOLD = 20;

/** Room 模式下从 round 的 assistant 消息中提取 agent_id，查找对应名字 */
function resolve_round_agent_name(
  messages: Message[],
  agent_name_map?: Record<string, string>,
): string | undefined {
  if (!agent_name_map) {
    return undefined;
  }
  const assistant_msg = messages.find((m) => m.role === "assistant");
  if (assistant_msg && "agent_id" in assistant_msg && assistant_msg.agent_id) {
    return agent_name_map[assistant_msg.agent_id];
  }
  return undefined;
}

/** Room 模式下从 round 的 assistant 消息中提取 agent_id，查找对应头像 */
function resolve_round_agent_avatar(
  messages: Message[],
  agent_avatar_map?: Record<string, string | null>,
): string | null | undefined {
  if (!agent_avatar_map) {
    return undefined;
  }
  const assistant_msg = messages.find((m) => m.role === "assistant");
  if (assistant_msg && "agent_id" in assistant_msg && assistant_msg.agent_id) {
    return agent_avatar_map[assistant_msg.agent_id];
  }
  return undefined;
}

export const GroupConversationFeed = memo(function GroupConversationFeed({
  bottom_anchor_ref,
  feed_ref,
  scroll_ref,
  compact = false,
  current_agent_name,
  current_agent_avatar,
  current_user_avatar,
  agent_name_map,
  agent_avatar_map,
  is_last_round_pending_permissions,
  is_loading,
  runtime_phase,
  live_round_ids,
  is_mobile_layout,
  message_groups,
  pending_permission_groups,
  pending_slot_groups,
  on_open_workspace_file,
  on_permission_response,
  can_respond_to_permissions = true,
  permission_read_only_reason,
  on_stop_message,
  round_ids,
}: GroupConversationFeedProps) {
  const use_virtual = round_ids.length >= VIRTUAL_THRESHOLD;

  if (use_virtual && scroll_ref) {
    return (
      <VirtualFeed
        bottom_anchor_ref={bottom_anchor_ref}
        feed_ref={feed_ref}
        scroll_ref={scroll_ref}
        compact={compact}
        current_agent_name={current_agent_name}
        current_agent_avatar={current_agent_avatar}
        current_user_avatar={current_user_avatar}
        agent_name_map={agent_name_map}
        agent_avatar_map={agent_avatar_map}
        is_last_round_pending_permissions={is_last_round_pending_permissions}
        is_loading={is_loading}
        runtime_phase={runtime_phase}
        live_round_ids={live_round_ids}
        is_mobile_layout={is_mobile_layout}
        message_groups={message_groups}
        pending_permission_groups={pending_permission_groups}
        pending_slot_groups={pending_slot_groups}
        on_open_workspace_file={on_open_workspace_file}
        on_permission_response={on_permission_response}
        can_respond_to_permissions={can_respond_to_permissions}
        permission_read_only_reason={permission_read_only_reason}
        on_stop_message={on_stop_message}
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
        const round_pending_permissions = pending_permission_groups.get(roundId) || [];
        const round_pending_slots = pending_slot_groups.get(roundId) || [];
        const isLastRound = idx === round_ids.length - 1;
        const is_last_round_live = isLastRound && live_round_ids.includes(roundId);
        const has_room_entries = has_room_agent_round_entries(roundMessages, round_pending_slots);

        // Group Room 中一旦出现 Agent 回复，就统一走 GroupRoundCardGroup。
        if (has_room_entries) {
          return (
            <GroupRoundCardGroup
              key={roundId}
              round_id={roundId}
              messages={roundMessages}
              pending_permissions={round_pending_permissions}
              pending_slots={round_pending_slots}
              agent_name_map={agent_name_map}
              agent_avatar_map={agent_avatar_map}
              current_user_avatar={current_user_avatar}
              is_last_round={isLastRound}
              is_loading={is_last_round_live}
              on_permission_response={on_permission_response}
              can_respond_to_permissions={can_respond_to_permissions}
              permission_read_only_reason={permission_read_only_reason}
              on_stop_message={on_stop_message}
              on_open_workspace_file={on_open_workspace_file}
            />
          );
        }

        // 纯用户轮次或尚未分配到 Agent 的轮次，沿用 MessageItem。
        const round_agent_name = resolve_round_agent_name(roundMessages, agent_name_map) ?? current_agent_name;
        const round_agent_avatar = resolve_round_agent_avatar(roundMessages, agent_avatar_map) ?? current_agent_avatar;
        return (
          <MessageItem
            key={roundId}
            compact={compact}
            current_agent_name={round_agent_name}
            current_agent_avatar={round_agent_avatar}
            current_user_avatar={current_user_avatar}
            round_id={roundId}
            messages={roundMessages}
            is_last_round={isLastRound}
            is_loading={is_last_round_live}
            runtime_phase={is_last_round_live ? runtime_phase : null}
            pending_permissions={is_last_round_live ? is_last_round_pending_permissions : []}
            on_permission_response={on_permission_response}
            can_respond_to_permissions={can_respond_to_permissions}
            permission_read_only_reason={permission_read_only_reason}
            on_open_workspace_file={on_open_workspace_file}
            on_stop_message={on_stop_message}
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
  current_agent_avatar,
  current_user_avatar,
  agent_name_map,
  agent_avatar_map,
  is_last_round_pending_permissions,
  is_loading,
  runtime_phase,
  live_round_ids,
  is_mobile_layout,
  message_groups,
  pending_permission_groups,
  pending_slot_groups,
  on_open_workspace_file,
  on_permission_response,
  can_respond_to_permissions = true,
  permission_read_only_reason,
  on_stop_message,
  round_ids,
}: Omit<GroupConversationFeedProps, "scroll_ref"> & { scroll_ref: RefObject<HTMLDivElement | null> }) {
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
    () => estimate_round_heights(round_ids, message_groups, container_width_ref.current),
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
          const round_pending_permissions = pending_permission_groups.get(roundId) || [];
          const round_pending_slots = pending_slot_groups.get(roundId) || [];
          const isLastRound = virtual_item.index === round_ids.length - 1;
          const is_last_round_live = isLastRound && live_round_ids.includes(roundId);
          const has_room_entries = has_room_agent_round_entries(roundMessages, round_pending_slots);

          return (
            <div
              key={roundId}
              data-index={virtual_item.index}
              ref={virtualizer.measureElement}
            >
              {has_room_entries ? (
                <GroupRoundCardGroup
                  round_id={roundId}
                  messages={roundMessages}
                  pending_permissions={round_pending_permissions}
                  pending_slots={round_pending_slots}
                  agent_name_map={agent_name_map}
                  agent_avatar_map={agent_avatar_map}
                  current_user_avatar={current_user_avatar}
                  is_last_round={isLastRound}
                  is_loading={is_last_round_live}
                  on_permission_response={on_permission_response}
                  can_respond_to_permissions={can_respond_to_permissions}
                  permission_read_only_reason={permission_read_only_reason}
                  on_stop_message={on_stop_message}
                  on_open_workspace_file={on_open_workspace_file}
                />
              ) : (
                <MessageItem
                  compact={compact}
                  current_agent_name={resolve_round_agent_name(roundMessages, agent_name_map) ?? current_agent_name}
                  current_agent_avatar={resolve_round_agent_avatar(roundMessages, agent_avatar_map) ?? current_agent_avatar}
                  current_user_avatar={current_user_avatar}
                  round_id={roundId}
                  messages={roundMessages}
                  is_last_round={isLastRound}
                  is_loading={is_last_round_live}
                  runtime_phase={is_last_round_live ? runtime_phase : null}
                  pending_permissions={is_last_round_live ? is_last_round_pending_permissions : []}
                  on_permission_response={on_permission_response}
                  can_respond_to_permissions={can_respond_to_permissions}
                  permission_read_only_reason={permission_read_only_reason}
                  on_open_workspace_file={on_open_workspace_file}
                  on_stop_message={on_stop_message}
                />
              )}
            </div>
          );
        })}
      </div>
      <div ref={bottom_anchor_ref} className="absolute bottom-0 h-px w-full" />
    </div>
  );
}
