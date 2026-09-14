// INPUT: 现有右栏宽度偏好、CSS 限制和受控容器/视口变化。
// OUTPUT: 键盘从有效宽度调整、范围变化与测量监听释放回归。
// POS: 右栏几何适配的 DOM 功能测试；使用给定尺寸，不做视觉验证。

import { act, fireEvent, render as renderDom, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ReactNode } from "react";
import { afterEach, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { PanelResizeHandle } from "@/shared/ui/layout/panel-resize-handle";

import { getRoomSidePanelWidthRange, getRoomSidePanelWidthStyle, type RoomSidePanelKind } from "./room-side-panel-width";
import { useRoomSidePanelResize } from "./use-room-side-panel-resize";

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

function render(children: ReactNode) {
  const wrapper = ({ children }: { children: ReactNode }) => <I18N_CONTEXT.Provider value={{
    locale: "en", setLocale: vi.fn(), t: (key, params) => key === "common.panel_width_pixels" ? `${params?.width} pixels wide` : key,
  }}>{children}</I18N_CONTEXT.Provider>;
  return renderDom(children, { wrapper });
}

it("retains the original CSS bounds and projects the effective resize range", () => {
  expect(getRoomSidePanelWidthStyle("thread")).toEqual({ minWidth: "360px", maxWidth: "560px" });
  expect(getRoomSidePanelWidthStyle("auxiliary")).toEqual({ minWidth: "min(520px, 46vw)", maxWidth: "min(860px, 54vw)" });
  expect(getRoomSidePanelWidthRange("thread", 56, 1600, 1600)).toEqual({ min: 480, max: 560, value: 560 });
  expect(getRoomSidePanelWidthRange("auxiliary", 56, 1000, 1000)).toEqual({ min: 460, max: 540, value: 540 });
  expect(getRoomSidePanelWidthRange("thread", 56, 600, 1600)).toEqual({ min: 360, max: 360, value: 360 });
  expect(getRoomSidePanelWidthRange("auxiliary", 56, 0, 1000)).toBeNull();
});

it("moves immediately from a CSS-capped width and updates geometry without rewriting the saved preference", async () => {
  let containerWidth = 1600;
  let notifyResize = () => {};
  const disconnect = vi.fn();
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(() => ({ width: containerWidth } as DOMRect));
  vi.stubGlobal("innerWidth", 1600);
  vi.stubGlobal("ResizeObserver", class {
    constructor(callback: () => void) { notifyResize = callback; }
    observe() {}
    disconnect = disconnect;
  });
  const changed = vi.fn();
  function Harness({ kind }: { kind: RoomSidePanelKind }) {
    const [percent, setPercent] = useState(56);
    const resize = useRoomSidePanelResize(kind, percent, (next) => { changed(next); setPercent(next); });
    return <div>
      <PanelResizeHandle ariaLabel="Details" controls={resize.panelId} control={resize.control} onResizeStart={vi.fn()} />
      <section id={resize.panelId} ref={resize.panelRef} style={{ width: `${percent}%`, ...resize.widthStyle }} />
    </div>;
  }
  const user = userEvent.setup();
  const { rerender, unmount } = render(<Harness kind="thread" />);
  const handle = screen.getByRole("separator");
  const panel = document.getElementById(handle.getAttribute("aria-controls")!);
  expect(panel).not.toBeNull();
  expect(handle.getAttribute("aria-valuenow")).toBe("560");
  await user.tab();
  await user.keyboard("{ArrowRight}");
  expect(changed).toHaveBeenLastCalledWith(34);
  expect(panel?.style.width).toBe("34%");
  expect(handle.getAttribute("aria-valuenow")).toBe("544");
  await user.keyboard("{Home}{End}");
  expect(changed.mock.calls.map(([value]) => value)).toEqual([34, 30, 35]);

  containerWidth = 1200;
  act(() => notifyResize());
  expect(handle.getAttribute("aria-valuenow")).toBe("420");
  expect(changed).toHaveBeenCalledTimes(3);
  rerender(<Harness kind="auxiliary" />);
  expect(handle.getAttribute("aria-valuenow")).toBe("520");
  vi.stubGlobal("innerWidth", 1000);
  fireEvent(window, new Event("resize"));
  expect(handle.getAttribute("aria-valuemin")).toBe("460");
  expect(handle.getAttribute("aria-valuemax")).toBe("540");
  expect(changed).toHaveBeenCalledTimes(3);

  containerWidth = 0;
  act(() => notifyResize());
  expect(handle.getAttribute("aria-disabled")).toBe("true");
  unmount();
  expect(disconnect).toHaveBeenCalledOnce();
  act(() => notifyResize());
  fireEvent(window, new Event("resize"));
  expect(changed).toHaveBeenCalledTimes(3);
});
