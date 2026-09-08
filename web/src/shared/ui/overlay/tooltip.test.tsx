// INPUT: Tooltip 文案、触发器以及用户的焦点、悬停、键盘和指针事件。
// OUTPUT: 证明共享 Tooltip 的延迟、ARIA 关联、关闭行为和语义层级合同。
// POS: Tooltip primitive DOM 行为测试；锚点坐标数学由 overlay model 合同测试负责。

import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { UiTooltip } from "@/shared/ui/overlay/tooltip";

afterEach(() => {
  vi.useRealTimers();
});

describe("UiTooltip", () => {
  it("opens on focus with an accessible relationship and closes with Escape", () => {
    render(
      <UiTooltip label="打开工作图" placement="bottom">
        <button type="button">工作图</button>
      </UiTooltip>,
    );

    const trigger = screen.getByRole("button", { name: "工作图" });
    act(() => trigger.focus());

    const tooltip = screen.getByRole("tooltip", { name: "打开工作图" });
    expect(trigger.getAttribute("aria-describedby")).toBe(tooltip.id);
    expect(tooltip.className).toContain("ui-layer-tooltip");
    expect(tooltip.dataset.uiOverlayOpen).toBe("true");

    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("tooltip")).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });

  it("waits before opening on hover and closes on pointer down", () => {
    vi.useFakeTimers();
    render(
      <UiTooltip label="最近会话">
        <button type="button">Amy</button>
      </UiTooltip>,
    );

    const trigger = screen.getByRole("button", { name: "Amy" });
    fireEvent.mouseEnter(trigger);
    act(() => vi.advanceTimersByTime(259));
    expect(screen.queryByRole("tooltip")).toBeNull();

    act(() => vi.advanceTimersByTime(1));
    expect(screen.getByRole("tooltip", { name: "最近会话" })).toBeTruthy();

    fireEvent.pointerDown(trigger);
    expect(screen.queryByRole("tooltip")).toBeNull();
  });

  it("closes a hovered tooltip with Escape without stealing focus or reopening", () => {
    vi.useFakeTimers();
    render(<>
      <input aria-label="Message" />
      <UiTooltip label="Context"><button type="button">Usage</button></UiTooltip>
    </>);
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
});
