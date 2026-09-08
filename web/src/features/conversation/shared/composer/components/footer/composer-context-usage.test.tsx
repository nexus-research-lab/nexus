// INPUT: DM/Room 上下文快照与 hover、focus、Escape 和清空事件。
// OUTPUT: 证明唯一详情、焦点和逐 Agent 快照保持，Room 高度只由共享边界和视口约束。
// POS: 上下文指标 DOM 回归；实际碰撞与截图由浏览器矩阵验证。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { ComposerContextUsage } from "./composer-context-usage";

const USAGE = { max_tokens: 1_000_000, percentage: 2, total_tokens: 16_900 };
const LOCALIZATION = { locale: "en" as const, setLocale: () => undefined, t: (key: string) => key };

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("ComposerContextUsage", () => {
  it("opens on the first click even when hover and focus precede activation", async () => {
    const user = userEvent.setup();
    render(<I18N_CONTEXT.Provider value={LOCALIZATION}><ComposerContextUsage usage={USAGE} /></I18N_CONTEXT.Provider>);
    const trigger = screen.getByRole("button", { name: "composer.context_usage_label" });
    await user.click(trigger);
    expect(screen.getAllByRole("tooltip")).toHaveLength(1);
    await user.click(trigger);
    expect(screen.getAllByRole("tooltip")).toHaveLength(1);
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("tooltip")).toBeNull();
  });

  it("keeps keyboard-focused detail open when the pointer leaves, until focus leaves", () => {
    vi.useFakeTimers();
    render(<I18N_CONTEXT.Provider value={LOCALIZATION}><ComposerContextUsage usage={USAGE} /></I18N_CONTEXT.Provider>);
    const trigger = screen.getByRole("button", { name: "composer.context_usage_label" });
    act(() => trigger.focus());
    fireEvent.mouseLeave(trigger);
    act(() => vi.advanceTimersByTime(150));
    expect(screen.getAllByRole("tooltip")).toHaveLength(1);
    act(() => trigger.blur());
    act(() => vi.advanceTimersByTime(150));
    expect(screen.queryByRole("tooltip")).toBeNull();
  });

  it("does not reopen an old detail when a usage snapshot becomes available again", () => {
    const view = (usage: typeof USAGE | null) => <I18N_CONTEXT.Provider value={LOCALIZATION}><ComposerContextUsage usage={usage} /></I18N_CONTEXT.Provider>;
    const { rerender } = render(view(USAGE));
    fireEvent.mouseEnter(screen.getByRole("button", { name: "composer.context_usage_label" }));
    expect(screen.getByRole("tooltip")).toBeTruthy();
    rerender(view(null));
    expect(screen.queryByRole("tooltip")).toBeNull();
    rerender(view(USAGE));
    expect(screen.queryByRole("tooltip")).toBeNull();
  });

  it("keeps one hover detail after the generic tooltip delay and preserves keyboard focus", () => {
    vi.useFakeTimers();
    render(<I18N_CONTEXT.Provider value={LOCALIZATION}><ComposerContextUsage usage={USAGE} /></I18N_CONTEXT.Provider>);
    const trigger = screen.getByRole("button", { name: "composer.context_usage_label" });

    fireEvent.mouseEnter(trigger);
    act(() => vi.advanceTimersByTime(500));
    const detail = screen.getByRole("tooltip");
    expect(within(detail).getByText("composer.context_window")).toBeTruthy();
    expect(trigger.getAttribute("aria-describedby")).toBe(detail.id);
    expect(trigger.getAttribute("title")).toBeNull();

    fireEvent.mouseLeave(trigger);
    fireEvent.mouseEnter(detail);
    act(() => vi.advanceTimersByTime(150));
    expect(screen.getAllByRole("tooltip")).toHaveLength(1);
    fireEvent.mouseLeave(detail);
    act(() => vi.advanceTimersByTime(150));
    expect(screen.queryByRole("tooltip")).toBeNull();

    act(() => trigger.focus());
    expect(screen.getAllByRole("tooltip")).toHaveLength(1);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("tooltip")).toBeNull();
    expect(document.activeElement).toBe(trigger);
    expect(trigger.getAttribute("aria-describedby")).toBeNull();
  });

  it("dismisses hover details with Escape without reopening or taking the message input focus", () => {
    vi.useFakeTimers();
    render(<I18N_CONTEXT.Provider value={LOCALIZATION}>
      <input aria-label="Message" />
      <ComposerContextUsage usage={USAGE} />
    </I18N_CONTEXT.Provider>);
    const input = screen.getByRole("textbox");
    act(() => input.focus());
    fireEvent.mouseEnter(screen.getByRole("button"));
    act(() => vi.advanceTimersByTime(500));
    expect(screen.getByRole("tooltip")).toBeTruthy();
    fireEvent.keyDown(document, { key: "Escape" });
    act(() => vi.advanceTimersByTime(500));
    expect(screen.queryByRole("tooltip")).toBeNull();
    expect(document.activeElement).toBe(input);
  });

  it("shows each Room agent in the same detail and removes stale details when snapshots disappear", () => {
    const view = (available: boolean) => <I18N_CONTEXT.Provider value={LOCALIZATION}>
      <ComposerContextUsage items={available ? [
        { agentId: "reader", name: "Reader", usage: USAGE },
        { agentId: "writer", name: "Writer", usage: null },
      ] : []} usage={null} />
    </I18N_CONTEXT.Provider>;
    const { container, rerender } = render(view(true));
    act(() => screen.getByRole("button").focus());
    const detail = screen.getByRole("tooltip");
    expect(within(detail).getByText("Reader")).toBeTruthy();
    expect(within(detail).getByText("Writer")).toBeTruthy();
    expect(within(detail).getByText("composer.context_no_snapshot")).toBeTruthy();
    expect(screen.getByRole("button").getAttribute("data-context-usage")).toBe("2");
    rerender(view(false));
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByRole("tooltip")).toBeNull();
    expect(container.querySelector('[data-context-usage-slot="empty"]')).toBeTruthy();
  });

  it("lets Room content use the available height without a guessed per-row cap and updates on resize", () => {
    const view = (count: number) => <I18N_CONTEXT.Provider value={LOCALIZATION}>
      <ComposerContextUsage items={Array.from({ length: count }, (_, index) => ({
        agentId: `agent-${index}`, name: `Agent ${index}`, usage: USAGE,
      }))} usage={null} />
    </I18N_CONTEXT.Provider>;
    const { rerender } = render(view(3));
    const trigger = screen.getByRole("button");
    const measure = vi.spyOn(trigger, "getBoundingClientRect")
      .mockReturnValue(new DOMRect(240, 360, 28, 28));
    fireEvent.mouseEnter(trigger);
    const detail = screen.getByRole("tooltip");
    const list = within(detail).getByRole("list", { name: "composer.context_window_by_agent" });
    expect(within(list).getAllByRole("listitem")).toHaveLength(3);
    expect(detail.style.maxHeight).toBe("248px");
    expect(detail.style.height).toBe("");

    measure.mockReturnValue(new DOMRect(240, 150, 28, 28));
    fireEvent.resize(window);
    expect(detail.style.maxHeight).toBe("132px");
    rerender(view(12));
    expect(within(list).getAllByRole("listitem")).toHaveLength(12);
    expect(detail.style.maxHeight).toBe("132px");

    measure.mockReturnValue(new DOMRect(240, 360, 28, 28));
    fireEvent.resize(window);
    expect(detail.style.maxHeight).toBe("248px");
    expect(screen.getAllByRole("tooltip")).toHaveLength(1);
    expect(trigger.getAttribute("aria-describedby")).toBe(detail.id);
  });
});
