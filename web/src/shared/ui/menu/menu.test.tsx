// INPUT: Select/Action Menu 的触发器、选项、disabled 项与用户键盘/点击事件。
// OUTPUT: 证明 Portal 菜单的 ARIA、选择、遍历、关闭和焦点归还合同。
// POS: Menu pattern DOM 行为测试；定位数学和业务菜单内容分别由模型/feature 测试负责。

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useRef, useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { UiActionMenu } from "@/shared/ui/menu/action-menu";
import { UiMenuActionRow } from "@/shared/ui/menu/menu-action-row";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { SelectMenuOptionRow } from "@/shared/ui/menu/select-menu-primitives";

describe("UiSelectMenu", () => {
  it("owns reusable listbox option semantics and preserves consumer events", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    const onMouseDown = vi.fn((event: React.MouseEvent) => event.preventDefault());

    render(
      <div role="listbox" aria-label="命令">
        <SelectMenuOptionRow
          active
          className="min-h-8"
          onClick={onClick}
          onMouseDown={onMouseDown}
        >
          /goal
        </SelectMenuOptionRow>
      </div>,
    );

    const option = screen.getByRole("option", { name: "/goal" });
    expect(option.getAttribute("type")).toBe("button");
    expect(option.getAttribute("aria-selected")).toBe("true");
    expect(option.getAttribute("data-active")).toBe("true");
    expect(option.className).toContain("radius-control-lg");

    await user.click(option);
    expect(onMouseDown).toHaveBeenCalledTimes(1);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("opens a named listbox, selects an option, and returns focus", async () => {
    const user = userEvent.setup();

    function Harness() {
      const [value, setValue] = useState("alpha");
      return (
        <UiSelectMenu
          ariaLabel="选择模型"
          onChange={setValue}
          options={[
            { label: "Alpha", value: "alpha" },
            { disabled: true, label: "Beta", value: "beta" },
            { label: "Gamma", value: "gamma" },
          ]}
          value={value}
        />
      );
    }

    render(<Harness />);
    const trigger = screen.getByRole("button", { name: "选择模型" });
    await user.click(trigger);
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByRole("listbox", { name: "选择模型" })).toBeTruthy();
    expect((screen.getByRole("option", { name: "Beta" }) as HTMLButtonElement).disabled).toBe(true);

    await user.click(screen.getByRole("option", { name: "Gamma" }));
    expect(screen.queryByRole("listbox", { name: "选择模型" })).toBeNull();
    expect(trigger.textContent).toContain("Gamma");
    expect(document.activeElement).toBe(trigger);
  });

  it("skips disabled options with arrows and closes from the trigger with Escape", async () => {
    const user = userEvent.setup();

    function Harness() {
      const [value, setValue] = useState("alpha");
      return (
        <UiSelectMenu
          ariaLabel="运行模式"
          onChange={setValue}
          options={[
            { label: "Alpha", value: "alpha" },
            { disabled: true, label: "Beta", value: "beta" },
            { label: "Gamma", value: "gamma" },
          ]}
          value={value}
        />
      );
    }

    render(<Harness />);
    const trigger = screen.getByRole("button", { name: "运行模式" });
    trigger.focus();
    await user.keyboard("{ArrowDown}");
    expect(trigger.textContent).toContain("Gamma");
    expect(screen.getByRole("listbox", { name: "运行模式" })).toBeTruthy();

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("listbox", { name: "运行模式" })).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });

  it("closes an open selector when disabled and does not reopen after it becomes available", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const options = [{ label: "Alpha", value: "alpha" }];
    const view = render(
      <UiSelectMenu ariaLabel="保存中的模型" onChange={onChange} options={options} value="alpha" />,
    );
    const trigger = screen.getByRole("button", { name: "保存中的模型" });
    await user.click(trigger);
    expect(screen.getByRole("listbox", { name: "保存中的模型" })).toBeTruthy();

    view.rerender(
      <UiSelectMenu ariaLabel="保存中的模型" disabled onChange={onChange} options={options} value="alpha" />,
    );
    expect(screen.queryByRole("listbox", { name: "保存中的模型" })).toBeNull();
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(trigger.getAttribute("aria-controls")).toBeNull();
    expect(onChange).not.toHaveBeenCalled();

    view.rerender(
      <UiSelectMenu ariaLabel="保存中的模型" onChange={onChange} options={options} value="alpha" />,
    );
    expect(screen.queryByRole("listbox", { name: "保存中的模型" })).toBeNull();
    await user.click(trigger);
    expect(screen.getByRole("listbox", { name: "保存中的模型" })).toBeTruthy();
  });
});

