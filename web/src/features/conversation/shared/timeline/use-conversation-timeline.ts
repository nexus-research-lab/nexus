import { useMemo } from "react";

import type {
  Message,
  RoomPendingAgentSlotState,
} from "@/types/conversation/message";
import type { PendingPermission } from "@/types/conversation/permission";
import type { SessionRoundIndexItem } from "@/types/conversation/room";
import type { AgentConversationChatType } from "@/types/agent/agent-conversation";

import {
  buildIndexedTimelineRoundIds,
  buildTimelineRoundIds,
  groupMessagesByRound,
  groupPendingPermissionsByRound,
  groupPendingSlotsByRound,
} from "./timeline-model";
import type { ConversationTimeline } from "./timeline-model";

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
