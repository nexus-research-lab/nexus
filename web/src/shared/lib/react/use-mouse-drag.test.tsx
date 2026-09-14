// INPUT: 鼠标、窗口可见性和调用方启用状态变化。
// OUTPUT: 拖动结束边界、最新回调及监听释放回归。
// POS: 共享鼠标生命周期的 DOM 功能测试，不验证视觉几何。

import { act, fireEvent, renderHook } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import { useMouseDrag } from "./use-mouse-drag";

afterEach(() => vi.restoreAllMocks());

it.each(["mouseup", "outside-release", "blur", "hidden"])("ends a drag on %s", (reason) => {
  const onMove = vi.fn();
  const { result } = renderHook(() => useMouseDrag(onMove));
  act(() => result.current.startDragging());
  fireEvent.mouseMove(window, { buttons: 1, clientX: 100 });
  expect(onMove).toHaveBeenCalledTimes(1);
  if (reason === "mouseup") fireEvent.mouseUp(window, { button: 0, buttons: 0 });
  if (reason === "outside-release") fireEvent.mouseMove(window, { buttons: 0, clientX: 200 });
  if (reason === "blur") fireEvent.blur(window);
  if (reason === "hidden") {
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
    fireEvent(document, new Event("visibilitychange"));
  }
  expect(result.current.isDragging).toBe(false);
  fireEvent.mouseMove(window, { buttons: 1, clientX: 300 });
  expect(onMove).toHaveBeenCalledTimes(1);
});

it("keeps the primary drag when a secondary button is released", () => {
  const onMove = vi.fn();
  const { result } = renderHook(() => useMouseDrag(onMove));
  act(() => result.current.startDragging());
  fireEvent.mouseUp(window, { button: 2, buttons: 1 });
  expect(result.current.isDragging).toBe(true);
  fireEvent.mouseMove(window, { buttons: 1 });
  expect(onMove).toHaveBeenCalledTimes(1);
  act(() => result.current.stopDragging());
  fireEvent.mouseMove(window, { buttons: 1 });
  expect(onMove).toHaveBeenCalledTimes(1);
});

it("stops when disabled and does not restart when enabled again", () => {
  const onMove = vi.fn();
  const { result, rerender } = renderHook(({ enabled }) => useMouseDrag(onMove, enabled), {
    initialProps: { enabled: true },
  });
  act(() => result.current.startDragging());
  rerender({ enabled: false });
  expect(result.current.isDragging).toBe(false);
  act(() => result.current.startDragging());
  fireEvent.mouseMove(window, { buttons: 1 });
  expect(onMove).not.toHaveBeenCalled();
  rerender({ enabled: true });
  expect(result.current.isDragging).toBe(false);
  fireEvent.mouseMove(window, { buttons: 1 });
  expect(onMove).not.toHaveBeenCalled();
});

it("uses the current projection and releases listeners on unmount", () => {
  const first = vi.fn();
  const second = vi.fn();
  const { result, rerender, unmount } = renderHook(({ onMove }) => useMouseDrag(onMove), {
    initialProps: { onMove: first },
  });
  act(() => result.current.startDragging());
  rerender({ onMove: second });
  fireEvent.mouseMove(window, { buttons: 1 });
  expect(first).not.toHaveBeenCalled();
  expect(second).toHaveBeenCalledTimes(1);
  unmount();
  fireEvent.mouseMove(window, { buttons: 1 });
  fireEvent.blur(window);
  expect(second).toHaveBeenCalledTimes(1);
});
