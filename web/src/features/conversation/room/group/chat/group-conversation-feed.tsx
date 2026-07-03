import { memo, useEffect, useMemo, useRef } from "react";
import type { RefObject } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { MessageItem } from "@/features/conversation/shared/message";
import { hasRoomAgentRoundEntries } from "@/features/conversation/shared/utils";
import { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import { Message, RoomPendingAgentSlotState } from "@/types/conversation/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/conversation/permission";
import { estimateRoundHeights } from "@/hooks/conversation/use-message-height";
import { GroupRoundCardGroup } from "../thread/group-round-card-group";

interface GroupConversationFeedProps {
  bottomAnchorRef: React.RefObject<HTMLDivElement | null>;
  feedRef?: RefObject<HTMLDivElement | null>;
  /** The scrollable container — needed by the virtualizer */
  scrollRef?: RefObject<HTMLDivElement | null>;
  compact?: boolean;
  currentAgentName: string | null;
  currentAgentAvatar?: string | null;
  currentUserAvatar?: string | null;
  /** Room 模式下的 agentId → name 映射（用于多 Agent 显示） */
  agentNameMap?: Record<string, string>;
  /** Room 模式下的 agentId → avatar 映射（用于多 Agent 显示） */
  agentAvatarMap?: Record<string, string | null>;
  isLastRoundPendingPermissions: PendingPermission[];
  isLoading: boolean;
  runtimePhase?: AgentConversationRuntimePhase | null;
  liveRoundIds: string[];
  isMobileLayout: boolean;
  messageGroups: Map<string, Message[]>;
  pendingPermissionGroups: Map<string, PendingPermission[]>;
  pendingSlotGroups: Map<string, RoomPendingAgentSlotState[]>;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
  canRespondToPermissions?: boolean;
  permissionReadOnlyReason?: string;
  /** Room 并发模式：停止单条消息生成 */
  onStopMessage?: (msgId: string) => void;
  roundIds: string[];
}

// Minimum rounds before we enable virtualization — below this threshold the
// overhead is not worth it and scroll behaviour is simpler without it.
const VIRTUAL_THRESHOLD = 20;

/** Room 模式下从 round 的 assistant 消息中提取 agentId，查找对应名字 */
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

/** Room 模式下从 round 的 assistant 消息中提取 agentId，查找对应头像 */
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
  bottomAnchorRef: bottomAnchorRef,
  feedRef: feedRef,
  scrollRef: scrollRef,
  compact = false,
  currentAgentName: currentAgentName,
  currentAgentAvatar: currentAgentAvatar,
  currentUserAvatar: currentUserAvatar,
  agentNameMap: agentNameMap,
  agentAvatarMap: agentAvatarMap,
  isLastRoundPendingPermissions: isLastRoundPendingPermissions,
  runtimePhase: runtimePhase,
  liveRoundIds: liveRoundIds,
  isMobileLayout: isMobileLayout,
  messageGroups: messageGroups,
  pendingPermissionGroups: pendingPermissionGroups,
  pendingSlotGroups: pendingSlotGroups,
  onOpenAgentContact: onOpenAgentContact,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  onPermissionResponse: onPermissionResponse,
  canRespondToPermissions: canRespondToPermissions = true,
  permissionReadOnlyReason: permissionReadOnlyReason,
  onStopMessage: onStopMessage,
  roundIds: roundIds,
}: GroupConversationFeedProps) {
  const useVirtual = roundIds.length >= VIRTUAL_THRESHOLD;

  if (useVirtual && scrollRef) {
    return (
      <VirtualFeed
        bottomAnchorRef={bottomAnchorRef}
        feedRef={feedRef}
        scrollRef={scrollRef}
        compact={compact}
        currentAgentName={currentAgentName}
        currentAgentAvatar={currentAgentAvatar}
        currentUserAvatar={currentUserAvatar}
        agentNameMap={agentNameMap}
        agentAvatarMap={agentAvatarMap}
        isLastRoundPendingPermissions={isLastRoundPendingPermissions}
        runtimePhase={runtimePhase}
        liveRoundIds={liveRoundIds}
        isMobileLayout={isMobileLayout}
        messageGroups={messageGroups}
        pendingPermissionGroups={pendingPermissionGroups}
        pendingSlotGroups={pendingSlotGroups}
        onOpenAgentContact={onOpenAgentContact}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        onPermissionResponse={onPermissionResponse}
        canRespondToPermissions={canRespondToPermissions}
        permissionReadOnlyReason={permissionReadOnlyReason}
        onStopMessage={onStopMessage}
        roundIds={roundIds}
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
        const hasRoomEntries = hasRoomAgentRoundEntries(roundMessages, roundPendingSlots);

        // Group Room 中一旦出现 Agent 回复，就统一走 GroupRoundCardGroup。
        if (hasRoomEntries) {
          return (
            <GroupRoundCardGroup
              key={roundId}
              roundId={roundId}
              messages={roundMessages}
              pendingPermissions={roundPendingPermissions}
              pendingSlots={roundPendingSlots}
              agentNameMap={agentNameMap}
              agentAvatarMap={agentAvatarMap}
              currentUserAvatar={currentUserAvatar}
              isLastRound={isLastRound}
              isLoading={isLastRoundLive}
              onPermissionResponse={onPermissionResponse}
              canRespondToPermissions={canRespondToPermissions}
              permissionReadOnlyReason={permissionReadOnlyReason}
              onOpenAgentContact={onOpenAgentContact}
              onStopMessage={onStopMessage}
              onOpenWorkspaceFile={onOpenWorkspaceFile}
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
            currentAgentName={roundAgentName}
            currentAgentAvatar={roundAgentAvatar}
            workspaceAgentId={roundWorkspaceAgentId}
            currentUserAvatar={currentUserAvatar}
            roundId={roundId}
            messages={roundMessages}
            isLastRound={isLastRound}
            isLoading={isLastRoundLive}
            runtimePhase={isLastRoundLive ? runtimePhase : null}
            pendingPermissions={isLastRoundLive ? isLastRoundPendingPermissions : []}
            onPermissionResponse={onPermissionResponse}
            canRespondToPermissions={canRespondToPermissions}
            permissionReadOnlyReason={permissionReadOnlyReason}
            onOpenAgentContact={onOpenAgentContact}
            onOpenWorkspaceFile={onOpenWorkspaceFile}
            onStopMessage={onStopMessage}
          />
        );
      })}
      <div ref={bottomAnchorRef} className="h-px w-full" />
    </div>
  );
});

