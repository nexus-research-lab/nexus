// INPUT: 受控菜单开关、原生按钮属性、ref 与上游点击/键盘动作。
// OUTPUT: 证明共享触发器保留原生表单安全、ARIA 关联、焦点和 disabled 语义。
// POS: SelectMenuTrigger DOM 行为合同；浮层开关及布局仍由菜单集成测试验证。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createRef, useRef, useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { SelectMenuTrigger, SelectMenuTriggerContent } from "./select-menu-primitives";
import { getSelectMenuSizeConfig } from "./select-menu-styles";

const styles = getSelectMenuSizeConfig("md");

describe("SelectMenuTrigger", () => {
  it("delegates click and keyboard commands without submitting its parent form", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault());
    const onKeyDown = vi.fn();

    function Harness() {
      const buttonRef = useRef<HTMLButtonElement>(null);
      const [isOpen, setIsOpen] = useState(false);
      return (
        <form onSubmit={onSubmit}>
          <SelectMenuTrigger
            ariaLabel="选择模型"
            buttonRef={buttonRef}
            disabled={false}
            isOpen={isOpen}
            menuId="trigger-options"
            onClick={() => setIsOpen((current) => !current)}
            onKeyDown={onKeyDown}
            styles={styles}
            surface="dialog"
          >
            <SelectMenuTriggerContent isOpen={isOpen}>当前模型</SelectMenuTriggerContent>
          </SelectMenuTrigger>
          {isOpen ? <div aria-label="选择模型" id="trigger-options" role="listbox" /> : null}
        </form>
      );
    }

    render(<Harness />);
    const trigger = screen.getByRole("button", { name: "选择模型" });
    expect(trigger.getAttribute("type")).toBe("button");
    expect(trigger.getAttribute("aria-haspopup")).toBe("listbox");
    expect(trigger.getAttribute("aria-disabled")).toBe("false");
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(trigger.hasAttribute("aria-controls")).toBe(false);
    await user.tab();
    expect(document.activeElement).toBe(trigger);
    await user.keyboard("{ArrowDown}");
    expect(onKeyDown).toHaveBeenCalledOnce();
    expect(onKeyDown.mock.calls[0][0].key).toBe("ArrowDown");
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    await user.keyboard("{Enter}");
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    expect(trigger.getAttribute("aria-controls")).toBe(screen.getByRole("listbox").id);
    expect(onSubmit).not.toHaveBeenCalled();
    await user.click(trigger);
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(trigger.hasAttribute("aria-controls")).toBe(false);
  });

  it("forwards native identity and ref while disabled actions stay outside focus and click flows", async () => {
    const user = userEvent.setup();
    const buttonRef = createRef<HTMLButtonElement>();
    const onClick = vi.fn();
    render(
      <>
        <SelectMenuTrigger
          ariaLabel="Room 技能"
          buttonRef={buttonRef}
          disabled
          id="room-skills-field"
          isOpen={false}
          menuId="room-skills-options"
          onClick={onClick}
          styles={styles}
          surface="dialog"
          title="研究、写作"
        >
          研究、写作
        </SelectMenuTrigger>
        <input aria-label="下一字段" />
      </>,
    );

    const trigger = screen.getByRole("button", { name: "Room 技能" });
    expect(buttonRef.current).toBe(trigger);
    expect(trigger.id).toBe("room-skills-field");
    expect(trigger.getAttribute("title")).toBe("研究、写作");
    expect(trigger.getAttribute("aria-disabled")).toBe("true");
    expect((trigger as HTMLButtonElement).disabled).toBe(true);
    await user.click(trigger);
    expect(onClick).not.toHaveBeenCalled();
    await user.tab();
    expect(document.activeElement).toBe(screen.getByRole("textbox", { name: "下一字段" }));
  });
});
