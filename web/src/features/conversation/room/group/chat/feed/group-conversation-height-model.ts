/**
 * INPUT: 共享消息估高与 Room pending slot 投影。
 * OUTPUT: 把尚未写入消息的 Agent 外壳计入虚拟 feed 初始高度。
 * POS: Room 虚拟列表专属的运行态估高修正，不参与滚动策略。
 */
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type { Message } from "@/types/conversation/message/entity";

interface ProjectGroupRoundHeightOptions {
  baseHeights: ReadonlyMap<string, number>;
  messageGroups: ReadonlyMap<string, Message[]>;
  pendingSlotGroups: ReadonlyMap<string, RoomPendingAgentSlotState[]>;
  roundIds: readonly string[];
}

const SLOT_ONLY_SHELL_HEIGHT = 112;

export function projectGroupRoundHeights({
  baseHeights,
  messageGroups,
  pendingSlotGroups,
  roundIds,
}: ProjectGroupRoundHeightOptions): Map<string, number> {
  const result = new Map(baseHeights);
  for (const roundId of roundIds) {
    const messages = messageGroups.get(roundId) ?? [];
    const slots = pendingSlotGroups.get(roundId) ?? [];
    const baseHeight = baseHeights.get(roundId) ?? SLOT_ONLY_SHELL_HEIGHT;
    const slotShellHeight = hasAssistantMessage(messages)
      ? 0
      : slots.length * SLOT_ONLY_SHELL_HEIGHT;
    result.set(
      roundId,
      Math.max(baseHeight, slotShellHeight),
    );
  }
  return result;
}

function hasAssistantMessage(messages: readonly Message[]): boolean {
  return messages.some((message) => message.role === "assistant");
}
