// INPUT: Agent Options 两档动作行、可执行状态、成功确认与待恢复失败反馈。
// OUTPUT: 同档按钮的独立键盘命令、禁用保存和完整单一状态播报。
// POS: Agent Options 动作行 DOM 合同；保存事务和失败分类由 editor 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AgentOptionsEditorActions } from "./agent-options-editor-actions";

const SAVE_ACTION = {
  enabled: true,
  label: "保存",
  run: vi.fn(),
};

describe("AgentOptionsEditorActions", () => {
  it.each([
    ["warning", "warning"],
    ["error", "danger"],
  ] as const)("projects %s save feedback through the shared %s notice", (
    feedbackTone,
    noticeTone,
  ) => {
    render(
      <AgentOptionsEditorActions
        deleteAction={null}
        feedback={{
          blocksRepeat: true,
          impact: "已有 Agent 设置仍然保留",
          title: "保存结果需要确认",
          tone: feedbackTone,
        }}
        saveAction={SAVE_ACTION}
        buttonSize="sm"
      />,
    );

    const notice = screen.getByRole("status");
    expect(notice.getAttribute("data-inline-notice-tone")).toBe(noticeTone);
    expect(notice.getAttribute("data-inline-notice-width")).toBe("full");
    expect(notice.className).toContain("order-first");
    expect(notice.textContent).toContain("已有 Agent 设置仍然保留");
  });

  it("announces a complete successful save without a second notice, tooltip or focus move", async () => {
    const user = userEvent.setup();
    const message = "The Agent settings were saved successfully, including the updated behavior template and all selected tools.";
    const props = { buttonSize: "sm" as const, deleteAction: null, saveAction: SAVE_ACTION };
    const rendered = render(<AgentOptionsEditorActions {...props} feedback={null} />);
    const save = screen.getByRole("button", { name: "保存" });
    await user.tab();
    expect(document.activeElement).toBe(save);
    rendered.rerender(<AgentOptionsEditorActions {...props} feedback={{ message, tone: "success" }} />);
    const status = screen.getByRole("status");
    expect(status.textContent).toBe(message);
    expect(status.getAttribute("aria-atomic")).toBe("true");
    expect(status.hasAttribute("title")).toBe(false);
    expect(status.hasAttribute("data-inline-notice-tone")).toBe(false);
    expect(status.className).not.toContain("truncate");
    expect(document.activeElement).toBe(save);
    rendered.rerender(<AgentOptionsEditorActions {...props} feedback={null} />);
    expect(screen.queryByRole("status")).toBeNull();
  });

  it.each(["sm", "md"] as const)("keeps %s actions independent and only enables executable saving", async (buttonSize) => {
    const user = userEvent.setup();
    const remove = vi.fn();
    const cancel = vi.fn();
    const save = vi.fn();
    const props = {
      buttonSize, deleteAction: { label: "Delete", run: remove }, cancelAction: { label: "Cancel", run: cancel },
      feedback: null, saveAction: { enabled: false, label: "Save", run: save },
    };
    const rendered = render(<AgentOptionsEditorActions {...props} />);
    const saveButton = screen.getByRole("button", { name: "Save" });
    expect(saveButton.hasAttribute("disabled")).toBe(true);
    await user.click(saveButton);
    expect(save).not.toHaveBeenCalled();
    await user.tab();
    await user.keyboard("{Enter}");
    expect(remove).toHaveBeenCalledOnce();
    await user.tab();
    await user.keyboard("{Enter}");
    expect(cancel).toHaveBeenCalledOnce();
    expect(save).not.toHaveBeenCalled();
    rendered.rerender(<AgentOptionsEditorActions {...props} saveAction={{ ...props.saveAction, enabled: true }} />);
    await user.tab();
    expect(document.activeElement).toBe(saveButton);
    await user.keyboard("{Enter}");
    expect(save).toHaveBeenCalledOnce();
    expect(remove).toHaveBeenCalledOnce();
    expect(cancel).toHaveBeenCalledOnce();
  });

  it("omits absent optional actions and keeps the sole save command", async () => {
    const run = vi.fn();
    render(
      <AgentOptionsEditorActions
        deleteAction={null}
        feedback={null}
        saveAction={{ ...SAVE_ACTION, run }}
        buttonSize="sm"
      />,
    );
    expect(screen.getAllByRole("button")).toHaveLength(1);
    await userEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(run).toHaveBeenCalledOnce();
    expect(screen.queryByRole("status")).toBeNull();
  });
});
