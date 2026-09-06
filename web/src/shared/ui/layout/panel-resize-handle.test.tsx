// INPUT: 不同鼠标键在分栏热区的按下事件。
// OUTPUT: 仅主键启动拖动并阻止原生文字选择。
// POS: 分栏入口事件合同回归，不进行视觉验收。

import { createEvent, fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";

import { PanelResizeHandle } from "./panel-resize-handle";

it("starts only a primary mouse drag", () => {
  const start = vi.fn();
  render(<PanelResizeHandle ariaLabel="Resize files" onResizeStart={start} />);
  const handle = screen.getByRole("button", { name: "Resize files" });
  fireEvent.mouseDown(handle, { button: 1 });
  fireEvent.mouseDown(handle, { button: 2 });
  expect(start).not.toHaveBeenCalled();
  const event = createEvent.mouseDown(handle, { button: 0, cancelable: true });
  fireEvent(handle, event);
  expect(event.defaultPrevented).toBe(true);
  expect(start).toHaveBeenCalledTimes(1);
});
