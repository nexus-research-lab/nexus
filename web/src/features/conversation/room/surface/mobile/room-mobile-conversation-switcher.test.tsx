// INPUT: Room 会话目录、当前选择与关闭/切换命令。
// OUTPUT: 证明 Switcher 跟随共享页头偏移、语义浮层和紧凑列表合同。
// POS: Room 窄窗会话切换器行为测试；历史过滤规则由 history model 测试负责。

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
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
  it("orders live activity like desktop history without mutating the directory", () => {
    render(<I18nProvider><RoomMobileConversationSwitcher activeConversationId={null} conversations={CONVERSATIONS}
      isOpen onClose={vi.fn()} onSelect={vi.fn()} /></I18nProvider>);
    const rows = within(screen.getByRole("dialog")).getAllByRole("button").filter((button) => button.getAttribute("role") === "button");
    expect(rows.map((row) => row.textContent)).toEqual([expect.stringContaining("交付检查"), expect.stringContaining("产品讨论")]);
    expect(CONVERSATIONS[0].conversation_id).toBe("conversation-1");
  });

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

  it("names each instance from its own heading", () => {
    const props = { activeConversationId: null, conversations: [], isOpen: true, onClose: vi.fn(), onSelect: vi.fn() };
    render(<I18nProvider><RoomMobileConversationSwitcher {...props} /><RoomMobileConversationSwitcher {...props} /></I18nProvider>);
    const dialogs = screen.getAllByRole("dialog");
    const ids = dialogs.map((dialog) => dialog.getAttribute("aria-labelledby"));
    expect(new Set(ids).size).toBe(2);
    dialogs.forEach((dialog, index) => expect(document.getElementById(ids[index]!)).toBe(within(dialog).getByRole("heading")));
  });

  it("uses shared modal focus, scroll lock and keyboard dismissal", async () => {
    const user = userEvent.setup();
    const originalOverflow = document.body.style.overflow;
    const bounds = vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([{ width: 10, height: 10 }] as unknown as DOMRectList);
    function Harness() {
      const [open, setOpen] = useState(false);
      return (
        <I18nProvider>
          <button onClick={() => setOpen(true)} type="button">历史</button>
          <RoomMobileConversationSwitcher
            activeConversationId="conversation-1"
            conversations={CONVERSATIONS}
            isOpen={open}
            onClose={() => setOpen(false)}
            onSelect={vi.fn()}
          />
        </I18nProvider>
      );
    }
    try {
      render(<Harness />);
      const trigger = screen.getByRole("button", { name: "历史" });
      await user.click(trigger);
      const dialog = screen.getByRole("dialog");
      const close = within(dialog).getByRole("button", { name: "Close" });
      await waitFor(() => expect(document.activeElement).toBe(close));
      expect(document.body.style.overflow).toBe("hidden");
      await user.keyboard("{Shift>}{Tab}{/Shift}");
      expect(document.activeElement).toBe(within(dialog).getByRole("button", { name: /产品讨论/ }));
      await user.keyboard("{Tab}");
      expect(document.activeElement).toBe(close);
      await screen.findByRole("tooltip", { name: "Close" });
      await user.keyboard("{Escape}");
      expect(screen.queryByRole("tooltip")).toBeNull();
      expect(screen.getByRole("dialog")).toBe(dialog);
      await user.keyboard("{Escape}");
      expect(screen.queryByRole("dialog")).toBeNull();
      expect(document.activeElement).toBe(trigger);
      expect(document.body.style.overflow).toBe(originalOverflow);
    } finally {
      bounds.mockRestore();
    }
  });

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
