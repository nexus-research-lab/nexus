// INPUT: Field、SearchInput、Checkbox、Choice 与 SegmentedControl 的用户事件。
// OUTPUT: 证明校验、清除、布尔切换与互斥选择使用真实 DOM/ARIA 合同。
// POS: 表单原语交互测试；业务草稿和网络提交由各 feature 测试负责。

import { fireEvent, render, screen, within } from "@testing-library/react";
import { LayoutGrid, List } from "lucide-react";
import userEvent from "@testing-library/user-event";
import { useState, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { UiCheckbox } from "@/shared/ui/form/checkbox";
import { UiCheckboxRow } from "@/shared/ui/form/checkbox-row";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import {
  UiField,
  UiInput,
  UiNativeSelect,
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

  it("keeps native select semantics while sharing form geometry", async () => {
    const user = userEvent.setup();

    function Harness() {
      const [role, setRole] = useState("member");
      return (
        <label htmlFor="member-role">
          角色
          <UiNativeSelect
            id="member-role"
            onChange={(event) => setRole(event.target.value)}
            value={role}
            variant="surface"
          >
            <option value="member">成员</option>
            <option value="admin">管理员</option>
          </UiNativeSelect>
        </label>
      );
    }

    render(<Harness />);
    const select = screen.getByRole("combobox", { name: "角色" }) as HTMLSelectElement;
    await user.selectOptions(select, "admin");
    expect(select.value).toBe("admin");
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

  it("keeps compact checkbox rows on shared shape and typography roles", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <UiCheckboxRow
        checked={false}
        density="compact"
        label="允许私有网络"
        onChange={onChange}
      />,
    );

    const checkbox = screen.getByRole("checkbox", { name: "允许私有网络" });
    const row = checkbox.closest("label");
    expect(row?.className).toContain("radius-control-md");
    expect(row?.className).not.toContain("rounded-[");
    expect(screen.getByText("允许私有网络").className).toContain("ui-type-caption");
    await user.click(checkbox);
    expect(onChange).toHaveBeenCalledWith(true);
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
            { label: "全局技能库", value: "once" },
            { label: "社区技能", value: "recurring" },
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

    const once = screen.getByRole("button", { name: "全局技能库" });
    const recurring = screen.getByRole("button", { name: "社区技能" });
    const group = screen.getByRole("group", { name: "执行频率" });
    expect(group.className).toContain("surface-radius-md");
    expect(group.className).not.toContain("rounded-full");
    expect(once.getAttribute("aria-pressed")).toBe("true");
    expect(once.className).toContain("radius-control-sm");
    expect(once.className).toContain("ui-type-caption");
    expect(once.className).toContain("whitespace-nowrap");
    expect(once.className).not.toContain("min-w-0");
    expect(once.className).not.toContain("shadow-");
    expect(recurring.getAttribute("aria-pressed")).toBe("false");
    await user.click(recurring);
    expect(onSegment).toHaveBeenCalledWith("recurring");
  });

  it("keeps icon-only segmented options accessible and compact", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <UiSegmentedControl
        density="compact"
        onChange={onChange}
        options={[
          { icon: LayoutGrid, iconOnly: true, label: "卡片视图", value: "grid" },
          { icon: List, iconOnly: true, label: "列表视图", value: "list" },
        ]}
        title="目录视图"
        value="grid"
      />,
    );

    const group = screen.getByRole("group", { name: "目录视图" });
    const grid = within(group).getByRole("button", { name: "卡片视图" });
    const list = within(group).getByRole("button", { name: "列表视图" });
    expect(grid.getAttribute("aria-pressed")).toBe("true");
    expect(grid.className).toContain("h-7");
    expect(grid.querySelector("span")?.className).toBe("sr-only");
    expect(grid.querySelector("svg")?.getAttribute("aria-hidden")).toBe("true");
    await user.click(list);
    expect(onChange).toHaveBeenCalledWith("list");
  });
});
