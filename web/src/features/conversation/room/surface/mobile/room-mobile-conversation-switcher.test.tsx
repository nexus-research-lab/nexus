// INPUT: Room 会话目录、当前选择与关闭/切换命令。
// OUTPUT: 证明 Switcher 跟随共享页头偏移、语义浮层和紧凑列表合同。
// POS: Room 窄窗会话切换器行为测试；历史过滤规则由 history model 测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
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
  it("uses shared geometry and selects one compact conversation row", () => {
    const onClose = vi.fn();
    const onSelect = vi.fn();
    const { container } = render(
      <I18nProvider>
        <RoomMobileConversationSwitcher
          activeConversationId="conversation-1"
          conversations={CONVERSATIONS}
          isOpen
          onClose={onClose}
          onSelect={onSelect}
        />
      </I18nProvider>,
    );

    const dialog = screen.getByRole("dialog");
    const activeRow = screen.getByRole("button", { name: /产品讨论/ });
    const nextRow = screen.getByRole("button", { name: /交付检查/ });
    const underlay = container.querySelector(".ui-layer-dialog-underlay");

    expect(dialog.className).toContain("top-[var(--mobile-shell-header-height,52px)]");
    expect(dialog.className).toContain("ui-layer-dialog");
    expect(underlay?.className).toContain("top-[var(--mobile-shell-header-height,52px)]");
    expect(activeRow.className).toContain("min-h-12");
    expect(activeRow.getAttribute("aria-current")).toBe("page");

    fireEvent.click(nextRow);
    expect(onSelect).toHaveBeenCalledWith("conversation-2");
    expect(onClose).toHaveBeenCalledOnce();
  });
});
