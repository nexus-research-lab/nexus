// INPUT: 鼠标、键盘与布局 owner 的有效宽度范围。
// OUTPUT: 主键拖动、键盘有界调整、原生焦点顺序和分隔条语义。
// POS: 分栏入口事件合同回归，不进行视觉验收。

import { createEvent, fireEvent, render as renderDom, screen } from "@testing-library/react";
import { type ReactNode } from "react";
import { expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { PanelResizeHandle } from "./panel-resize-handle";

function render(children: ReactNode) {
  const wrapper = ({ children }: { children: ReactNode }) => <I18N_CONTEXT.Provider value={{
    locale: "en", setLocale: vi.fn(), t: (key, params) => key === "common.panel_width_pixels" ? `${params?.width} pixels wide` : key,
  }}>{children}</I18N_CONTEXT.Provider>;
  return renderDom(children, { wrapper });
}

it("starts only a primary mouse drag", () => {
  const start = vi.fn();
  render(<PanelResizeHandle ariaLabel="Resize files" controls="files" control={{ min: 200, max: 360, value: 240, onChange: vi.fn() }} onResizeStart={start} />);
  const handle = screen.getByRole("separator", { name: "Resize files" });
  fireEvent.mouseDown(handle, { button: 1 });
  fireEvent.mouseDown(handle, { button: 2 });
  expect(start).not.toHaveBeenCalled();
  const event = createEvent.mouseDown(handle, { button: 0, cancelable: true });
  fireEvent(handle, event);
  expect(event.defaultPrevented).toBe(true);
  expect(start).toHaveBeenCalledTimes(1);
  expect(document.activeElement).toBe(handle);
});

it("consumes only unmodified resize keys, preserving other page shortcuts", () => {
  const onChange = vi.fn();
  const parentKey = vi.fn();
  document.addEventListener("keydown", parentKey);
  try {
    render(<PanelResizeHandle ariaLabel="Files" controls="files" control={{ min: 200, max: 360, value: 240, onChange }} onResizeStart={vi.fn()} />);
    const handle = screen.getByRole("separator");
    for (const modifiers of [{ ctrlKey: true }, { altKey: true }, { metaKey: true }]) {
      const event = createEvent.keyDown(handle, { key: "ArrowLeft", cancelable: true, ...modifiers });
      fireEvent(handle, event);
      expect(event.defaultPrevented).toBe(false);
    }
    fireEvent.keyDown(handle, { key: "Enter" });
    fireEvent.keyDown(handle, { key: " " });
    expect(onChange).not.toHaveBeenCalled();
    expect(parentKey).toHaveBeenCalledTimes(5);
    const event = createEvent.keyDown(handle, { key: "ArrowLeft", cancelable: true });
    fireEvent(handle, event);
    expect(event.defaultPrevented).toBe(true);
    expect(parentKey).toHaveBeenCalledTimes(5);
    expect(onChange).toHaveBeenCalledWith(256);
  } finally {
    document.removeEventListener("keydown", parentKey);
  }
});
