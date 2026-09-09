// INPUT: 原生滚轮事件与明确的视口尺寸。
// OUTPUT: 验证滚轮消费边界与缩放手势保留。
// POS: 共享标签滚动 Hook 的 DOM 行为回归，不声称浏览器几何验收。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { useConversationTabsScroll } from "./use-conversation-tabs-scroll";

function Harness() {
  const { viewportRef } = useConversationTabsScroll({
    activeConversationId: null,
    contentKey: "tabs",
  });
  return <div ref={viewportRef} data-testid="viewport"><div /></div>;
}

function setup(scrollWidth = 800) {
  render(<Harness />);
  const viewport = screen.getByTestId("viewport");
  Object.defineProperties(viewport, {
    clientWidth: { value: 200 },
    scrollWidth: { value: scrollWidth },
  });
  return viewport;
}

function wheel(viewport: HTMLElement, init: WheelEventInit) {
  const event = new WheelEvent("wheel", { bubbles: true, cancelable: true, ...init });
  fireEvent(viewport, event);
  return event;
}

describe("conversation tabs wheel handling", () => {
  it("preserves zoom gestures and events already consumed by a child", () => {
    const viewport = setup();
    for (const modifier of [{ ctrlKey: true }, { metaKey: true }]) {
      expect(wheel(viewport, { deltaY: 60, ...modifier }).defaultPrevented).toBe(false);
      expect(viewport.scrollLeft).toBe(0);
    }
    const consumed = new WheelEvent("wheel", { deltaY: 60, cancelable: true });
    consumed.preventDefault();
    fireEvent(viewport, consumed);
    expect(viewport.scrollLeft).toBe(0);
  });

  it("normalizes pixel, line and page deltas and releases outward wheel events at either edge", () => {
    const viewport = setup();
    expect(wheel(viewport, { deltaX: 70, deltaY: 10 }).defaultPrevented).toBe(true);
    expect(viewport.scrollLeft).toBe(70);
    wheel(viewport, { deltaY: 2, deltaMode: 1 });
    expect(viewport.scrollLeft).toBe(102);
    wheel(viewport, { deltaY: 1, deltaMode: 2 });
    expect(viewport.scrollLeft).toBe(302);
    wheel(viewport, { deltaY: 1000 });
    expect(viewport.scrollLeft).toBe(600);
    expect(wheel(viewport, { deltaY: 30 }).defaultPrevented).toBe(false);
    wheel(viewport, { deltaY: -1000 });
    expect(viewport.scrollLeft).toBe(0);
    expect(wheel(viewport, { deltaY: -30 }).defaultPrevented).toBe(false);
  });

  it("leaves wheel events untouched when all tabs fit", () => {
    const viewport = setup(200);
    expect(wheel(viewport, { deltaY: 80 }).defaultPrevented).toBe(false);
    expect(viewport.scrollLeft).toBe(0);
  });
});
