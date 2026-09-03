// INPUT: ListRow/ListAction 的交互开关、嵌套动作和键盘事件。
// OUTPUT: 证明行级按钮语义、Enter/Space 激活、事件阻断和安全默认 type。
// POS: List primitives DOM 行为测试；业务选择、路由与删除事务由消费者负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UiListActionButton } from "@/shared/ui/list/list-action";
import { UiListRow } from "@/shared/ui/list/list-row";

describe("UiListRow", () => {
  it("becomes one keyboard-operable row only when it has an action", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    render(
      <UiListRow
        data-testid="interactive-row"
        description="最近更新"
        onClick={onOpen}
        title="项目 Alpha"
      />,
    );

    const row = screen.getByTestId("interactive-row");
    expect(row.getAttribute("role")).toBe("button");
    expect(row.getAttribute("tabindex")).toBe("0");
    row.focus();
    await user.keyboard("{Enter} ");
    expect(onOpen).toHaveBeenCalledTimes(2);
  });

  it("does not turn static content into a focus target", () => {
    render(<UiListRow data-testid="static-row" title="只读项目" />);
    const row = screen.getByTestId("static-row");
    expect(row.hasAttribute("role")).toBe(false);
    expect(row.hasAttribute("tabindex")).toBe(false);
  });

  it("keeps compact geometry inside the shared density contract", () => {
    render(
      <UiListRow
        data-testid="compact-row"
        density="compact"
        onClick={() => undefined}
        title="紧凑会话"
      />,
    );

    const row = screen.getByTestId("compact-row");
    expect(row.className).toContain("min-h-12");
    expect(row.className).toContain("radius-control-md");
    expect(row.className).not.toContain("min-h-[64px]");
  });
});

describe("UiListActionButton", () => {
  it("can stop a nested action from activating its parent row", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const onDelete = vi.fn();
    render(
      <form>
        <UiListRow
          actions={(
            <UiListActionButton
              aria-label="删除项目"
              onClick={onDelete}
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
    expect(action.getAttribute("type")).toBe("button");
    await user.click(action);
    expect(onDelete).toHaveBeenCalledOnce();
    expect(onOpen).not.toHaveBeenCalled();
  });
});
