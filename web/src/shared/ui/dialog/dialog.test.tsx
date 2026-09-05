// INPUT: Dialog 的打开状态、嵌套关系、焦点元素、子浮层与用户键盘/点击事件。
// OUTPUT: 证明 Portal 模态的焦点圈、关闭顺序、遮罩策略、滚动锁和焦点归还合同。
// POS: Dialog primitive DOM 行为测试；业务确认结果和具体文案由 feature 测试负责。

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useState } from "react";

import {
  UiDialogBackdrop,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import {
  isTopDialogModal,
  registerDialogModal,
  unregisterDialogModal,
} from "@/shared/ui/dialog/dialog-modal-runtime";

function TestDialog({
  onClose,
  title = "测试弹窗",
}: {
  onClose: () => void;
  title?: string;
}) {
  const titleId = `dialog-title-${title}`;
  return (
    <UiDialogPortal>
      <UiDialogBackdrop labelledBy={titleId} onClose={onClose}>
        <UiDialogShell>
          <h2 id={titleId}>{title}</h2>
          <button type="button">第一个操作</button>
          <button disabled type="button">禁用操作</button>
          <button type="button">最后一个操作</button>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue(
    [{} as DOMRect] as unknown as DOMRectList,
  );
});

afterEach(() => {
  vi.restoreAllMocks();
  document.body.style.overflow = "";
});

describe("UiDialog modal behavior", () => {
  it("uses the dialog semantic layer by default and preserves explicit nesting", () => {
    render(
      <>
        <UiDialogBackdrop
          data-testid="default-layer"
          labelledBy="default-layer-title"
          trapFocus={false}
        >
          <h2 id="default-layer-title">默认层</h2>
        </UiDialogBackdrop>
        <UiDialogBackdrop
          data-testid="nested-layer"
          labelledBy="nested-layer-title"
          layer="dialogNested"
          trapFocus={false}
        >
          <h2 id="nested-layer-title">嵌套层</h2>
        </UiDialogBackdrop>
      </>,
    );

    expect(screen.getByTestId("default-layer").className)
      .toContain("ui-layer-dialog");
    expect(screen.getByTestId("nested-layer").className)
      .toContain("ui-layer-dialog-nested");
  });

  it("cycles focus, closes with Escape, and restores the opener and body scroll", async () => {
    const user = userEvent.setup();
    document.body.style.overflow = "clip";

    function Harness() {
      const [isOpen, setIsOpen] = useState(false);
      return (
        <>
          <button onClick={() => setIsOpen(true)} type="button">打开弹窗</button>
          {isOpen ? <TestDialog onClose={() => setIsOpen(false)} /> : null}
        </>
      );
    }

    render(<Harness />);
    const opener = screen.getByRole("button", { name: "打开弹窗" });
    await user.click(opener);

    const first = screen.getByRole("button", { name: "第一个操作" });
    const last = screen.getByRole("button", { name: "最后一个操作" });
    await waitFor(() => expect(document.activeElement).toBe(first));
    expect(document.body.style.overflow).toBe("hidden");

    await user.keyboard("{Shift>}{Tab}{/Shift}");
    expect(document.activeElement).toBe(last);
    await user.keyboard("{Tab}");
    expect(document.activeElement).toBe(first);

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog", { name: "测试弹窗" })).toBeNull();
    expect(document.activeElement).toBe(opener);
    expect(document.body.style.overflow).toBe("clip");
  });

  it("closes only the top dialog before its parent", async () => {
    const user = userEvent.setup();

    function Harness() {
      const [outerOpen, setOuterOpen] = useState(false);
      const [innerOpen, setInnerOpen] = useState(false);
      return (
        <>
          <button onClick={() => setOuterOpen(true)} type="button">打开外层</button>
          {outerOpen ? (
            <UiDialogPortal>
              <UiDialogBackdrop labelledBy="outer-title" onClose={() => setOuterOpen(false)}>
                <UiDialogShell>
                  <h2 id="outer-title">外层弹窗</h2>
                  <button onClick={() => setInnerOpen(true)} type="button">打开内层</button>
                </UiDialogShell>
              </UiDialogBackdrop>
            </UiDialogPortal>
          ) : null}
          {innerOpen ? <TestDialog onClose={() => setInnerOpen(false)} title="内层弹窗" /> : null}
        </>
      );
    }

    render(<Harness />);
    const outerOpener = screen.getByRole("button", { name: "打开外层" });
    await user.click(outerOpener);
    const innerOpener = screen.getByRole("button", { name: "打开内层" });
    await waitFor(() => expect(document.activeElement).toBe(innerOpener));
    await user.click(innerOpener);
    await screen.findByRole("dialog", { name: "内层弹窗" });

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog", { name: "内层弹窗" })).toBeNull();
    expect(screen.getByRole("dialog", { name: "外层弹窗" })).toBeTruthy();
    expect(document.activeElement).toBe(innerOpener);

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog", { name: "外层弹窗" })).toBeNull();
    expect(document.activeElement).toBe(outerOpener);
  });

  it("lets an open child overlay consume Escape first", async () => {
    const user = userEvent.setup();

    function Harness() {
      const [isOpen, setIsOpen] = useState(false);
      return (
        <>
          <button onClick={() => setIsOpen(true)} type="button">打开设置</button>
          {isOpen ? <TestDialog onClose={() => setIsOpen(false)} title="设置" /> : null}
        </>
      );
    }

    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "打开设置" }));
    await screen.findByRole("dialog", { name: "设置" });

    const overlay = document.createElement("div");
    overlay.dataset.uiOverlayOpen = "true";
    screen.getByRole("dialog", { name: "设置" }).append(overlay);
    await user.keyboard("{Escape}");
    expect(screen.getByRole("dialog", { name: "设置" })).toBeTruthy();

    overlay.remove();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog", { name: "设置" })).toBeNull();
  });

  it("closes from the backdrop itself but not from dialog content", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(
      <UiDialogBackdrop labelledBy="click-title" onClose={onClose} trapFocus={false}>
        <UiDialogShell>
          <h2 id="click-title">点击策略</h2>
          <p>弹窗内容</p>
        </UiDialogShell>
      </UiDialogBackdrop>,
    );

    await user.click(screen.getByText("弹窗内容"));
    expect(onClose).not.toHaveBeenCalled();
    await user.click(screen.getByRole("dialog", { name: "点击策略" }));
    expect(onClose).toHaveBeenCalledOnce();
  });
});

describe("dialog modal runtime", () => {
  it("keeps scroll locked until every unique registration is released", () => {
    document.body.style.overflow = "clip";
    const first = registerDialogModal();
    const second = registerDialogModal();

    expect(isTopDialogModal(second)).toBe(true);
    unregisterDialogModal(first);
    unregisterDialogModal(first);
    expect(document.body.style.overflow).toBe("hidden");
    expect(isTopDialogModal(second)).toBe(true);

    unregisterDialogModal(second);
    expect(document.body.style.overflow).toBe("clip");
  });
});