// ─── VirtualFeed ──────────────────────────────────────────────────────────────

function VirtualFeed({
  bottomAnchorRef: bottomAnchorRef,
  feedRef: feedRef,
  scrollRef: scrollRef,
  compact,
  currentAgentName: currentAgentName,
  currentAgentAvatar: currentAgentAvatar,
  currentUserAvatar: currentUserAvatar,
  agentNameMap: agentNameMap,
  agentAvatarMap: agentAvatarMap,
  isLastRoundPendingPermissions: isLastRoundPendingPermissions,
  runtimePhase: runtimePhase,
  liveRoundIds: liveRoundIds,
  isMobileLayout: isMobileLayout,
  messageGroups: messageGroups,
  pendingPermissionGroups: pendingPermissionGroups,
  pendingSlotGroups: pendingSlotGroups,
  onOpenAgentContact: onOpenAgentContact,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  onPermissionResponse: onPermissionResponse,
  canRespondToPermissions: canRespondToPermissions = true,
  permissionReadOnlyReason: permissionReadOnlyReason,
  onStopMessage: onStopMessage,
  roundIds: roundIds,
}: Omit<GroupConversationFeedProps, "isLoading" | "scrollRef"> & { scrollRef: RefObject<HTMLDivElement | null> }) {
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
    () => estimateRoundHeights(roundIds, messageGroups, containerWidthRef.current),
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
        // Merge feedRef with containerRef
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
          const hasRoomEntries = hasRoomAgentRoundEntries(roundMessages, roundPendingSlots);

          return (
            <div
              key={roundId}
              data-index={virtualItem.index}
              ref={virtualizer.measureElement}
            >
              {hasRoomEntries ? (
                <GroupRoundCardGroup
                  roundId={roundId}
                  messages={roundMessages}
                  pendingPermissions={roundPendingPermissions}
                  pendingSlots={roundPendingSlots}
                  agentNameMap={agentNameMap}
                  agentAvatarMap={agentAvatarMap}
                  currentUserAvatar={currentUserAvatar}
                  isLastRound={isLastRound}
                  isLoading={isLastRoundLive}
                  onPermissionResponse={onPermissionResponse}
                  canRespondToPermissions={canRespondToPermissions}
                  permissionReadOnlyReason={permissionReadOnlyReason}
                  onOpenAgentContact={onOpenAgentContact}
                  onStopMessage={onStopMessage}
                  onOpenWorkspaceFile={onOpenWorkspaceFile}
                />
              ) : (
                <MessageItem
                  compact={compact}
                  currentAgentName={resolveRoundAgentName(roundMessages, agentNameMap) ?? currentAgentName}
                  currentAgentAvatar={resolveRoundAgentAvatar(roundMessages, agentAvatarMap) ?? currentAgentAvatar}
                  workspaceAgentId={resolveRoundAgentId(roundMessages)}
                  currentUserAvatar={currentUserAvatar}
                  roundId={roundId}
                  messages={roundMessages}
                  isLastRound={isLastRound}
                  isLoading={isLastRoundLive}
                  runtimePhase={isLastRoundLive ? runtimePhase : null}
                  pendingPermissions={isLastRoundLive ? isLastRoundPendingPermissions : []}
                  onPermissionResponse={onPermissionResponse}
                  canRespondToPermissions={canRespondToPermissions}
                  permissionReadOnlyReason={permissionReadOnlyReason}
                  onOpenAgentContact={onOpenAgentContact}
                  onOpenWorkspaceFile={onOpenWorkspaceFile}
                  onStopMessage={onStopMessage}
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
