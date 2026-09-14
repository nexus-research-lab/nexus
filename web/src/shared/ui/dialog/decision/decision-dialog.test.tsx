// INPUT: 单行 Prompt 的默认值、确认/取消动作与共享 i18n 上下文。
// OUTPUT: 证明紧凑尺寸、共享控件样式、键盘提交与关闭行为保持一致。
// POS: Decision Dialog DOM 合同；Workspace 等业务调用方只提供文案和命令。

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { MESSAGES } from "@/shared/i18n/messages";
import { ConfirmDialog, PromptDialog } from "@/shared/ui/dialog/decision/decision-dialog";

function renderPrompt(onCancel = vi.fn(), onConfirm = vi.fn()) {
  return {
    onCancel,
    onConfirm,
    ...render(
      <I18N_CONTEXT.Provider
        value={{
          locale: "zh",
          setLocale: vi.fn(),
          t: (key) => MESSAGES.zh[key],
        }}
      >
        <PromptDialog
          defaultValue="new-folder"
          isOpen
          onCancel={onCancel}
          onConfirm={onConfirm}
          placeholder="例如：new-folder"
          title="新建文件夹"
        />
      </I18N_CONTEXT.Provider>,
    ),
  };
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

describe("PromptDialog", () => {
  it.each([false, true])("leaves IME events to the %s multiline input and preserves its submit shortcut", async (multiline) => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(<I18nProvider><PromptDialog defaultValue="draft" isOpen multiline={multiline} onCancel={onCancel} onConfirm={onConfirm} title="Edit instruction" /></I18nProvider>);
    const input = screen.getByRole("textbox");
    await waitFor(() => expect(document.activeElement).toBe(input));
    fireEvent.keyDown(input, { key: "Enter", ctrlKey: multiline, isComposing: true });
    fireEvent.keyDown(input, { key: "Enter", metaKey: multiline, keyCode: 229 });
    fireEvent.keyDown(input, { key: "Escape", isComposing: true });
    expect(onConfirm).not.toHaveBeenCalled();
    expect(onCancel).not.toHaveBeenCalled();
    await user.clear(input);
    await user.type(input, multiline ? "first{Enter}second" : "name");
    expect(onConfirm).not.toHaveBeenCalled();
    await user.keyboard(multiline ? "{Control>}{Enter}{/Control}" : "{Enter}");
    expect(onConfirm).toHaveBeenCalledExactlyOnceWith(multiline ? "first\nsecond" : "name");
  });

  it.each(["en", "zh"] as const)("localizes default decisions and describes the input in %s without naming it from an example", (locale) => {
    const wrapper = ({ children }: { children: React.ReactNode }) => <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>{children}</I18N_CONTEXT.Provider>;
    const view = render(<PromptDialog isOpen multiline message="Describe the next step." placeholder="Example only" title="Task instruction" onCancel={vi.fn()} onConfirm={vi.fn()} />, { wrapper });
    const input = screen.getByRole("textbox", { name: "Task instruction" });
    const description = input.getAttribute("aria-describedby");
    expect(description).toBeTruthy();
    expect(document.getElementById(description!)?.textContent).toContain("Cmd/Ctrl + Enter");
    expect(screen.getByRole("button", { name: locale === "en" ? "Cancel" : "取消" })).toBeTruthy();
    expect(screen.getByRole("button", { name: locale === "en" ? "Confirm" : "确认" })).toBeTruthy();
    view.rerender(<ConfirmDialog isOpen title="Decision" message="Continue?" onCancel={vi.fn()} onConfirm={vi.fn()} />);
    expect(screen.getByRole("button", { name: locale === "en" ? "Cancel" : "取消" })).toBeTruthy();
    expect(screen.getByRole("button", { name: locale === "en" ? "Confirm" : "确认" })).toBeTruthy();
  });

  it("uses the compact decision geometry and shared input/action recipes", async () => {
    const user = userEvent.setup();
    const { onConfirm } = renderPrompt();
    const dialog = screen.getByRole("dialog", { name: "新建文件夹" });
    const input = screen.getByRole("textbox", { name: "新建文件夹" });
    const confirm = screen.getByRole("button", { name: "确认" });

    expect(dialog.querySelector(".max-w-sm")).toBeTruthy();
    expect(input.className).toContain("dialog-input");
    expect(confirm.className).toContain("bg-(--button-primary-background)");
    await waitFor(() => expect(document.activeElement).toBe(input));

    await user.clear(input);
    await user.type(input, "docs{Enter}");
    expect(onConfirm).toHaveBeenCalledWith("docs");
  });

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
