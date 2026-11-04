// INPUT: 列表行、嵌套动作、分组名称与用户键盘/点击事件。
// OUTPUT: 行启停、次动作隔离与可访问分组名称。
// POS: 列表交互的唯一测试入口；按钮通用合同由 Button 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";

import { UiListActionButton } from "@/shared/ui/list/list-action";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiListSectionDivider } from "@/shared/ui/list/list-section-divider";

describe("UiListRow", () => {
  it("keeps a muted row actionable while disabled preserves semantics and blocks activation", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const { rerender } = render(<UiListRow muted onClick={onOpen} title="正在刷新" variant="outlined" />);
    const row = screen.getByRole("button", { name: "正在刷新" });
    expect(row.getAttribute("tabindex")).toBe("0");
    await user.click(row);
    await user.keyboard("{Enter} ");
    expect(onOpen).toHaveBeenCalledTimes(3);
    rerender(<UiListRow disabled muted onClick={onOpen} title="正在刷新" variant="outlined" />);
    expect(row.getAttribute("role")).toBe("button");
    expect(row.getAttribute("aria-disabled")).toBe("true");
    expect(row.hasAttribute("tabindex")).toBe(false);
    await user.click(row);
    await user.keyboard("{Enter} ");
    expect(onOpen).toHaveBeenCalledTimes(3);
  });

  it("does not turn static content into a focus target", () => {
    render(
      <UiListRow
        data-testid="static-row"
        description="最近更新"
        title="只读项目"
      />,
    );
    const row = screen.getByTestId("static-row");
    expect(row.hasAttribute("role")).toBe(false);
    expect(row.hasAttribute("tabindex")).toBe(false);

  });

});

describe("UiListActionButton", () => {
  it("keeps pointer and keyboard secondary actions separate from the row command", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const onDelete = vi.fn();
    const ref = createRef<HTMLButtonElement>();
    render(
      <form>
        <UiListRow
          actions={(
            <UiListActionButton
              aria-label="删除项目"
              onClick={onDelete}
              ref={ref}
              stopPropagation
              tone="danger"
            >
              删除
            </UiListActionButton>
          )}
          onClick={onOpen}
          title="项目 Alpha"
        />
      </form>,
    );

    const action = screen.getByRole("button", { name: "删除项目" });
    expect(ref.current).toBe(action);
    await user.click(action);
    action.focus();
    await user.keyboard("{Enter}");
    expect(onDelete).toHaveBeenCalledTimes(2);
    expect(onOpen).not.toHaveBeenCalled();

  });
});

describe("UiListSectionDivider", () => {
  it("uses visible group names by default and preserves explicit naming across updates", () => {
    const { rerender } = render(<>
      <UiListSectionDivider label="Active conversations" />
      <UiListSectionDivider aria-label="Pinned" label="Current selection" />
    </>);
    const named = screen.getByRole("separator", { name: "Active conversations" });
    expect(named.getAttribute("aria-orientation")).toBe("horizontal");
    expect(named.getAttribute("aria-labelledby")).toBe(screen.getByText("Active conversations").id);
    expect(screen.getByRole("separator", { name: "Pinned" }).hasAttribute("aria-labelledby")).toBe(false);
    rerender(<>
      <span id="external-group-name">Saved conversations</span>
      <UiListSectionDivider aria-labelledby="external-group-name" label="Archived" />
      <UiListSectionDivider />
    </>);
    expect(screen.getByRole("separator", { name: "Saved conversations" })).toBeTruthy();
    const unnamed = screen.getByRole("separator", { name: "" });
    expect(unnamed.hasAttribute("aria-labelledby")).toBe(false);
  });

});
