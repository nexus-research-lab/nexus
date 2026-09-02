// INPUT: Field、SearchInput、Checkbox、Choice 与 SegmentedControl 的用户事件。
// OUTPUT: 证明校验、清除、布尔切换与互斥选择使用真实 DOM/ARIA 合同。
// POS: 表单原语交互测试；业务草稿和网络提交由各 feature 测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { UiCheckbox } from "@/shared/ui/form/checkbox";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import {
  UiField,
  UiInput,
  UiSearchInput,
} from "@/shared/ui/form/form-control";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";

function renderWithI18n(children: ReactNode) {
  const messages: Record<string, string> = {
    "common.clear": "清除",
    "common.invalid_field": "字段格式不正确",
    "common.required_field": "请填写此字段",
  };
  return render(
    <I18N_CONTEXT.Provider
      value={{
        locale: "zh",
        setLocale: vi.fn(),
        t: (key) => messages[key] ?? key,
      }}
    >
      {children}
    </I18N_CONTEXT.Provider>,
  );
}

describe("form primitives", () => {
  it("projects native required validation into one accessible field error", async () => {
    const user = userEvent.setup();
    renderWithI18n(
      <form>
        <UiField htmlFor="agent-name" label="名称" required>
          <UiInput id="agent-name" required />
        </UiField>
        <button type="submit">提交</button>
      </form>,
    );

    const input = screen.getByRole("textbox", { name: "名称" });
    fireEvent.invalid(input);
    expect(input.getAttribute("aria-invalid")).toBe("true");
    expect(screen.getByRole("alert").textContent).toBe("请填写此字段");

    await user.type(input, "Nexus");
    expect(input.hasAttribute("aria-invalid")).toBe(false);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("gives search a name and clears through the shared icon action", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const { container } = renderWithI18n(
      <UiSearchInput onChange={onChange} placeholder="搜索 Agent" value="writer" />,
    );

    expect(container.firstElementChild?.tagName).toBe("DIV");
    expect(screen.getByRole("searchbox", { name: "搜索 Agent" })).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "清除" }));
    expect(onChange).toHaveBeenCalledWith("");
  });

  it("keeps checkbox native semantics while sharing size and disabled states", async () => {
    const user = userEvent.setup();

    function Harness() {
      const [checked, setChecked] = useState(false);
      return (
        <>
          <label htmlFor="enabled-checkbox">
            <UiCheckbox
              checked={checked}
              checkboxSize="small"
              id="enabled-checkbox"
              onChange={(event) => setChecked(event.target.checked)}
            />
            启用
          </label>
          <label htmlFor="disabled-checkbox">
            <UiCheckbox disabled id="disabled-checkbox" />
            不可用
          </label>
        </>
      );
    }

    render(<Harness />);
    const enabled = screen.getByRole("checkbox", { name: "启用" }) as HTMLInputElement;
    await user.tab();
    expect(document.activeElement).toBe(enabled);
    await user.keyboard(" ");
    expect(enabled.checked).toBe(true);
    expect((screen.getByRole("checkbox", { name: "不可用" }) as HTMLInputElement).disabled).toBe(true);
  });

  it("exposes pressed state for choice and segmented selections", async () => {
    const user = userEvent.setup();
    const onChoice = vi.fn();
    const onSegment = vi.fn();
    render(
      <>
        <UiChoiceButton active onClick={onChoice} variant="picker">当前来源</UiChoiceButton>
        <UiSegmentedControl
          onChange={onSegment}
          options={[
            { label: "一次", value: "once" },
            { label: "重复", value: "recurring" },
          ]}
          title="执行频率"
          value="once"
        />
      </>,
    );

    const choice = screen.getByRole("button", { name: "当前来源" });
    expect(choice.getAttribute("aria-pressed")).toBe("true");
    expect(choice.className.includes("shadow-[")).toBe(false);
    await user.click(choice);
    expect(onChoice).toHaveBeenCalledTimes(1);

    const once = screen.getByRole("button", { name: "一次" });
    const recurring = screen.getByRole("button", { name: "重复" });
    expect(once.getAttribute("aria-pressed")).toBe("true");
    expect(recurring.getAttribute("aria-pressed")).toBe("false");
    await user.click(recurring);
    expect(onSegment).toHaveBeenCalledWith("recurring");
  });
});
