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
  on_open_agent_contact?: (agentId: string) => void;
  on_open_workspace_file?: (path: string) => void;
  on_permission_response: (payload: PermissionDecisionPayload) => boolean;
  can_respond_to_permissions?: boolean;
  permission_read_only_reason?: string;
  /** Room 并发模式：停止单条消息生成 */
  on_stop_message?: (msgId: string) => void;
  round_ids: string[];
}

// Minimum rounds before we enable virtualization — below this threshold the
// overhead is not worth it and scroll behaviour is simpler without it.
const VIRTUAL_THRESHOLD = 20;

/** Room 模式下从 round 的 assistant 消息中提取 agent_id，查找对应名字 */
function resolveRoundAgentName(
  messages: Message[],
  agentNameMap?: Record<string, string>,
): string | undefined {
  if (!agentNameMap) {
    return undefined;
  }
  const assistantMsg = messages.find((m) => m.role === "assistant");
  if (assistantMsg && "agent_id" in assistantMsg && assistantMsg.agent_id) {
    return agentNameMap[assistantMsg.agent_id];
  }
  return undefined;
}

/** Room 模式下从 round 的 assistant 消息中提取 agent_id，查找对应头像 */
function resolveRoundAgentAvatar(
  messages: Message[],
  agentAvatarMap?: Record<string, string | null>,
): string | null | undefined {
  if (!agentAvatarMap) {
    return undefined;
  }
  const assistantMsg = messages.find((m) => m.role === "assistant");
  if (assistantMsg && "agent_id" in assistantMsg && assistantMsg.agent_id) {
    return agentAvatarMap[assistantMsg.agent_id];
  }
  return undefined;
}

/** Markdown 中的 workspace 文件按产出它的 Agent workspace 下载。 */
function resolveRoundAgentId(messages: Message[]): string | null {
  const assistantMsg = messages.find((message) => message.role === "assistant");
  if (assistantMsg && "agent_id" in assistantMsg && assistantMsg.agent_id) {
    return assistantMsg.agent_id;
  }
  return null;
}

