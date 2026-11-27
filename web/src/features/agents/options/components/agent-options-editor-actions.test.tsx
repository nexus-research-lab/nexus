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

    expect(notice.textContent).toContain("已有 Agent 设置仍然保留");
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
});
