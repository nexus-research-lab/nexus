// INPUT: 组合菜单、嵌套菜单、输入框、禁用条目与键盘事件。
// OUTPUT: 证明菜单导航只遍历当前层级，保留输入法/输入控件和已处理事件。
// POS: 菜单共享键盘的 DOM 行为测试；浮层关闭及实际业务选择由消费者测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createPortal } from "react-dom";
import { describe, expect, it, vi } from "vitest";

import { UiMenuActionRow } from "./menu-action-row";
import { handleMenuKeyDown } from "./menu-keyboard";

describe("menu keyboard ownership", () => {
  it("traverses only the focused menu and skips disabled fieldset descendants", async () => {
    const user = userEvent.setup();
    render(<div role="menu" tabIndex={-1} onKeyDown={handleMenuKeyDown}>
      <UiMenuActionRow>主项</UiMenuActionRow>
      <fieldset disabled><UiMenuActionRow>字段禁用</UiMenuActionRow></fieldset>
      <UiMenuActionRow disabled>项禁用</UiMenuActionRow>
      <div role="menu">
        <UiMenuActionRow>子项一</UiMenuActionRow>
        <UiMenuActionRow>子项二</UiMenuActionRow>
      </div>
      <UiMenuActionRow>底部动作</UiMenuActionRow>
    </div>);
    screen.getByRole("menuitem", { name: "主项" }).focus();
    await user.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "底部动作" }));
    await user.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "主项" }));
    screen.getByRole("menuitem", { name: "子项一" }).focus();
    await user.keyboard("{End}{ArrowUp}");
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "子项一" }));
    await user.keyboard("{ArrowUp}");
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "子项二" }));
  });

  it("leaves IME, editable controls and consumed events with their owner", () => {
    const onExit = vi.fn();
    render(<div role="menu" tabIndex={-1} onKeyDown={(event) => handleMenuKeyDown(event, onExit)}>
      <UiMenuActionRow onKeyDown={(event) => { if (event.key === "Home") event.preventDefault(); }}>当前</UiMenuActionRow>
      <UiMenuActionRow>下一项</UiMenuActionRow>
      <input aria-label="查询" />
    </div>);
    const current = screen.getByRole("menuitem", { name: "当前" });
    current.focus();
    for (const flags of [{ isComposing: true }, { keyCode: 229 }, { which: 229 }]) {
      fireEvent.keyDown(current, { key: "ArrowDown", ...flags });
      fireEvent.keyDown(current, { key: "Tab", ...flags });
      expect(document.activeElement).toBe(current);
    }
    expect(fireEvent.keyDown(current, { key: "Home" })).toBe(false);
    expect(document.activeElement).toBe(current);
    expect(onExit).not.toHaveBeenCalled();
    const input = screen.getByRole("textbox");
    input.focus();
    expect(fireEvent.keyDown(input, { key: "End" })).toBe(true);
    expect(document.activeElement).toBe(input);
  });

  it("does not dismiss its parent through React portal bubbling", () => {
    const onExit = vi.fn();
    render(<div role="menu" tabIndex={-1} onKeyDown={(event) => handleMenuKeyDown(event, onExit)}>
      <UiMenuActionRow>父项</UiMenuActionRow>
      {createPortal(<div role="menu"><UiMenuActionRow>外部浮层</UiMenuActionRow></div>, document.body)}
    </div>);
    const child = screen.getByRole("menuitem", { name: "外部浮层" });
    child.focus();
    expect(fireEvent.keyDown(child, { key: "Tab" })).toBe(true);
    expect(onExit).not.toHaveBeenCalled();
    expect(document.activeElement).toBe(child);
  });
});
