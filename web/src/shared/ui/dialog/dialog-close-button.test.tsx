// INPUT: Dialog 关闭动作的禁用态、可访问名称及指针/键盘事件。
// OUTPUT: 证明复用 IconButton 后关闭仍隔离父级事件、不提交表单且不增加 Tooltip。
// POS: Dialog close 组合行为测试；完整模态生命周期由 dialog.test.tsx 覆盖。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { UiDialogCloseButton } from "@/shared/ui/dialog/dialog";

describe("UiDialogCloseButton", () => {
  it("closes once per pointer or keyboard action without bubbling or adding a tooltip", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onParentClick = vi.fn();
    const onParentPointerDown = vi.fn();
    render(
      <div onClick={onParentClick} onPointerDown={onParentPointerDown} role="presentation">
        <UiDialogCloseButton ariaLabel="关闭设置" onClose={onClose} />
      </div>,
      { wrapper: I18nProvider },
    );

    const button = screen.getByRole("button", { name: "关闭设置" });
    expect(button.getAttribute("type")).toBe("button");
    expect(button.getAttribute("title")).toBeNull();
    await user.click(button);
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("tooltip")).toBeNull();
    await user.keyboard("{Enter}");
    expect(onClose).toHaveBeenCalledTimes(2);
    await user.keyboard(" ");
    expect(onClose).toHaveBeenCalledTimes(3);
    expect(onParentClick).not.toHaveBeenCalled();
    expect(onParentPointerDown).not.toHaveBeenCalled();
    expect(fireEvent.click(button)).toBe(false);
    expect(onClose).toHaveBeenCalledTimes(4);
  });

  it("keeps a disabled close action inert and outside keyboard focus", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault());
    render(
      <form onSubmit={onSubmit}>
        <UiDialogCloseButton ariaLabel="关闭设置" disabled onClose={onClose} />
        <button type="button">继续编辑</button>
      </form>,
      { wrapper: I18nProvider },
    );

    await user.click(screen.getByRole("button", { name: "关闭设置" }));
    await user.tab();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "继续编辑" }));
    expect(onClose).not.toHaveBeenCalled();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
