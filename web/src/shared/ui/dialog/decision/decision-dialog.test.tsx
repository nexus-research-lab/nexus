// INPUT: 单行 Prompt 的默认值、确认/取消动作与共享 i18n 上下文。
// OUTPUT: 证明紧凑尺寸、共享控件样式、键盘提交与关闭行为保持一致。
// POS: Decision Dialog DOM 合同；Workspace 等业务调用方只提供文案和命令。

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { PromptDialog } from "@/shared/ui/dialog/decision/decision-dialog";

function renderPrompt(onCancel = vi.fn(), onConfirm = vi.fn()) {
  return {
    onCancel,
    onConfirm,
    ...render(
      <I18N_CONTEXT.Provider
        value={{
          locale: "zh",
          setLocale: vi.fn(),
          t: (key) => key === "common.close" ? "关闭" : key,
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
  it("uses the compact decision geometry and shared input/action recipes", async () => {
    const user = userEvent.setup();
    const { onConfirm } = renderPrompt();
    const dialog = screen.getByRole("dialog", { name: "新建文件夹" });
    const input = screen.getByRole("textbox", { name: "例如：new-folder" });
    const confirm = screen.getByRole("button", { name: "确认" });

    expect(dialog.querySelector(".max-w-sm")).toBeTruthy();
    expect(input.className).toContain("dialog-input");
    expect(confirm.className).toContain("bg-(--button-primary-background)");
    await waitFor(() => expect(document.activeElement).toBe(input));

    await user.clear(input);
    await user.type(input, "docs{Enter}");
    expect(onConfirm).toHaveBeenCalledWith("docs");
  });
});
