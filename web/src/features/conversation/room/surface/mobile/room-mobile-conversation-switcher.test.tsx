// INPUT: Room 会话目录、当前选择与关闭/切换命令。
// OUTPUT: 证明 Switcher 跟随共享页头偏移、语义浮层和紧凑列表合同。
// POS: Room 窄窗会话切换器行为测试；历史过滤规则由 history model 测试负责。

import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import type { RoomConversationView } from "@/types/conversation/conversation";

import { RoomMobileConversationSwitcher } from "./room-mobile-conversation-switcher";

const CONVERSATIONS = [
  {
    conversation_id: "conversation-1",
    created_at: 1,
    last_activity_at: 1_700_000_000_000,
    options: {},
    room_id: "room-1",
    session_id: null,
    session_key: "room-1:conversation-1",
    title: "产品讨论",
  },
  {
    conversation_id: "conversation-2",
    created_at: 2,
    last_activity_at: 1_700_000_100_000,
    options: {},
    room_id: "room-1",
    session_id: null,
    session_key: "room-1:conversation-2",
    title: "交付检查",
  },
] as RoomConversationView[];

describe("RoomMobileConversationSwitcher", () => {

  it.each(["en", "zh"] as const)("uses %s time labels and explains an empty history without exposing drafts", (locale: Locale) => {
    const props = { activeConversationId: null, isOpen: true, onClose: vi.fn(), onSelect: vi.fn() };
    const wrapper = ({ children }: { children: React.ReactNode }) => <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>{children}</I18N_CONTEXT.Provider>;
    const view = render(<RoomMobileConversationSwitcher {...props} conversations={[{ ...CONVERSATIONS[0], last_activity_at: 0 }]} />, { wrapper });
    expect(within(screen.getByRole("dialog")).getByText(locale === "en" ? "Just now" : "刚刚")).toBeTruthy();
    view.rerender(<RoomMobileConversationSwitcher {...props} conversations={[{ ...CONVERSATIONS[0], is_draft: true }]} />);
    expect(screen.queryByRole("button", { name: /产品讨论/ })).toBeNull();
    expect(within(screen.getByRole("dialog")).getByRole("status").textContent).toBe(locale === "en" ? "No conversations yet" : "暂无对话");
    expect(props.onSelect).not.toHaveBeenCalled();
  });
});
