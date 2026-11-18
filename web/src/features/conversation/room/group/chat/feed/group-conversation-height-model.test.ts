import { expect, it } from "vitest";

import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type { Message } from "@/types/conversation/message/entity";

import { projectGroupRoundHeights } from "./group-conversation-height-model";

const assistant = (roundId: string): Message => ({
  message_id: `m-${roundId}`, session_key: "s", agent_id: "a", round_id: roundId,
  timestamp: 0, role: "assistant", content: [{ type: "text", text: "hi" }],
});
const slot = {} as RoomPendingAgentSlotState;

it("layers pending slot shells over cached base heights", () => {
  const heights = projectGroupRoundHeights({
    baseHeights: new Map([["answered", 300], ["waiting", 80]]),
    messageGroups: new Map([["answered", [assistant("answered")]], ["waiting", []]]),
    pendingSlotGroups: new Map([
      ["answered", [slot, slot, slot, slot]],
      ["waiting", [slot, slot]],
      ["slot-only", [slot]],
    ]),
    roundIds: ["answered", "waiting", "slot-only"],
  });
  // 已有 Assistant 正文的轮次沿用缓存估高，不再叠加 slot 外壳。
  expect(heights.get("answered")).toBe(300);
  // 尚无回复时按待回复 Agent 数量撑开外壳，避免占位比真实外壳矮。
  expect(heights.get("waiting")).toBe(224);
  expect(heights.get("slot-only")).toBe(112);
});
