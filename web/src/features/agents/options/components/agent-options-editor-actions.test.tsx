// INPUT: Agent Options 保存动作、成功确认与待恢复失败反馈。
// OUTPUT: 证明异常反馈复用共享行内提示，成功确认仍保持为轻量邻接文本。
// POS: Agent Options 动作行 DOM 合同；保存事务和失败分类由 editor 测试负责。

import { render, screen } from "@testing-library/react";
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
        saveButtonSize="sm"
      />,
    );

    const notice = screen.getByRole("status");
    expect(notice.getAttribute("data-inline-notice-tone")).toBe(noticeTone);
    expect(notice.getAttribute("data-inline-notice-width")).toBe("full");
    expect(notice.className).toContain("order-first");
    expect(notice.textContent).toContain("已有 Agent 设置仍然保留");
  });

  it("keeps successful save confirmation as a lightweight adjacent label", () => {
    render(
      <AgentOptionsEditorActions
        deleteAction={null}
        feedback={{ message: "已保存", tone: "success" }}
        saveAction={SAVE_ACTION}
        saveButtonSize="sm"
      />,
    );

    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.getByText("已保存").className).toContain("text-(--success)");
  });
});
