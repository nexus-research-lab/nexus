import { useMemo } from "react";

import type {
  Message,
  RoomPendingAgentSlotState,
} from "@/types/conversation/message";
import type { PendingPermission } from "@/types/conversation/permission";
import type { SessionRoundIndexItem } from "@/types/conversation/room";
import type { AgentConversationChatType } from "@/types/agent/agent-conversation";

import {
  groupMessagesByRound,
  groupRoomPendingPermissionsByRound,
  groupRoomPendingSlotsByRound,
} from "./utils";
import {
  buildIndexedTimelineRoundIds,
  buildTimelineRoundIds,
} from "./timeline-rounds";

/**
 * 前端唯一的对话时间线投影。
 * DM / Room 共用：所有按 round 的分组、排序、占位推导只在这里发生，
 * feed / navigator / thread 都消费这一份投影，不再各自持有分组真相。
 */
export interface ConversationTimeline {
  /** root round -> 该轮全部消息 */
  message_groups: Map<string, Message[]>;
  /** root round -> Room 占位槽位 */
  pending_slot_groups: Map<string, RoomPendingAgentSlotState[]>;
  /** root round -> 待确认权限 */
  pending_permission_groups: Map<string, PendingPermission[]>;
  /** 已加载轮次（含 live 占位）的展示顺序 */
  loaded_round_ids: string[];
  /** 叠加导航索引后的完整时间线轮次（含未加载占位） */
  feed_round_ids: string[];
  /** turn 导航索引 */
  round_index_items: SessionRoundIndexItem[];
  live_round_ids: string[];
}

export interface UseConversationTimelineOptions {
  chat_type: AgentConversationChatType;
  messages: Message[];
  live_round_ids: string[];
  round_index_items: SessionRoundIndexItem[];
  pending_agent_slots?: RoomPendingAgentSlotState[];
  pending_permissions?: PendingPermission[];
}

const EMPTY_SLOTS: RoomPendingAgentSlotState[] = [];
const EMPTY_PERMISSIONS: PendingPermission[] = [];

export function useConversationTimeline({
  chat_type: chatType,
  messages,
  live_round_ids: liveRoundIds,
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
    () => (isRoom ? groupRoomPendingSlotsByRound(pendingAgentSlots) : new Map<string, RoomPendingAgentSlotState[]>()),
    [isRoom, pendingAgentSlots],
  );
  const pendingPermissionGroups = useMemo(
    () => (isRoom ? groupRoomPendingPermissionsByRound(pendingPermissions) : new Map<string, PendingPermission[]>()),
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
  const feedRoundIds = useMemo(
    () => buildIndexedTimelineRoundIds(roundIndexItems, loadedRoundIds),
    [loadedRoundIds, roundIndexItems],
  );

  return useMemo(
    () => ({
      message_groups: messageGroups,
      pending_slot_groups: pendingSlotGroups,
      pending_permission_groups: pendingPermissionGroups,
      loaded_round_ids: loadedRoundIds,
      feed_round_ids: feedRoundIds,
      round_index_items: roundIndexItems,
      live_round_ids: liveRoundIds,
    }),
    [
      feedRoundIds,
      liveRoundIds,
      loadedRoundIds,
      messageGroups,
      pendingPermissionGroups,
      pendingSlotGroups,
      roundIndexItems,
    ],
  );
}
