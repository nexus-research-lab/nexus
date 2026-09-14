// INPUT: 主按钮、右键与面板边界外指针事件。
// OUTPUT: 验证只有合法右缘拖拽能提交宽度。
// POS: 侧栏 resize 交互回归，不访问持久化 Store。
import { act, renderHook } from "@testing-library/react";
import type { PointerEvent } from "react";
import { describe, expect, it, vi } from "vitest";

import { useSidebarPanelResize } from "./use-sidebar-panel-resize";

function setup() {
  const setWidth = vi.fn();
  const hook = renderHook(() => useSidebarPanelResize({ width: 300, setWidth }));
  const element = document.createElement("div");
  element.getBoundingClientRect = () => ({ right: 300 }) as DOMRect;
  element.setPointerCapture = vi.fn();
  hook.result.current.rootRef.current = element;
  const event = (clientX: number, button = 0) => ({
    button, clientX, pointerId: 1, target: element, currentTarget: element,
    preventDefault: vi.fn(),
  }) as unknown as PointerEvent<HTMLDivElement>;
  return { ...hook, setWidth, event, element };
}

describe("sidebar resize admission", () => {
  it("ignores secondary buttons and events outside the panel", () => {
    const { result, event, element, setWidth } = setup();
    for (const input of [event(298, 2), event(310), event(280)]) {
      act(() => result.current.handlePointerDown(input));
      expect(result.current.isResizing).toBe(false);
    }
    act(() => result.current.handlePointerMove(event(310)));
    expect(result.current.isResizeHotzoneActive).toBe(false);
    expect(element.setPointerCapture).not.toHaveBeenCalled();
    expect(setWidth).not.toHaveBeenCalled();
  });

  it("keeps a captured primary drag active outside the original edge until released", () => {
    const { result, event, setWidth } = setup();
    act(() => result.current.handlePointerDown(event(298)));
    expect(result.current.isResizing).toBe(true);
    act(() => result.current.handlePointerMove(event(350)));
    expect(setWidth).toHaveBeenLastCalledWith(352);
    act(() => result.current.handlePointerUp());
    expect(result.current.isResizing).toBe(false);
    expect(result.current.isResizeHotzoneActive).toBe(false);
  });
});
