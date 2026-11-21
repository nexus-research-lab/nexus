import { describe, expect, it } from "vitest";

import {
  resolveGroupConversationRound,
  type GroupConversationRoundSource,
} from "./group-conversation-feed-model";

describe("last round of a resident Room window", () => {
  const source = (historyGapAfterRoundId: string | null): GroupConversationRoundSource => ({
    historyGapAfterRoundId,
    liveLayoutActive: false,
    liveRoundIds: [],
    messageGroups: new Map(),
    pendingPermissionGroups: new Map(),
    pendingSlotGroups: new Map(),
    roomAgentExecutionStateGroups: new Map(),
    // Feed 节点是 Agent 轮次，断点记录的是它所属的根轮次。
    rootRoundIds: new Map([["round-2:agent-1", "round-2"]]),
    roundIds: ["round-1", "round-2:agent-1"],
    scopeKey: "room:test",
  });

  it("is the conversation's last round only when the window reaches the latest content", () => {
    expect(resolveGroupConversationRound(source(null), 1).isLast).toBe(true);
    expect(resolveGroupConversationRound(source("round-2"), 1).isLast).toBe(false);
    expect(resolveGroupConversationRound(source("round-1"), 1).isLast).toBe(true);
  });
});
