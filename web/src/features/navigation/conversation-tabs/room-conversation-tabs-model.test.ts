// INPUT: 服务端明确 draft 标记、外部 Session 事实及创建/活动时间。
// OUTPUT: 验证页面新建资格只来自当前内部 draft，排序不受消息活动影响。
// POS: 标签领域纯模型回归，与共享几何模型分开验证。

import { describe, expect, it } from "vitest";

import type { RoomConversationView } from "@/types/conversation/conversation";
import { getConversationIdsByCreationTime, resolveSelectedDraftConversationId } from "./room-conversation-tabs-model";

function conversation(id: string, patch: Partial<RoomConversationView> = {}): RoomConversationView {
  return {
    conversation_id: id, room_id: "room", session_key: id, session_id: null,
    created_at: 1, last_activity_at: 1, options: {}, title: id, ...patch,
  };
}

describe("Room conversation tab rules", () => {
  it("reuses only the explicitly selected internal draft", () => {
    const conversations = [
      conversation("draft", { is_draft: true }),
      conversation("empty", { message_count: 0 }),
      conversation("external", { is_draft: true, options: { external_session: true } }),
    ];
    expect(resolveSelectedDraftConversationId(conversations, "draft")).toBe("draft");
    for (const selected of ["empty", "external", "missing", null]) {
      expect(resolveSelectedDraftConversationId(conversations, selected)).toBeNull();
    }
  });

  it("orders by creation time and identity even when recent activity runs in reverse", () => {
    expect(getConversationIdsByCreationTime([
      conversation("latest", { created_at: 2 }),
      conversation("b", { last_activity_at: 99 }),
      conversation("a", { last_activity_at: 100 }),
    ])).toEqual(["a", "b", "latest"]);
  });
});