describe("UiActionMenu", () => {
  it("focuses only after positioning makes the menu visible and preserves focus while repositioning", async () => {
    const user = userEvent.setup();
    const focusVisibility: string[] = [];
    const originalFocus = HTMLElement.prototype.focus;
    const focusSpy = vi.spyOn(HTMLElement.prototype, "focus").mockImplementation(function (
      this: HTMLElement,
      options?: FocusOptions,
    ) {
      if (this.getAttribute("role") === "menuitem") {
        // jsdom 允许聚焦 visibility:hidden；记录调用当刻的真实样式以保留浏览器约束。
        focusVisibility.push(window.getComputedStyle(this).visibility);
      }
      originalFocus.call(this, options);
    });
    function Harness() {
      const anchorRef = useRef<HTMLButtonElement>(null);
      const [isOpen, setIsOpen] = useState(false);
      return (
        <>
          <button ref={anchorRef} onClick={() => setIsOpen(true)} type="button">定位菜单</button>
          <UiActionMenu
            anchorRef={anchorRef}
            ariaLabel="定位后的动作"
            isOpen={isOpen}
            items={[{ label: "打开", value: "open" }, { label: "删除", value: "delete" }]}
            onClose={() => setIsOpen(false)}
            onSelect={() => undefined}
          />
        </>
      );
    }

    try {
      render(<Harness />);
      const anchor = screen.getByRole("button", { name: "定位菜单" });
      const anchorBounds = vi.spyOn(anchor, "getBoundingClientRect").mockReturnValue(
        new DOMRect(40, 40, 100, 32),
      );
      await user.click(anchor);
      await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "打开" })));
      expect(focusVisibility).toEqual(["visible"]);

      await user.keyboard("{ArrowDown}");
      const secondItem = screen.getByRole("menuitem", { name: "删除" });
      expect(document.activeElement).toBe(secondItem);
      anchorBounds.mockReturnValue(new DOMRect(80, 60, 100, 32));
      fireEvent.scroll(window);
      await waitFor(() => expect(screen.getByRole("menu", { name: "定位后的动作" }).style.left).toBe("80px"));
      expect(document.activeElement).toBe(secondItem);
      expect(focusVisibility).toEqual(["visible", "visible"]);
    } finally {
      focusSpy.mockRestore();
    }
  });

  it("owns native menu action rows and their state semantics", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();

    render(
      <div aria-label="文件操作" role="menu">
        <UiMenuActionRow active onClick={onClick} tone="danger">
          删除
        </UiMenuActionRow>
        <UiMenuActionRow disabled>不可用</UiMenuActionRow>
      </div>,
    );

    const action = screen.getByRole("menuitem", { name: "删除" });
    const disabledAction = screen.getByRole("menuitem", { name: "不可用" });
    expect(action.getAttribute("type")).toBe("button");
    expect(action.getAttribute("data-active")).toBe("true");
    expect(action.className).toContain("radius-control-lg");
    expect(action.className).toContain("text-(--destructive)");
    expect((disabledAction as HTMLButtonElement).disabled).toBe(true);
    expect(disabledAction.getAttribute("aria-disabled")).toBe("true");

    await user.click(action);
    await user.click(disabledAction);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("focuses and traverses enabled items, then restores the anchor", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();

    function Harness() {
      const anchorRef = useRef<HTMLButtonElement>(null);
      const [isOpen, setIsOpen] = useState(false);
      return (
        <>
          <button ref={anchorRef} onClick={() => setIsOpen(true)} type="button">
            更多操作
          </button>
          <UiActionMenu
            anchorRef={anchorRef}
            ariaLabel="会话操作"
            isOpen={isOpen}
            items={[
              { label: "打开", value: "open" },
              { disabled: true, label: "不可用", value: "disabled" },
              { label: "删除", tone: "danger", value: "delete" },
            ]}
            onClose={() => setIsOpen(false)}
            onSelect={onSelect}
          />
        </>
      );
    }

    render(<Harness />);
    const anchor = screen.getByRole("button", { name: "更多操作" });
    await user.click(anchor);
    const firstItem = await screen.findByRole("menuitem", { name: "打开" });
    await waitFor(() => expect(document.activeElement).toBe(firstItem));

    await user.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "删除" }));
    await user.keyboard("{Enter}");
    expect(onSelect).toHaveBeenCalledWith("delete");
    expect(screen.queryByRole("menu", { name: "会话操作" })).toBeNull();
    expect(document.activeElement).toBe(anchor);

    await user.click(anchor);
    await screen.findByRole("menu", { name: "会话操作" });
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menu", { name: "会话操作" })).toBeNull();
    expect(document.activeElement).toBe(anchor);
  });
});
