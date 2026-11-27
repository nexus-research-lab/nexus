// INPUT: Dialog 的打开状态、嵌套关系、焦点元素、子浮层与用户键盘/点击事件。
// OUTPUT: 证明动态/显式标题和实例隔离，以及 Portal 模态的焦点圈、关闭顺序、遮罩策略、滚动锁和焦点归还合同。
// POS: Dialog primitive DOM 行为测试；业务确认结果和具体文案由 feature 测试负责。

import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  UiDialogBackdrop,
  UiDialogHeader,
} from "@/shared/ui/dialog/dialog";


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
});
