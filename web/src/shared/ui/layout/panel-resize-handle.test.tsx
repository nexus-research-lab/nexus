// INPUT: 鼠标、键盘与布局 owner 的有效宽度范围。
// OUTPUT: 主键拖动、键盘有界调整、原生焦点顺序和分隔条语义。
// POS: 分栏入口事件合同回归，不进行视觉验收。

import { createEvent, fireEvent, render as renderDom, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ReactNode } from "react";
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

it("adjusts the right pane with arrows and bounds without trapping Tab or starting a drag", async () => {
  const onChange = vi.fn();
  const start = vi.fn();
  function Harness() {
    const [value, setValue] = useState(240);
    return <>
      <button>Before</button>
      <PanelResizeHandle ariaLabel="Files" controls="files" control={{ min: 200, max: 360, value,
        onChange: (next) => { onChange(next); setValue(next); } }} onResizeStart={start} />
      <section id="files"><button>After</button></section>
    </>;
  }
  const user = userEvent.setup();
  render(<Harness />);
  await user.tab();
  await user.tab();
  const handle = screen.getByRole("separator", { name: "Files" });
  expect(document.activeElement).toBe(handle);
  expect(handle.getAttribute("aria-controls")).toBe("files");
  expect(document.getElementById("files")).not.toBeNull();
  expect(handle.getAttribute("aria-orientation")).toBe("vertical");
  await user.keyboard("{ArrowLeft}{ArrowRight}{End}{End}{Home}{ArrowRight}");
  expect(onChange.mock.calls.map(([width]) => width)).toEqual([256, 240, 360, 200]);
  expect(handle.getAttribute("aria-valuenow")).toBe("200");
  expect(handle.getAttribute("aria-valuetext")).toBe("200 pixels wide");
  expect(handle.getAttribute("aria-valuemin")).toBe("200");
  expect(handle.getAttribute("aria-valuemax")).toBe("360");
  expect(start).not.toHaveBeenCalled();
  await user.tab();
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "After" }));
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

it("skips unmeasured and fixed-width handles in the focus order", async () => {
  const start = vi.fn();
  const onChange = vi.fn();
  const { rerender } = render(<><PanelResizeHandle ariaLabel="Files" controls="files" control={null} onResizeStart={start} /><button>After</button></>);
  const user = userEvent.setup();
  await user.tab();
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "After" }));
  const fixed = { min: 200, max: 200, value: 200, onChange };
  rerender(<><PanelResizeHandle ariaLabel="Files" controls="files" control={fixed} onResizeStart={start} /><button>After</button></>);
  const handle = screen.getByRole("separator");
  expect(handle.getAttribute("aria-disabled")).toBe("true");
  fireEvent.mouseDown(handle, { button: 0 });
  fireEvent.keyDown(handle, { key: "ArrowLeft" });
  expect(start).not.toHaveBeenCalled();
  expect(onChange).not.toHaveBeenCalled();
});
