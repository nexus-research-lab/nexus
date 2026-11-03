// INPUT: ListRow/ListContent/ListAction/SectionDivider 的内容、交互开关和键盘事件。
// OUTPUT: 证明行级语义、复用内容/分组布局、Enter/Space 激活、事件阻断和安全默认 type。
// POS: List primitives DOM 行为测试；业务选择、路由与删除事务由消费者负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
    await user.click(row);
    await user.keyboard("{Enter}");
    expect(onOpen).toHaveBeenCalledTimes(2);
    rerender(<UiListRow disabled muted onClick={onOpen} title="正在刷新" variant="outlined" />);
    expect(row.getAttribute("aria-disabled")).toBe("true");
    await user.click(row);
    await user.keyboard("{Enter} ");
    expect(onOpen).toHaveBeenCalledTimes(2);
  });

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

  it("provides a shared dense row for 40px dialog and sidebar lists", () => {
    render(
      <UiListRow
        data-testid="dense-row"
        density="dense"
        onClick={() => undefined}
        title="记忆条目"
        tooltip="memory/note.md"
      />,
    );

    const row = screen.getByTestId("dense-row");

    expect(row.getAttribute("title")).toBeNull();
  });

  it("keeps disabled actions semantic and inert", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    render(
      <UiListRow
        data-testid="disabled-row"
        disabled
        onClick={onOpen}
        title="正在启动"
      />,
    );

    const row = screen.getByTestId("disabled-row");
    expect(row.getAttribute("role")).toBe("button");
    expect(row.getAttribute("aria-disabled")).toBe("true");
    expect(row.hasAttribute("tabindex")).toBe(false);

    await user.click(row);
    expect(onOpen).not.toHaveBeenCalled();
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
        <UiListActionButton aria-label="确认修改" tone="primary">
          确认
        </UiListActionButton>
      </form>,
    );

    const action = screen.getByRole("button", { name: "删除项目" });
    expect(action.getAttribute("type")).toBe("button");
    await user.click(action);
    expect(onDelete).toHaveBeenCalledOnce();
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

  it("owns the labeled horizontal separation between list groups", () => {
    render(<UiListSectionDivider aria-label="IM" label="IM" />);
    const divider = screen.getByRole("separator", { name: "IM" });

    expect(divider.getAttribute("aria-orientation")).toBe("horizontal");

  });
});
