// INPUT: DM/Room 上下文快照与 hover、focus、Escape 和清空事件。
// OUTPUT: 证明唯一详情、焦点和逐 Agent 快照保持，Room 高度只由共享边界和视口约束。
// POS: 上下文指标 DOM 回归；实际碰撞在 Gallery 中人工查看。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
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

  it("breaks the window down by category when the runtime reports one", () => {
    const usage = {
      ...USAGE,
      categories: [
        { name: "System prompt", tokens: 6_800 },
        { name: "Messages", tokens: 10_100 },
        { name: "Autocompact buffer", tokens: 33_000 },
        { name: "Free space", tokens: 950_100 },
        { name: "Deferred MCP tools", tokens: 19_200, deferred: true },
      ],
      model: "claude-opus-5-5",
    };
    render(<I18N_CONTEXT.Provider value={LOCALIZATION}><ComposerContextUsage usage={usage} /></I18N_CONTEXT.Provider>);
    fireEvent.mouseEnter(screen.getByRole("button", { name: "composer.context_usage_label" }));
    const detail = screen.getByRole("tooltip");
    expect(within(detail).getByText("claude-opus-5-5")).toBeTruthy();
    expect(within(detail).getByText("composer.context_category_system_prompt")).toBeTruthy();
    expect(within(detail).getByText("6.8K")).toBeTruthy();
    expect(within(detail).getByText("0.7%")).toBeTruthy();
    const deferred = within(detail).getByRole("list", { name: "composer.context_category_deferred_group" });
    expect(within(deferred).getByText("composer.context_category_deferred")).toBeTruthy();
    expect(within(deferred).getByText("—")).toBeTruthy();
    // 空闲空间只是轨道底色，不画成占用切片。
    expect(detail.querySelector('[data-context-usage-segment="free_space"]')).toBeNull();
    expect(detail.querySelector('[data-context-usage-segment="messages"]')).toBeTruthy();
  });
});
