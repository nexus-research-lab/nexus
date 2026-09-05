// INPUT: 受控标签事实、busy 状态及独立选择、创建、关闭和固定命令。
// OUTPUT: 验证共享导航带只投影传入状态并派发准确命令，保留几何与 Spinner。
// POS: 标签共享视图回归；Room 持久化与异步事务由 navigation Feature 测试负责。

import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { WorkspaceConversationTabs } from "./workspace-conversation-tabs";

describe("WorkspaceConversationTabs", () => {
  it("projects the controlled creation state without owning an asynchronous transaction", () => {
    const onCreateConversation = vi.fn();
    const props = {
      activeConversationId: null,
      tabs: [],
      onCloseConversation: vi.fn(),
      onCreateConversation,
      onSelectConversation: vi.fn(),
    };
    const view = render(<I18nProvider><WorkspaceConversationTabs {...props} /></I18nProvider>);
    const createButton = within(screen.getByRole("navigation")).getByRole("button");
    expect(createButton.className).toContain("h-8 w-8");
    expect(createButton.className).toContain("border-transparent");
    expect(createButton.querySelector(".lucide-plus")).toBeTruthy();

    fireEvent.click(createButton);
    expect(onCreateConversation).toHaveBeenCalledOnce();
    expect(createButton.getAttribute("aria-busy")).toBe("false");

    view.rerender(<I18nProvider><WorkspaceConversationTabs {...props} isCreating /></I18nProvider>);
    expect(createButton.getAttribute("aria-busy")).toBe("true");
    expect((createButton as HTMLButtonElement).disabled).toBe(true);
    expect(createButton.querySelector(".lucide-plus")).toBeNull();
    expect(createButton.querySelector(".lucide-loader-circle")?.getAttribute("class"))
      .toContain("motion-reduce:animate-none");
    fireEvent.click(createButton);
    expect(onCreateConversation).toHaveBeenCalledOnce();

    view.rerender(<I18nProvider><WorkspaceConversationTabs {...props} /></I18nProvider>);
    expect((createButton as HTMLButtonElement).disabled).toBe(false);
    expect(createButton.querySelector(".lucide-plus")).toBeTruthy();
  });

  it("keeps selection, closing and pinning controlled and independent", () => {
    const onSelectConversation = vi.fn();
    const onCloseConversation = vi.fn();
    const onTogglePin = vi.fn();
    const props = {
      activeConversationId: "first",
      tabs: [
        { id: "first", title: "First", canClose: false },
        { id: "second", title: "Second", canClose: true, canPin: true, isPinned: false },
      ],
      onSelectConversation,
      onCloseConversation,
      onTogglePin,
    };
    const view = render(<I18nProvider><WorkspaceConversationTabs {...props} /></I18nProvider>);
    const second = screen.getByRole("button", { name: "Second" });
    fireEvent.click(second);
    expect(onSelectConversation).toHaveBeenCalledWith("second");
    expect(second.getAttribute("aria-pressed")).toBe("false");

    const secondTab = second.closest("[data-conversation-tab-id]") as HTMLElement;
    const [, pinButton, closeButton] = within(secondTab).getAllByRole("button");
    fireEvent.click(pinButton);
    fireEvent.click(closeButton);
    expect(onTogglePin).toHaveBeenCalledWith("second");
    expect(onCloseConversation).toHaveBeenCalledWith("second");
    expect(onSelectConversation).toHaveBeenCalledOnce();
    expect(screen.getByRole("button", { name: "Second" })).toBe(second);
    expect(pinButton.getAttribute("aria-pressed")).toBe("false");

    view.rerender(<I18nProvider><WorkspaceConversationTabs {...props} activeConversationId="second" /></I18nProvider>);
    expect(second.getAttribute("aria-pressed")).toBe("true");
    expect(within(screen.getByRole("button", { name: "First" }).parentElement!).getAllByRole("button")).toHaveLength(1);
  });
});
