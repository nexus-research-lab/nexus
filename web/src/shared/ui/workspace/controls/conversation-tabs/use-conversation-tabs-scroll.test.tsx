// INPUT: 原生滚轮事件与明确的视口尺寸。
// OUTPUT: 验证滚轮消费边界与缩放手势保留。
// POS: 共享标签滚动 Hook 的 DOM 行为回归，不声称浏览器几何验收。

import type { PointerEvent } from "react";
import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { useConversationTabsScroll } from "./use-conversation-tabs-scroll";

const motion = vi.hoisted(() => ({ reduced: false }));
vi.mock("@/shared/lib/react/use-prefers-reduced-motion", () => ({ usePrefersReducedMotion: () => motion.reduced }));

function Harness({ active = false }: { active?: boolean }) {
  const { viewportRef } = useConversationTabsScroll({
    activeConversationId: active ? "active" : null,
    contentKey: "tabs",
  });
  return <div ref={viewportRef} data-testid="viewport"><div data-conversation-tab-id="active" /></div>;
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


it.each([false, true])("uses the system motion preference for active tab alignment (%s)", async (reduced) => {
  motion.reduced = reduced;
  const view = render(<Harness active />);
  const viewport = screen.getByTestId("viewport");
  const scrollTo = vi.fn();
  viewport.scrollTo = scrollTo;
  await waitFor(() => expect(scrollTo).toHaveBeenCalled());
  expect(scrollTo.mock.calls[0][0].behavior).toBe(reduced ? "auto" : "smooth");
  view.unmount();
  motion.reduced = false;
});

it("stops dragging when pointer capture is lost without moving on later pointer events", () => {
  const { result } = renderHook(() => useConversationTabsScroll({ activeConversationId: null, contentKey: "" }));
  const viewport = document.createElement("div");
  viewport.setPointerCapture = vi.fn();
  viewport.hasPointerCapture = vi.fn(() => false);
  viewport.releasePointerCapture = vi.fn();
  const pointer = (clientX: number) => ({
    button: 0, pointerType: "mouse", pointerId: 4, clientX,
    currentTarget: viewport, preventDefault: vi.fn(),
  }) as unknown as PointerEvent<HTMLDivElement>;
  act(() => { result.current.handlePointerDown(pointer(100)); });
  act(() => { result.current.handlePointerMove(pointer(90)); });
  expect(result.current.isDragging).toBe(true);
  expect(viewport.scrollLeft).toBe(10);
  act(() => { result.current.handleLostPointerCapture(pointer(90)); });
  expect(result.current.isDragging).toBe(false);
  act(() => { result.current.handlePointerMove(pointer(50)); });
  expect(viewport.scrollLeft).toBe(10);
});
