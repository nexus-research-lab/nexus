import { describe, expect, it } from "vitest";

import type { Message } from "@/types/conversation/message/entity";

import {
  resolveConversationRound,
  type ConversationRoundSource,
} from "./conversation-feed-model";

describe("last round of a resident window", () => {
  const source = (historyGapAfterRoundId: string | null): ConversationRoundSource => ({
    historyGapAfterRoundId,
    liveLayoutActive: false,
    liveRoundIds: [],
    messageGroups: new Map([
      ["round-1", [{ message_id: "user-1", role: "user", round_id: "round-1" } as Message]],
      ["round-2", [{ message_id: "user-2", role: "user", round_id: "round-2" } as Message]],
    ]),
    pendingPermissions: [],
    roundIds: ["round-1", "round-2"],
    scopeKey: "agent:amy:ws:dm:test",
  });

  it("is the conversation's last round only when the window reaches the latest content", () => {
    expect(resolveConversationRound(source(null), 1).isLast).toBe(true);
    expect(resolveConversationRound(source(null), 0).isLast).toBe(false);
    // 窗口止于断点：其后还有未驻留的轮次，不能把它当作可编辑/重跑的最后一轮。
    expect(resolveConversationRound(source("round-2"), 1).isLast).toBe(false);
    // 断点之后驻留的实时尾部仍是最后一轮。
    expect(resolveConversationRound(source("round-1"), 1).isLast).toBe(true);
  });
});
