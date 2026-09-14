// INPUT: 已绑定测试 owner 的 Room 标签与固定偏好、可选固定权限。
// OUTPUT: 验证领域适配器向共享视图提供真实标题、固定状态和独立命令。
// POS: DM、Group 与 Contacts 共用标签入口的集成回归。

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { setRoomNavigationOwnerScope, useRoomNavigationStore } from "@/store/room-navigation";
import type { RoomConversationView } from "@/types/conversation/conversation";
import { RoomConversationTabs } from "./room-conversation-tabs";

// jsdom has no layout/scroll or pointer capture; the browser suite owns those.
const browserMethods = ["scrollTo", "hasPointerCapture"] as const;
const descriptors = browserMethods.map((method) => Object.getOwnPropertyDescriptor(HTMLElement.prototype, method));
beforeAll(() => {
  Object.defineProperty(HTMLElement.prototype, "scrollTo", { configurable: true, value: vi.fn() });
  Object.defineProperty(HTMLElement.prototype, "hasPointerCapture", { configurable: true, value: () => false });
});
afterAll(() => browserMethods.forEach((method, index) => {
  const descriptor = descriptors[index];
  if (descriptor) Object.defineProperty(HTMLElement.prototype, method, descriptor);
  else Reflect.deleteProperty(HTMLElement.prototype, method);
}));

beforeEach(() => {
  setRoomNavigationOwnerScope(null, () => false);
  setRoomNavigationOwnerScope("user-id:room-tabs-test", () => true);
});

describe("RoomConversationTabs", () => {
  it("keeps browser-style tabs through creation, history selection, closing and remount", async () => {
    const user = userEvent.setup();
    const makeConversation = (id: string, created: number): RoomConversationView => ({
      conversation_id: id, room_id: "room", session_key: `room/${id}`, session_id: null,
      title: id, created_at: created, last_activity_at: created, options: {},
    });
    const first = makeConversation("First session", 1);
    const history = makeConversation("History session", 0);
    let currentItems = [first, history];
    function Harness() {
      const [items, setItems] = useState(currentItems);
      const [active, setActive] = useState("First session");
      return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
        <RoomConversationTabs conversations={items} conversationId={active} onSelectConversation={setActive}
          leadingControl={<button onClick={() => setActive(history.conversation_id)}>Open history</button>}
          onCreateConversation={async () => {
            const next = makeConversation("New session", 2);
            currentItems = [...items, next];
            setItems(currentItems);
            setActive(next.conversation_id);
            return next.conversation_id;
          }} />
      </I18N_CONTEXT.Provider>;
    }
    const view = render(<Harness />);
    expect(screen.queryByRole("button", { name: history.title })).toBeNull();
    await user.click(screen.getByRole("button", { name: "room.new_conversation" }));
    expect(screen.getByRole("button", { name: first.title })).toBeTruthy();
    expect(screen.getByRole("button", { name: "New session" }).getAttribute("aria-current")).toBe("page");
    await user.click(screen.getByRole("button", { name: "Open history" }));
    expect(screen.getByRole("button", { name: history.title }).getAttribute("aria-current")).toBe("page");
    expect(screen.getByRole("button", { name: first.title })).toBeTruthy();
    expect(screen.getByRole("button", { name: "New session" })).toBeTruthy();
    view.unmount();
    render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
      <RoomConversationTabs conversations={currentItems} conversationId={history.conversation_id} onSelectConversation={vi.fn()} />
    </I18N_CONTEXT.Provider>);
    const oldTab = screen.getByRole("button", { name: first.title }).parentElement!;
    await user.click(within(oldTab).getByRole("button", { name: "room.close_conversation" }));
    expect(screen.queryByRole("button", { name: first.title })).toBeNull();
    expect(screen.getByRole("button", { name: history.title })).toBeTruthy();
    expect(screen.getByRole("button", { name: "New session" })).toBeTruthy();
  });

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
