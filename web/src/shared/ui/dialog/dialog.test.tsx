// INPUT: Dialog 的打开状态、嵌套关系、焦点元素、子浮层与用户键盘/点击事件。
// OUTPUT: 证明动态/显式标题和实例隔离，以及 Portal 模态的焦点圈、关闭顺序、遮罩策略、滚动锁和焦点归还合同。
// POS: Dialog primitive DOM 行为测试；业务确认结果和具体文案由 feature 测试负责。

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { StrictMode, useState } from "react";

import {
  UiDialogBackdrop,
  UiDialogHeader,
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
  it("names a dialog from its mounted Header and updates or clears the relationship with that title", () => {
    const view = (title: string | null) => <StrictMode>
      <UiDialogBackdrop trapFocus={false}>
        <UiDialogShell><UiDialogHeader title={title} subtitle="Additional context" /></UiDialogShell>
      </UiDialogBackdrop>
    </StrictMode>;
    const { rerender } = render(view("Import Skill"));
    const dialog = screen.getByRole("dialog", { name: "Import Skill" });
    const titleId = dialog.getAttribute("aria-labelledby");
    expect(titleId).toBeTruthy();
    expect(document.getElementById(titleId!)?.textContent).toBe("Import Skill");
    // Complex content is not automatically flattened into a description.
    expect(dialog.hasAttribute("aria-describedby")).toBe(false);
    rerender(view("Review Skill"));
    expect(screen.getByRole("dialog", { name: "Review Skill" })).toBe(dialog);
    expect(dialog.getAttribute("aria-labelledby")).toBe(titleId);
    rerender(view(null));
    expect(dialog.hasAttribute("aria-labelledby")).toBe(false);
    expect(document.getElementById(titleId!)).toBeNull();
    rerender(view("Choose Skill"));
    expect(screen.getByRole("dialog", { name: "Choose Skill" })).toBe(dialog);
  });

  it.each([
    { labelledBy: "explicit-dialog-title" },
    { "aria-labelledby": "explicit-dialog-title" },
    { "aria-label": "Custom preview" },
  ])("preserves explicit dialog naming %j alongside the standard Header", (nameProps) => {
    const { rerender } = render(<UiDialogBackdrop {...nameProps} describedBy="dialog-description" trapFocus={false}>
      <UiDialogHeader title="Visible heading" titleId="custom-header-id" />
      <span id="explicit-dialog-title">Custom preview</span>
      <p id="dialog-description">Selected object details.</p>
    </UiDialogBackdrop>);
    const dialog = screen.getByRole("dialog", { name: "Custom preview" });
    expect(dialog.getAttribute("aria-describedby")).toBe("dialog-description");
    expect(screen.getByRole("heading", { name: "Visible heading" }).id).toBe("custom-header-id");
    rerender(<UiDialogBackdrop trapFocus={false}>
      <UiDialogHeader title="Visible heading" titleId="custom-header-id" />
    </UiDialogBackdrop>);
    expect(screen.getByRole("dialog", { name: "Visible heading" })).toBe(dialog);
    expect(dialog.getAttribute("aria-labelledby")).toBe("custom-header-id");
  });

  it("isolates nested portal titles and releases a removed Header without naming its parent", () => {
    function Nested({ inner }: { inner: boolean }) {
      return <UiDialogBackdrop data-testid="outer-named-dialog" trapFocus={false}>
        <UiDialogHeader title="Outer editor" />
        {inner ? <UiDialogPortal><UiDialogBackdrop data-testid="inner-named-dialog" trapFocus={false}>
          <UiDialogHeader title="Inner preview" />
        </UiDialogBackdrop></UiDialogPortal> : null}
      </UiDialogBackdrop>;
    }
    const { rerender } = render(<Nested inner />);
    const outer = screen.getByRole("dialog", { name: "Outer editor" });
    const inner = screen.getByRole("dialog", { name: "Inner preview" });
    expect(outer.getAttribute("aria-labelledby")).not.toBe(inner.getAttribute("aria-labelledby"));
    expect(within(outer).queryByRole("heading", { name: "Inner preview" })).toBeNull();
    rerender(<Nested inner={false} />);
    expect(screen.getByRole("dialog", { name: "Outer editor" })).toBe(outer);
    expect(screen.queryByRole("dialog", { name: "Inner preview" })).toBeNull();
  });

  it("leaves a custom Header's naming to its caller without dangling generated references", () => {
    render(<UiDialogBackdrop aria-label="Custom workspace" trapFocus={false}>
      <UiDialogHeader title="Unused default"><h2>Custom content</h2></UiDialogHeader>
    </UiDialogBackdrop>);
    const dialog = screen.getByRole("dialog", { name: "Custom workspace" });
    expect(dialog.hasAttribute("aria-labelledby")).toBe(false);
    expect(screen.queryByText("Unused default")).toBeNull();
  });

  it("keeps same-named dialog instances bound to their own title when a sibling unmounts", () => {
    const view = (first: boolean) => <>
      {first ? <UiDialogBackdrop key="first" trapFocus={false}><UiDialogHeader title="Review" /></UiDialogBackdrop> : null}
      <UiDialogBackdrop key="second" trapFocus={false}><UiDialogHeader title="Review" /></UiDialogBackdrop>
    </>;
    const { rerender } = render(view(true));
    const dialogs = screen.getAllByRole("dialog", { name: "Review" });
    const ids = dialogs.map((dialog) => dialog.getAttribute("aria-labelledby"));
    expect(new Set(ids).size).toBe(2);
    dialogs.forEach((dialog, index) => expect(dialog.contains(document.getElementById(ids[index]!))).toBe(true));
    rerender(view(false));
    expect(screen.getByRole("dialog", { name: "Review" })).toBe(dialogs[1]);
    expect(dialogs[1].getAttribute("aria-labelledby")).toBe(ids[1]);
  });

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