export const GroupConversationFeed = memo(function GroupConversationFeed({
  bottom_anchor_ref: bottomAnchorRef,
  feed_ref: feedRef,
  scroll_ref: scrollRef,
  compact = false,
  current_agent_name: currentAgentName,
  current_agent_avatar: currentAgentAvatar,
  current_user_avatar: currentUserAvatar,
  agent_name_map: agentNameMap,
  agent_avatar_map: agentAvatarMap,
  is_last_round_pending_permissions: isLastRoundPendingPermissions,
  runtime_phase: runtimePhase,
  live_round_ids: liveRoundIds,
  is_mobile_layout: isMobileLayout,
  message_groups: messageGroups,
  pending_permission_groups: pendingPermissionGroups,
  pending_slot_groups: pendingSlotGroups,
  on_open_agent_contact: onOpenAgentContact,
  on_open_workspace_file: onOpenWorkspaceFile,
  on_permission_response: onPermissionResponse,
  can_respond_to_permissions: canRespondToPermissions = true,
  permission_read_only_reason: permissionReadOnlyReason,
  on_stop_message: onStopMessage,
  round_ids: roundIds,
}: GroupConversationFeedProps) {
  const useVirtual = roundIds.length >= VIRTUAL_THRESHOLD;

  if (useVirtual && scrollRef) {
    return (
      <VirtualFeed
        bottom_anchor_ref={bottomAnchorRef}
        feed_ref={feedRef}
        scroll_ref={scrollRef}
        compact={compact}
        current_agent_name={currentAgentName}
        current_agent_avatar={currentAgentAvatar}
        current_user_avatar={currentUserAvatar}
        agent_name_map={agentNameMap}
        agent_avatar_map={agentAvatarMap}
        is_last_round_pending_permissions={isLastRoundPendingPermissions}
        runtime_phase={runtimePhase}
        live_round_ids={liveRoundIds}
        is_mobile_layout={isMobileLayout}
        message_groups={messageGroups}
        pending_permission_groups={pendingPermissionGroups}
        pending_slot_groups={pendingSlotGroups}
        on_open_agent_contact={onOpenAgentContact}
        on_open_workspace_file={onOpenWorkspaceFile}
        on_permission_response={onPermissionResponse}
        can_respond_to_permissions={canRespondToPermissions}
        permission_read_only_reason={permissionReadOnlyReason}
        on_stop_message={onStopMessage}
        round_ids={roundIds}
      />
    );
  }

  return (
    <div
      ref={feedRef}
      className={isMobileLayout ? "nexus-chat-feed space-y-4" : "nexus-chat-feed mx-auto flex w-full max-w-[980px] flex-col gap-1"}
    >
      {roundIds.map((roundId, idx) => {
        const roundMessages = messageGroups.get(roundId) || [];
        const roundPendingPermissions = pendingPermissionGroups.get(roundId) || [];
        const roundPendingSlots = pendingSlotGroups.get(roundId) || [];
        const isLastRound = idx === roundIds.length - 1;
        const isLastRoundLive = isLastRound && liveRoundIds.includes(roundId);
        const hasRoomEntries = has_room_agent_round_entries(roundMessages, roundPendingSlots);

        // Group Room 中一旦出现 Agent 回复，就统一走 GroupRoundCardGroup。
        if (hasRoomEntries) {
          return (
            <GroupRoundCardGroup
              key={roundId}
              round_id={roundId}
              messages={roundMessages}
              pending_permissions={roundPendingPermissions}
              pending_slots={roundPendingSlots}
              agent_name_map={agentNameMap}
              agent_avatar_map={agentAvatarMap}
              current_user_avatar={currentUserAvatar}
              is_last_round={isLastRound}
              is_loading={isLastRoundLive}
              on_permission_response={onPermissionResponse}
              can_respond_to_permissions={canRespondToPermissions}
              permission_read_only_reason={permissionReadOnlyReason}
              on_open_agent_contact={onOpenAgentContact}
              on_stop_message={onStopMessage}
              on_open_workspace_file={onOpenWorkspaceFile}
            />
          );
        }

        // 纯用户轮次或尚未分配到 Agent 的轮次，沿用 MessageItem。
        const roundAgentName = resolveRoundAgentName(roundMessages, agentNameMap) ?? currentAgentName;
        const roundAgentAvatar = resolveRoundAgentAvatar(roundMessages, agentAvatarMap) ?? currentAgentAvatar;
        const roundWorkspaceAgentId = resolveRoundAgentId(roundMessages);
        return (
          <MessageItem
            key={roundId}
            compact={compact}
            current_agent_name={roundAgentName}
            current_agent_avatar={roundAgentAvatar}
            workspace_agent_id={roundWorkspaceAgentId}
            current_user_avatar={currentUserAvatar}
            round_id={roundId}
            messages={roundMessages}
            is_last_round={isLastRound}
            is_loading={isLastRoundLive}
            runtime_phase={isLastRoundLive ? runtimePhase : null}
            pending_permissions={isLastRoundLive ? isLastRoundPendingPermissions : []}
            on_permission_response={onPermissionResponse}
            can_respond_to_permissions={canRespondToPermissions}
            permission_read_only_reason={permissionReadOnlyReason}
            on_open_agent_contact={onOpenAgentContact}
            on_open_workspace_file={onOpenWorkspaceFile}
            on_stop_message={onStopMessage}
          />
        );
      })}
      <div ref={bottomAnchorRef} className="h-px w-full" />
    </div>
  );
});

// ─── VirtualFeed ──────────────────────────────────────────────────────────────

