// INPUT: 最后标签的 draft/外部事实、可控 runtime 关闭和提交作用域。
// OUTPUT: 验证新建、关闭、提交的顺序及失败/过期时的提交边界。
// POS: 标签 Feature 最终替换事务回归，不触发真实 runtime 或路由。

import { describe, expect, it, vi } from "vitest";

import type { RoomConversationView } from "@/types/conversation/conversation";
import { replaceFinalConversation } from "./final-conversation-replacement";

const conversation: RoomConversationView = {
  conversation_id: "old", room_id: "room", session_key: "room/old", session_id: null,
  title: "Old", created_at: 1, last_activity_at: 1, options: {},
};

function transaction(overrides: Partial<RoomConversationView> = {}) {
  const order: string[] = [];
  return {
    order,
    options: {
      conversation: { ...conversation, ...overrides },
      closeConversation: vi.fn(async () => { order.push("close"); }),
      createConversation: vi.fn(async () => { order.push("create"); return "new"; }),
      commitConversation: vi.fn(() => { order.push("commit"); }),
      isCurrent: () => true,
    },
  };
}

describe("replaceFinalConversation", () => {
  it("commits a new draft before stopping a started conversation", async () => {
    const { order, options } = transaction();
    await replaceFinalConversation(options);
    expect(order).toEqual(["create", "commit", "close"]);
    expect(options.commitConversation).toHaveBeenCalledWith("new");
  });

  it("stops a draft before ensuring and committing its potentially reused identity", async () => {
    const { order, options } = transaction({ is_draft: true });
    options.createConversation.mockImplementation(async () => { order.push("create"); return "old"; });
    await replaceFinalConversation(options);
    expect(order).toEqual(["close", "create", "commit"]);
    expect(options.commitConversation).toHaveBeenCalledWith("old");
  });

  it("does not close an external runtime", async () => {
    const { order, options } = transaction({ is_draft: true, options: { external_session: true } });
    await replaceFinalConversation(options);
    expect(order).toEqual(["create", "commit"]);
    expect(options.closeConversation).not.toHaveBeenCalled();
  });

  it("retains the old conversation when stopping its reusable draft fails", async () => {
    const { options } = transaction({ is_draft: true });
    options.closeConversation.mockRejectedValue(new Error("close failed"));
    await replaceFinalConversation(options);
    expect(options.createConversation).not.toHaveBeenCalled();
    expect(options.commitConversation).not.toHaveBeenCalled();
  });

  it("does not commit or close a started conversation after newer navigation", async () => {
    const { options } = transaction();
    let current = true;
    options.isCurrent = () => current;
    options.createConversation.mockImplementation(async () => {
      current = false;
      return "new";
    });
    await replaceFinalConversation(options);
    expect(options.commitConversation).not.toHaveBeenCalled();
    expect(options.closeConversation).not.toHaveBeenCalled();
  });
});
