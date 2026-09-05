// INPUT: 已绑定测试 owner 的 Room 标签与固定偏好、可选固定权限。
// OUTPUT: 验证领域适配器向共享视图提供真实标题、固定状态和独立命令。
// POS: DM、Group 与 Contacts 共用标签入口的集成回归。

import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { setRoomNavigationOwnerScope, useRoomNavigationStore } from "@/store/room-navigation";
import type { RoomConversationView } from "@/types/conversation/conversation";
import { RoomConversationTabs } from "./room-conversation-tabs";

beforeEach(() => {
  setRoomNavigationOwnerScope(null, () => false);
  setRoomNavigationOwnerScope("user-id:room-tabs-test", () => true);
});

describe("RoomConversationTabs", () => {
  it("persists the exact Room pin without selecting and honors disabled pinning", () => {
    const onSelectConversation = vi.fn();
    const conversations: RoomConversationView[] = [{
      conversation_id: "same-id", room_id: "room", session_key: "room/session", session_id: null,
      title: "  Research  ", created_at: 1, last_activity_at: 1, options: {},
    }];
    useRoomNavigationStore.getState().toggle_pinned_conversation({
      room_id: "other-room", conversation_id: "same-id", session_key: "other/session", title: "Other",
    });
    const props = { conversationId: "same-id", conversations, onSelectConversation };
    const view = render(<I18nProvider><RoomConversationTabs {...props} /></I18nProvider>);
    const tab = screen.getByRole("button", { name: "Research" }).parentElement!;
    const pin = within(tab).getAllByRole("button")[1];
    expect(pin.getAttribute("aria-pressed")).toBe("false");
    fireEvent.click(pin);
    expect(pin.getAttribute("aria-pressed")).toBe("true");
    expect(onSelectConversation).not.toHaveBeenCalled();
    expect(useRoomNavigationStore.getState().pinned_conversations).toContainEqual({
      room_id: "room", conversation_id: "same-id", session_key: "room/session", title: "Research",
    });
    fireEvent.click(pin);
    expect(useRoomNavigationStore.getState().pinned_conversations.map((item) => item.room_id)).toEqual(["other-room"]);
    view.rerender(<I18nProvider><RoomConversationTabs {...props} pinningEnabled={false} /></I18nProvider>);
    expect(within(tab).getAllByRole("button")).toHaveLength(1);
  });
});
