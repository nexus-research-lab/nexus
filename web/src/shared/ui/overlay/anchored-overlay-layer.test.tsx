// INPUT: 真实 Portal 浮层、嵌套 Tooltip 与跨模态根的键盘/指针事件。
// OUTPUT: 证明 Escape 只关闭当前模态范围的最上层浮层，且焦点归还不越过该范围。
// POS: Anchored Overlay 生命周期集成测试；不复制业务菜单或定位计算。

import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useCallback, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { describe, expect, it, vi } from "vitest";

import { UiDialogBackdrop, UiDialogPortal, UiDialogShell } from "@/shared/ui/dialog/dialog";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";

import { useAnchoredOverlayLayer } from "./anchored-overlay-layer";
import { resolveUiAnchoredOverlayPosition } from "./anchored-overlay-layout";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "./overlay-contract";
import { UiTooltip } from "./tooltip";

function NestedOverlayHarness({ deferUntilAnchor = false, initialOpen = false, revision = 0 }: {
  deferUntilAnchor?: boolean;
  initialOpen?: boolean;
  revision?: number;
}) {
  const anchorRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(initialOpen);
  const estimatePosition = useCallback((anchor: HTMLButtonElement) => (
    resolveUiAnchoredOverlayPosition({
      anchor,
      estimatedContentHeight: 100,
      placement: "auto",
      preset: "form-picker",
    })
  ), []);
  const { overlayRef, overlayStyle, portalContainer } = useAnchoredOverlayLayer({
    anchorRef,
    disabled: false,
    estimatePosition,
    isOpen,
    onClose: () => setIsOpen(false),
  });
  return (
    <>
      <button ref={anchorRef} onClick={() => setIsOpen(true)} type="button">打开浮层</button>
      {isOpen && (!deferUntilAnchor || anchorRef.current) && portalContainer ? createPortal(
        <div
          ref={overlayRef}
          aria-label="父浮层"
          data-revision={revision}
          role="dialog"
          style={overlayStyle}
          {...OPEN_OVERLAY_DATA_ATTRIBUTES}
        >
          <UiTooltip label="子提示">
            <button type="button">说明</button>
          </UiTooltip>
          <UiSelectMenu
            ariaLabel="子选择"
            onChange={() => undefined}
            options={[{ label: "Alpha", value: "alpha" }]}
            value="alpha"
          />
        </div>,
        portalContainer,
      ) : null}
    </>
  );
}

describe("anchored overlay dismissal", () => {
  it("registers a delayed initial overlay in the same portal container and releases it after closing", async () => {
    const user = userEvent.setup();
    const view = render(<NestedOverlayHarness deferUntilAnchor initialOpen />);
    const opener = screen.getByRole("button", { name: "打开浮层" });
    const overlay = await screen.findByRole("dialog", { name: "父浮层" });
    expect(overlay.parentElement).toBe(document.body);

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog", { name: "父浮层" })).toBeNull();
    expect(document.activeElement).toBe(opener);

    await user.click(opener);
    expect(screen.getByRole("dialog", { name: "父浮层" })).toBeTruthy();
    await user.click(document.body);
    expect(screen.queryByRole("dialog", { name: "父浮层" })).toBeNull();
    view.unmount();

    const escape = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, key: "Escape" });
    document.dispatchEvent(escape);
    expect(escape.defaultPrevented).toBe(false);
  });

  it("keeps an initially open overlay registered when its portal resolves into a modal", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(
      <UiDialogPortal>
        <UiDialogBackdrop labelledBy="initial-modal-title" onClose={onClose}>
          <UiDialogShell>
            <h2 id="initial-modal-title">初始弹窗</h2>
            <NestedOverlayHarness initialOpen />
          </UiDialogShell>
        </UiDialogBackdrop>
      </UiDialogPortal>,
    );
    const modal = screen.getByRole("dialog", { name: "初始弹窗" });
    const overlay = screen.getByRole("dialog", { name: "父浮层" });
    await waitFor(() => expect(overlay.parentElement).toBe(modal));

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog", { name: "父浮层" })).toBeNull();
    expect(screen.getByRole("dialog", { name: "初始弹窗" })).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();

    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("closes a nested tooltip before its parent overlay and restores each trigger in order", async () => {
    const user = userEvent.setup();
    const view = render(<NestedOverlayHarness />);
    const opener = screen.getByRole("button", { name: "打开浮层" });
    await user.click(opener);
    const childTrigger = screen.getByRole("button", { name: "说明" });
    act(() => childTrigger.focus());
    expect(screen.getByRole("tooltip", { name: "子提示" })).toBeTruthy();
    // 父组件回调更新不能把已打开的父层重新提升到子层上方。
    view.rerender(<NestedOverlayHarness revision={1} />);

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("tooltip")).toBeNull();
    expect(screen.getByRole("dialog", { name: "父浮层" })).toBeTruthy();
    expect(document.activeElement).toBe(childTrigger);

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog", { name: "父浮层" })).toBeNull();
    expect(document.activeElement).toBe(opener);
  });

  it("treats a child portal as inside its parent and dismisses outside without stealing focus", async () => {
    const user = userEvent.setup();
    render(
      <>
        <NestedOverlayHarness />
        <button type="button">外部动作</button>
      </>,
    );
    await user.click(screen.getByRole("button", { name: "打开浮层" }));
    await user.click(screen.getByRole("button", { name: "子选择" }));
    await user.click(screen.getByRole("option", { name: "Alpha" }));
    expect(screen.queryByRole("listbox", { name: "子选择" })).toBeNull();
    expect(screen.getByRole("dialog", { name: "父浮层" })).toBeTruthy();

    const outsideAction = screen.getByRole("button", { name: "外部动作" });
    await user.click(outsideAction);
    expect(screen.queryByRole("dialog", { name: "父浮层" })).toBeNull();
    expect(document.activeElement).toBe(outsideAction);
  });

  it("keeps background selectors out of the current modal Escape scope", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const rects = vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue(
      [{} as DOMRect] as unknown as DOMRectList,
    );
    function Harness({ showModal }: { showModal: boolean }) {
      return (
        <>
          <UiSelectMenu
            ariaLabel="背景选择"
            onChange={() => undefined}
            options={[{ label: "Alpha", value: "alpha" }]}
            value="alpha"
          />
          {showModal ? (
            <UiDialogPortal>
              <UiDialogBackdrop labelledBy="modal-title" onClose={onClose}>
                <UiDialogShell>
                  <h2 id="modal-title">当前弹窗</h2>
                  <button type="button">弹窗动作</button>
                </UiDialogShell>
              </UiDialogBackdrop>
            </UiDialogPortal>
          ) : null}
        </>
      );
    }

    try {
      const view = render(<Harness showModal={false} />);
      await user.click(screen.getByRole("button", { name: "背景选择" }));
      view.rerender(<Harness showModal />);
      const modalAction = screen.getByRole("button", { name: "弹窗动作" });
      await waitFor(() => expect(document.activeElement).toBe(modalAction));

      await user.keyboard("{Escape}");
      expect(onClose).toHaveBeenCalledOnce();
      expect(screen.getByRole("listbox", { name: "背景选择" })).toBeTruthy();
      expect(document.activeElement).toBe(modalAction);
    } finally {
      rects.mockRestore();
    }
  });
});
