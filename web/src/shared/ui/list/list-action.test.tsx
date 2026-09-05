// INPUT: 行内次动作的点击、键盘、ref、disabled 与事件隔离策略。
// OUTPUT: 证明 ListAction 复用原生按钮合同且不会误触发行级命令。
// POS: ListAction 组合行为测试；hover/focus 可见性由浏览器验收。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";

import { UiListActionButton } from "./list-action";
import { UiListRow } from "./list-row";

describe("UiListActionButton", () => {
  it("forwards its native ref and isolates pointer and keyboard actions", async () => {
    const user = userEvent.setup();
    const onRow = vi.fn();
    const onAction = vi.fn();
    const ref = createRef<HTMLButtonElement>();
    render(
      <UiListRow
        actions={(
          <UiListActionButton onClick={onAction} ref={ref} stopPropagation title="编辑条目">
            <span aria-hidden="true">✎</span>
          </UiListActionButton>
        )}
        onClick={onRow}
        title="行主动作"
      />,
    );
    const action = screen.getByRole("button", { name: "编辑条目" });
    expect(ref.current).toBe(action);
    expect(action.getAttribute("type")).toBe("button");
    await user.click(action);
    action.focus();
    await user.keyboard("{Enter}");
    expect(onAction).toHaveBeenCalledTimes(2);
    expect(onRow).not.toHaveBeenCalled();
  });

  it("preserves disabled and explicit form submission semantics", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault());
    render(
      <form onSubmit={onSubmit}>
        <UiListActionButton aria-label="未就绪" disabled type="submit">×</UiListActionButton>
        <UiListActionButton aria-label="保存" type="submit">✓</UiListActionButton>
      </form>,
    );
    await user.click(screen.getByRole("button", { name: "未就绪" }));
    expect(onSubmit).not.toHaveBeenCalled();
    await user.tab();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "保存" }));
    await user.keyboard("{Enter}");
    expect(onSubmit).toHaveBeenCalledOnce();
  });
});
