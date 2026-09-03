// INPUT: Select/Action Menu 的触发器、选项、disabled 项与用户键盘/点击事件。
// OUTPUT: 证明 Portal 菜单的 ARIA、选择、遍历、关闭和焦点归还合同。
// POS: Menu pattern DOM 行为测试；定位数学和业务菜单内容分别由模型/feature 测试负责。

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useRef, useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { UiActionMenu } from "@/shared/ui/menu/action-menu";
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
});

describe("UiActionMenu", () => {
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
