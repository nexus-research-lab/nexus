/**
 * INPUT: 当前已加载消息、slot/permission/execution 运行态、原始 round 索引与已解析历史窗口。
 * OUTPUT: feed、navigator 共用的记忆化 ConversationTimeline。
 * POS: React 装配层；轮次过滤与排序规则留在 timeline-model。
 */
import { useMemo } from "react";

import type { Message } from "@/types/conversation/message/entity";
import type {
  RoomAgentExecutionState,
  RoomPendingAgentSlotState,
} from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import type { SessionRoundIndexItem } from "@/types/conversation/history";
import type { AgentConversationChatType } from "@/types/agent/agent-conversation";

import {
  buildIndexedTimelineRoundIds,
  buildTimelineRoundIds,
  filterVisibleRoomLiveRoundIds,
  filterVisibleTimelineMessages,
  filterResolvedEmptyRoundIndexItems,
  filterSupersededRoundIndexItems,
  groupMessagesByRound,
  groupPendingPermissionsByRound,
  groupPendingSlotsByRound,
  groupRoomAgentExecutionStatesByRound,
  mergeLoadedRoundIndexItems,
  projectVisibleRoomTimelineEvidence,
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
  room_agent_execution_states?: RoomAgentExecutionState[];
}

const EMPTY_SLOTS: RoomPendingAgentSlotState[] = [];
const EMPTY_PERMISSIONS: PendingPermission[] = [];
const EMPTY_EXECUTION_STATES: RoomAgentExecutionState[] = [];
const EMPTY_ROUND_IDS: string[] = [];

export function useConversationTimeline({
  chat_type: chatType,
  messages,
  live_round_ids: liveRoundIds,
  resolved_history_round_ids: resolvedHistoryRoundIds = EMPTY_ROUND_IDS,
  round_index_items: roundIndexItems,
  pending_agent_slots: pendingAgentSlots = EMPTY_SLOTS,
  pending_permissions: pendingPermissions = EMPTY_PERMISSIONS,
  room_agent_execution_states: roomAgentExecutionStates = EMPTY_EXECUTION_STATES,
}: UseConversationTimelineOptions): ConversationTimeline {
  const isRoom = chatType === "group";

  const visibleMessages = useMemo(
    () => filterVisibleTimelineMessages(messages),
    [messages],
  );
  const visibleRoomEvidence = useMemo(
    () => isRoom
      ? projectVisibleRoomTimelineEvidence(
          pendingAgentSlots,
          pendingPermissions,
          roomAgentExecutionStates,
        )
      : {
          executionStates: EMPTY_EXECUTION_STATES,
          pendingPermissions: EMPTY_PERMISSIONS,
          pendingSlots: EMPTY_SLOTS,
        },
    [
      isRoom,
      pendingAgentSlots,
      pendingPermissions,
      roomAgentExecutionStates,
    ],
  );

  const messageGroups = useMemo(
    () => groupMessagesByRound(visibleMessages),
    [visibleMessages],
  );
  const pendingSlotGroups = useMemo(
    () =>
      isRoom
        ? groupPendingSlotsByRound(visibleRoomEvidence.pendingSlots)
        : new Map<string, RoomPendingAgentSlotState[]>(),
    [isRoom, visibleRoomEvidence.pendingSlots],
  );
  const pendingPermissionGroups = useMemo(
    () =>
      isRoom
        ? groupPendingPermissionsByRound(
            visibleRoomEvidence.pendingPermissions,
          )
        : new Map<string, PendingPermission[]>(),
    [isRoom, visibleRoomEvidence.pendingPermissions],
  );
  const roomAgentExecutionStateGroups = useMemo(
    () =>
      isRoom
        ? groupRoomAgentExecutionStatesByRound(
            visibleRoomEvidence.executionStates,
          )
        : new Map<string, RoomAgentExecutionState[]>(),
    [isRoom, visibleRoomEvidence.executionStates],
  );
  const visibleLiveRoundIds = useMemo(
    () => isRoom
      ? filterVisibleRoomLiveRoundIds(
          liveRoundIds,
          messageGroups,
          pendingPermissionGroups,
          pendingSlotGroups,
          roomAgentExecutionStateGroups,
        )
      : liveRoundIds,
    [
      isRoom,
      liveRoundIds,
      messageGroups,
      pendingPermissionGroups,
      pendingSlotGroups,
      roomAgentExecutionStateGroups,
    ],
  );
  const loadedRoundIds = useMemo(
    () =>
      buildTimelineRoundIds(messageGroups, visibleLiveRoundIds, [
        ...pendingSlotGroups.keys(),
        ...pendingPermissionGroups.keys(),
        ...roomAgentExecutionStateGroups.keys(),
      ]),
    [
      messageGroups,
      pendingPermissionGroups,
      pendingSlotGroups,
      roomAgentExecutionStateGroups,
      visibleLiveRoundIds,
    ],
  );
  const unsupersededRoundIndexItems = useMemo(
    () => filterSupersededRoundIndexItems(roundIndexItems, visibleMessages),
    [roundIndexItems, visibleMessages],
  );
  const visibleRoundIndexItems = useMemo(
    () => filterResolvedEmptyRoundIndexItems(
      unsupersededRoundIndexItems,
      loadedRoundIds,
      resolvedHistoryRoundIds,
    ),
    [loadedRoundIds, resolvedHistoryRoundIds, unsupersededRoundIndexItems],
  );
  const mergedRoundIndexItems = useMemo(
    () => mergeLoadedRoundIndexItems(visibleRoundIndexItems, loadedRoundIds),
    [loadedRoundIds, visibleRoundIndexItems],
  );
  const feedRoundIds = useMemo(
    () => buildIndexedTimelineRoundIds(mergedRoundIndexItems, loadedRoundIds),
    [loadedRoundIds, mergedRoundIndexItems],
  );

  return useMemo(
    () => ({
      message_groups: messageGroups,
      pending_slot_groups: pendingSlotGroups,
      pending_permission_groups: pendingPermissionGroups,
      room_agent_execution_state_groups: roomAgentExecutionStateGroups,
      loaded_round_ids: loadedRoundIds,
      feed_round_ids: feedRoundIds,
      round_index_items: mergedRoundIndexItems,
      live_round_ids: visibleLiveRoundIds,
    }),
    [
      feedRoundIds,
      loadedRoundIds,
      messageGroups,
      mergedRoundIndexItems,
      pendingPermissionGroups,
      pendingSlotGroups,
      roomAgentExecutionStateGroups,
      visibleLiveRoundIds,
    ],
  );
}