function VirtualFeed({
  bottom_anchor_ref: bottomAnchorRef,
  feed_ref: feedRef,
  scroll_ref: scrollRef,
  compact,
  current_agent_name: currentAgentName,
  current_agent_avatar: currentAgentAvatar,
  current_user_avatar: currentUserAvatar,
  agent_name_map: agentNameMap,
  agent_avatar_map: agentAvatarMap,
  is_last_round_pending_permissions: isLastRoundPendingPermissions,
  runtime_phase: runtimePhase,
  live_round_ids: liveRoundIds,
  is_mobile_layout: isMobileLayout,
  message_groups: messageGroups,
  pending_permission_groups: pendingPermissionGroups,
  pending_slot_groups: pendingSlotGroups,
  on_open_agent_contact: onOpenAgentContact,
  on_open_workspace_file: onOpenWorkspaceFile,
  on_permission_response: onPermissionResponse,
  can_respond_to_permissions: canRespondToPermissions = true,
  permission_read_only_reason: permissionReadOnlyReason,
  on_stop_message: onStopMessage,
  round_ids: roundIds,
}: Omit<GroupConversationFeedProps, "is_loading" | "scroll_ref"> & { scroll_ref: RefObject<HTMLDivElement | null> }) {
  const containerRef = useRef<HTMLDivElement>(null);

  // Measure scroll container width for pretext height estimation
  const containerWidthRef = useRef(680);
  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    containerWidthRef.current = el.clientWidth || 680;
    const observer = new ResizeObserver(() => {
      containerWidthRef.current = el.clientWidth || 680;
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, [scrollRef]);

  // Pretext-based height estimates (recomputed when round count changes)
  const heightMap = useMemo(
    () => estimate_round_heights(roundIds, messageGroups, containerWidthRef.current),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [roundIds.length, messageGroups],
  );

  const virtualizer = useVirtualizer({
    count: roundIds.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: (i) => heightMap.get(roundIds[i]) ?? 200,
    overscan: 5,
    // Allow measured sizes to override estimates as items render
    measureElement: (el) => el.getBoundingClientRect().height,
  });

  const virtualItems = virtualizer.getVirtualItems();
  const totalSize = virtualizer.getTotalSize();

  return (
    <div
      ref={(el) => {
        // Merge feed_ref with container_ref
        containerRef.current = el;
        if (feedRef) (feedRef as React.MutableRefObject<HTMLDivElement | null>).current = el;
      }}
      className={isMobileLayout ? "nexus-chat-feed relative" : "nexus-chat-feed relative mx-auto w-full max-w-[980px]"}
      style={{ height: totalSize }}
    >
      <div
        style={{
          position: "absolute",
          top: 0,
          left: 0,
          width: "100%",
          transform: `translateY(${virtualItems[0]?.start ?? 0}px)`,
        }}
      >
        {virtualItems.map((virtualItem) => {
          const roundId = roundIds[virtualItem.index];
          const roundMessages = messageGroups.get(roundId) || [];
          const roundPendingPermissions = pendingPermissionGroups.get(roundId) || [];
          const roundPendingSlots = pendingSlotGroups.get(roundId) || [];
          const isLastRound = virtualItem.index === roundIds.length - 1;
          const isLastRoundLive = isLastRound && liveRoundIds.includes(roundId);
          const hasRoomEntries = has_room_agent_round_entries(roundMessages, roundPendingSlots);

          return (
            <div
              key={roundId}
              data-index={virtualItem.index}
              ref={virtualizer.measureElement}
            >
              {hasRoomEntries ? (
                <GroupRoundCardGroup
                  round_id={roundId}
                  messages={roundMessages}
                  pending_permissions={roundPendingPermissions}
                  pending_slots={roundPendingSlots}
                  agent_name_map={agentNameMap}
                  agent_avatar_map={agentAvatarMap}
                  current_user_avatar={currentUserAvatar}
                  is_last_round={isLastRound}
                  is_loading={isLastRoundLive}
                  on_permission_response={onPermissionResponse}
                  can_respond_to_permissions={canRespondToPermissions}
                  permission_read_only_reason={permissionReadOnlyReason}
                  on_open_agent_contact={onOpenAgentContact}
                  on_stop_message={onStopMessage}
                  on_open_workspace_file={onOpenWorkspaceFile}
                />
              ) : (
                <MessageItem
                  compact={compact}
                  current_agent_name={resolveRoundAgentName(roundMessages, agentNameMap) ?? currentAgentName}
                  current_agent_avatar={resolveRoundAgentAvatar(roundMessages, agentAvatarMap) ?? currentAgentAvatar}
                  workspace_agent_id={resolveRoundAgentId(roundMessages)}
                  current_user_avatar={currentUserAvatar}
                  round_id={roundId}
                  messages={roundMessages}
                  is_last_round={isLastRound}
                  is_loading={isLastRoundLive}
                  runtime_phase={isLastRoundLive ? runtimePhase : null}
                  pending_permissions={isLastRoundLive ? isLastRoundPendingPermissions : []}
                  on_permission_response={onPermissionResponse}
                  can_respond_to_permissions={canRespondToPermissions}
                  permission_read_only_reason={permissionReadOnlyReason}
                  on_open_agent_contact={onOpenAgentContact}
                  on_open_workspace_file={onOpenWorkspaceFile}
                  on_stop_message={onStopMessage}
                />
              )}
            </div>
          );
        })}
      </div>
      <div ref={bottomAnchorRef} className="absolute bottom-0 h-px w-full" />
    </div>
  );
}
