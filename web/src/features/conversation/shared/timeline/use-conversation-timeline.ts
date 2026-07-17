/**
 * INPUT: 当前已加载消息、运行态、原始 round 索引与已解析历史窗口。
 * OUTPUT: feed、navigator 共用的记忆化 ConversationTimeline。
 * POS: React 装配层；轮次过滤与排序规则留在 timeline-model。
 */
import { useMemo } from "react";

import type { Message } from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import type { SessionRoundIndexItem } from "@/types/conversation/history";
import type { AgentConversationChatType } from "@/types/agent/agent-conversation";

import {
  buildIndexedTimelineRoundIds,
  buildTimelineRoundIds,
  filterResolvedEmptyRoundIndexItems,
  filterSupersededRoundIndexItems,
  groupMessagesByRound,
  groupPendingPermissionsByRound,
  groupPendingSlotsByRound,
} from "./timeline-model";
import type { ConversationTimeline } from "./timeline-model";

export interface UseConversationTimelineOptions {
  chat_type: AgentConversationChatType;
  messages: Message[];
  live_round_ids: string[];
  resolved_history_round_ids?: string[];
  round_index_items: SessionRoundIndexItem[];
  pending_agent_slots?: RoomPendingAgentSlotState[];
  pending_permissions?: PendingPermission[];
}

const EMPTY_SLOTS: RoomPendingAgentSlotState[] = [];
const EMPTY_PERMISSIONS: PendingPermission[] = [];
const EMPTY_ROUND_IDS: string[] = [];

export function useConversationTimeline({
  chat_type: chatType,
  messages,
  live_round_ids: liveRoundIds,
  resolved_history_round_ids: resolvedHistoryRoundIds = EMPTY_ROUND_IDS,
  round_index_items: roundIndexItems,
  pending_agent_slots: pendingAgentSlots = EMPTY_SLOTS,
  pending_permissions: pendingPermissions = EMPTY_PERMISSIONS,
}: UseConversationTimelineOptions): ConversationTimeline {
  const isRoom = chatType === "group";

  const messageGroups = useMemo(
    () => groupMessagesByRound(messages),
    [messages],
  );
  const pendingSlotGroups = useMemo(
    () =>
      isRoom
        ? groupPendingSlotsByRound(pendingAgentSlots)
        : new Map<string, RoomPendingAgentSlotState[]>(),
    [isRoom, pendingAgentSlots],
  );
  const pendingPermissionGroups = useMemo(
    () =>
      isRoom
        ? groupPendingPermissionsByRound(pendingPermissions)
        : new Map<string, PendingPermission[]>(),
    [isRoom, pendingPermissions],
  );
  const loadedRoundIds = useMemo(
    () =>
      buildTimelineRoundIds(messageGroups, liveRoundIds, [
        ...pendingSlotGroups.keys(),
        ...pendingPermissionGroups.keys(),
      ]),
    [liveRoundIds, messageGroups, pendingPermissionGroups, pendingSlotGroups],
  );
  const unsupersededRoundIndexItems = useMemo(
    () => filterSupersededRoundIndexItems(roundIndexItems, messages),
    [messages, roundIndexItems],
  );
  const visibleRoundIndexItems = useMemo(
    () => filterResolvedEmptyRoundIndexItems(
      unsupersededRoundIndexItems,
      loadedRoundIds,
      resolvedHistoryRoundIds,
    ),
    [loadedRoundIds, resolvedHistoryRoundIds, unsupersededRoundIndexItems],
  );
  const feedRoundIds = useMemo(
    () => buildIndexedTimelineRoundIds(visibleRoundIndexItems, loadedRoundIds),
    [loadedRoundIds, visibleRoundIndexItems],
  );

  return useMemo(
    () => ({
      message_groups: messageGroups,
      pending_slot_groups: pendingSlotGroups,
      pending_permission_groups: pendingPermissionGroups,
      loaded_round_ids: loadedRoundIds,
      feed_round_ids: feedRoundIds,
      round_index_items: visibleRoundIndexItems,
      live_round_ids: liveRoundIds,
    }),
    [
      feedRoundIds,
      liveRoundIds,
      loadedRoundIds,
      messageGroups,
      pendingPermissionGroups,
      pendingSlotGroups,
      visibleRoundIndexItems,
    ],
  );
}
