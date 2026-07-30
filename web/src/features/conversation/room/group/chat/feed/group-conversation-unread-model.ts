/**
 * INPUT: Room Agent 稳定节点、全局未读消息锚点与当前 viewport 中已挂载轮次。
 * OUTPUT: 按 room sequence 排序的精确未读消息/Agent 节点映射，以及目标相对视口的方向。
 * POS: Room 未读导航纯模型；不拥有 React 状态、通知计数或滚动写入。
 */
import {
  CONVERSATION_ROUND_SELECTOR,
  findConversationRoundElement,
} from "@/features/conversation/shared/timeline/scroll/round-scroll";
import type {
  ChatUnreadAnchorState,
} from "@/store/sidebar";

import type {
  GroupAgentTimelineProjection,
} from "./group-agent-timeline-model";

export type GroupUnreadNodePosition = "above" | "below" | "visible";

export interface GroupUnreadMessageTarget {
  messageId: string;
  nodeId: string | null;
  rootRoundId: string | null;
  roomSeq: number | null;
  targetKey: string;
  timestamp: number;
}

const VIEWPORT_EDGE_INSET_PX = 8;

export function resolveStoredUnreadMessages(
  source: GroupAgentTimelineProjection,
  anchors: readonly ChatUnreadAnchorState[],
): GroupUnreadMessageTarget[] {
  const nodeByAgentRoundId = new Map<
    string,
    { nodeId: string; rootRoundId: string }
  >();
  const nodeByMessageId = new Map<
    string,
    { nodeId: string; rootRoundId: string }
  >();
  for (const nodeId of source.roundIds) {
    const rootRoundId = source.rootRoundIds.get(nodeId) ?? nodeId;
    for (const message of source.messageGroups.get(nodeId) ?? []) {
      if (message.role !== "assistant") {
        continue;
      }
      const target = {
        nodeId,
        rootRoundId,
      };
      nodeByMessageId.set(message.message_id, target);
      const agentRoundId = message.agent_round_id?.trim();
      if (agentRoundId) {
        nodeByAgentRoundId.set(agentRoundId, target);
      }
      const resultMessageId = message.result_summary?.message_id?.trim();
      if (resultMessageId) {
        nodeByMessageId.set(resultMessageId, target);
        nodeByMessageId.set(`assistant_${resultMessageId}`, target);
      }
    }
  }

  return anchors
    .flatMap((anchor) => anchor.messages.map((message) => {
      const node = nodeByMessageId.get(message.message_id)
        ?? nodeByAgentRoundId.get(message.agent_round_id?.trim() ?? "");
      return {
        messageId: message.message_id,
        nodeId: node?.nodeId ?? null,
        rootRoundId: node?.rootRoundId ?? normalizeId(message.round_id),
        roomSeq: Number.isFinite(message.room_seq)
          ? Number(message.room_seq)
          : null,
        targetKey: anchor.key,
        timestamp: message.timestamp,
      };
    }))
    .sort(compareStoredUnreadMessages);
}

export function countUnreadAgentNodes(
  messages: readonly GroupUnreadMessageTarget[],
): number {
  const identities = new Set<string>();
  for (const message of messages) {
    identities.add(message.nodeId ?? `message:${message.messageId}`);
  }
  return identities.size;
}

export function resolveGroupUnreadNodePosition(
  scrollElement: HTMLDivElement,
  roundIds: readonly string[],
  nodeId: string,
): GroupUnreadNodePosition {
  const target = findConversationRoundElement(scrollElement, nodeId);
  if (target) {
    const viewportRect = scrollElement.getBoundingClientRect();
    const targetRect = target.getBoundingClientRect();
    if (targetRect.bottom <= viewportRect.top + VIEWPORT_EDGE_INSET_PX) {
      return "above";
    }
    if (targetRect.top >= viewportRect.bottom - VIEWPORT_EDGE_INSET_PX) {
      return "below";
    }
    return "visible";
  }

  const targetIndex = roundIds.indexOf(nodeId);
  const mountedIndexes = Array.from(
    scrollElement.querySelectorAll<HTMLElement>(CONVERSATION_ROUND_SELECTOR),
  ).flatMap((element) => {
    const index = Number(element.dataset.conversationRoundIndex);
    return Number.isInteger(index) ? [index] : [];
  });
  if (targetIndex < 0 || mountedIndexes.length === 0) {
    return "below";
  }
  if (targetIndex < Math.min(...mountedIndexes)) {
    return "above";
  }
  return "below";
}

function normalizeId(value: string | null | undefined): string | null {
  const normalized = value?.trim();
  return normalized || null;
}

function compareStoredUnreadMessages(
  left: GroupUnreadMessageTarget,
  right: GroupUnreadMessageTarget,
): number {
  return (left.roomSeq ?? Number.POSITIVE_INFINITY)
    - (right.roomSeq ?? Number.POSITIVE_INFINITY)
    || left.timestamp - right.timestamp
    || left.messageId.localeCompare(right.messageId);
}
