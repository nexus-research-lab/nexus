// INPUT: 单行 Prompt 的默认值、确认/取消动作与共享 i18n 上下文。
// OUTPUT: 证明紧凑尺寸、共享控件样式、键盘提交与关闭行为保持一致。
// POS: Decision Dialog DOM 合同；Workspace 等业务调用方只提供文案和命令。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { PromptDialog } from "@/shared/ui/dialog/decision/decision-dialog";


beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue(
    [{} as DOMRect] as unknown as DOMRectList,
  );
});

afterEach(() => {
  vi.restoreAllMocks();
  document.body.style.overflow = "";
});

describe("PromptDialog", () => {

  it("associates caller validation with the field and locks all exit/submit paths while busy", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    const view = (busy: boolean, error?: string) => <I18nProvider><PromptDialog
      busy={busy} cancelText="Discard" confirmText="Create" defaultValue="example" error={error}
      inputLabel="Folder name" isOpen onCancel={onCancel} onConfirm={onConfirm} title="Create folder"
    /></I18nProvider>;
    const { rerender } = render(view(false, "Use a valid folder name."));
    const input = screen.getByRole("textbox", { name: "Folder name" }) as HTMLInputElement;
    expect(input.getAttribute("aria-invalid")).toBe("true");
    expect(input.getAttribute("aria-errormessage")).toBe(screen.getByRole("alert").id);
    rerender(view(true));
    expect(screen.queryByRole("alert")).toBeNull();
    expect(input.hasAttribute("aria-errormessage")).toBe(false);
    expect(input.disabled).toBe(true);
    for (const button of screen.getAllByRole("button")) expect((button as HTMLButtonElement).disabled).toBe(true);
    await user.click(screen.getByRole("button", { name: "Create" }));
    fireEvent.keyDown(input, { key: "Enter" });
    fireEvent.keyDown(document, { key: "Escape" });
    await user.click(screen.getByRole("dialog"));
    expect(onConfirm).not.toHaveBeenCalled();
    expect(onCancel).not.toHaveBeenCalled();
    rerender(view(false));
    await user.click(screen.getByRole("button", { name: "Create" }));
    expect(onConfirm).toHaveBeenCalledExactlyOnceWith("example");
  });
});
