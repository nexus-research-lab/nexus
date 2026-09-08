import { describe, expect, it } from "vitest";

import { AppRouteBuilders } from "./route-paths";

describe("shared navigation contract", () => {
  it("keeps opaque resource identities inside their own path and query segments", () => {
    expect(AppRouteBuilders.roomConversation("room /测试", "conversation?#%"))
      .toBe("/rooms/room%20%2F%E6%B5%8B%E8%AF%95/conversations/conversation%3F%23%25");
    expect(AppRouteBuilders.contactAgent("agent&a=b#c"))
      .toBe("/contacts?agent=agent%26a%3Db%23c");
    expect(AppRouteBuilders.settings("provider/中文"))
      .toBe("/settings?section=provider%2F%E4%B8%AD%E6%96%87");
  });

  it("preserves distinct internal Conversation and external Session destinations", () => {
    expect(AppRouteBuilders.conversation("room-a", "conversation-a"))
      .toBe("/rooms/room-a/conversations/conversation-a");
    expect(AppRouteBuilders.conversation("room-a", "external-session:feishu:account/chat"))
      .toBe("/rooms/room-a/sessions/feishu%3Aaccount%2Fchat");
    expect(AppRouteBuilders.conversation("room-a", "external-session:"))
      .toBe("/rooms/room-a/conversations/external-session%3A");
  });
});
