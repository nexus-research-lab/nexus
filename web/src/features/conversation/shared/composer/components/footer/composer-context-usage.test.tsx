// INPUT: DM/Room 上下文快照与 hover、focus、Escape 和清空事件。
// OUTPUT: 证明只有一份关联详情，延迟 Tooltip 不叠加，Room 保留逐 Agent 快照。
// POS: 上下文指标 DOM 回归；实际碰撞与截图由浏览器矩阵验证。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { ComposerContextUsage } from "./composer-context-usage";

const USAGE = { max_tokens: 1_000_000, percentage: 2, total_tokens: 16_900 };
const LOCALIZATION = { locale: "en" as const, setLocale: () => undefined, t: (key: string) => key };

afterEach(() => vi.useRealTimers());

describe("ComposerContextUsage", () => {
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
});
