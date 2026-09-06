// INPUT: Field/Select 关联、原生/业务校验、SearchInput、Checkbox 与选择控件的用户事件。
// OUTPUT: 证明描述/错误归属、最新调用方属性恢复、清除与互斥选择使用真实 DOM/ARIA 合同。
// POS: 表单原语交互测试；业务草稿和网络提交由各 feature 测试负责。

import { fireEvent, render, screen, within } from "@testing-library/react";
import { LayoutGrid, List } from "lucide-react";
import userEvent from "@testing-library/user-event";
import { createRef, useState, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { UiCheckbox } from "@/shared/ui/form/checkbox";
import { UiCheckboxRow } from "@/shared/ui/form/checkbox-row";
import { UiChoiceButton, UiRadioChoice } from "@/shared/ui/form/choice";
import {
  UiField,
  UiInput,
  UiNativeSelect,
  UiSearchInput,
  UiTextarea,
} from "@/shared/ui/form/form-control";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";
import { SidebarSearchAction, SidebarSearchField } from "@/shared/ui/form/sidebar-search-field";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";

function renderWithI18n(children: ReactNode) {
  const messages: Record<string, string> = {
    "common.clear": "清除",
    "common.search": "搜索",
    "common.invalid_field": "字段格式不正确",
    "common.required_field": "请填写此字段",
  };
  const wrapper = ({ children }: { children: ReactNode }) => (
    <I18N_CONTEXT.Provider
      value={{
        locale: "zh",
        setLocale: vi.fn(),
        t: (key) => messages[key] ?? key,
      }}
    >
      {children}
    </I18N_CONTEXT.Provider>
  );
  return render(children, { wrapper });
}

describe("form primitives", () => {
  it.each(["input", "textarea", "native-select", "search", "select-menu"])(
    "associates the %s with its visible description and explicit error",
    (kind) => {
      function Harness({ error }: { error?: string }) {
        return (
          <UiField description="Visible help" error={error} htmlFor="field-control" label="Field">
            {kind === "input" ? <UiInput id="field-control" /> : null}
            {kind === "textarea" ? <UiTextarea id="field-control" /> : null}
            {kind === "native-select" ? <UiNativeSelect id="field-control"><option>A</option></UiNativeSelect> : null}
            {kind === "search" ? <UiSearchInput aria-label="Field" id="field-control" onChange={vi.fn()} value="" /> : null}
            {kind === "select-menu" ? <UiSelectMenu ariaLabel="Field" id="field-control" onChange={vi.fn()} options={[{ label: "A", value: "a" }]} value="a" /> : null}
          </UiField>
        );
      }
      const { rerender } = renderWithI18n(<Harness />);
      const control = screen.getByLabelText("Field");
      expect(document.getElementById(control.getAttribute("aria-describedby")!)?.textContent).toBe("Visible help");

      rerender(<Harness error="Already in use" />);
      expect(control.getAttribute("aria-invalid")).toBe("true");
      expect(document.getElementById(control.getAttribute("aria-errormessage")!)?.textContent).toBe("Already in use");
      expect(control.hasAttribute("aria-describedby")).toBe(false);
      expect(screen.queryByText("Visible help")).toBeNull();

      rerender(<Harness />);
      expect(control.hasAttribute("aria-invalid")).toBe(false);
      expect(control.hasAttribute("aria-errormessage")).toBe(false);
      expect(document.getElementById(control.getAttribute("aria-describedby")!)?.textContent).toBe("Visible help");
    },
  );

  it("adds help only to the explicitly bound control and preserves caller descriptions", () => {
    renderWithI18n(
      <UiField description="Field help" htmlFor="primary-field" label="Primary">
        <span id="external-help">External help</span>
        <UiInput aria-describedby="external-help external-help" id="primary-field" />
        <UiInput aria-describedby="external-help" aria-label="Secondary" />
      </UiField>,
    );
    const primary = screen.getByLabelText("Primary");
    const descriptionIds = primary.getAttribute("aria-describedby")!.split(" ");
    expect(descriptionIds).toHaveLength(2);
    expect(descriptionIds.map((id) => document.getElementById(id)?.textContent)).toEqual(["External help", "Field help"]);
    expect(screen.getByLabelText("Secondary").getAttribute("aria-describedby")).toBe("external-help");
  });

  it("names compound fields as groups without assigning their aggregate error to every input", () => {
    const { container } = renderWithI18n(
      <UiField error="Duplicate key" label="Environment variables">
        <UiInput aria-label="Key" />
        <UiInput aria-label="Value" />
      </UiField>,
    );
    const group = screen.getByRole("group", { name: "Environment variables" });
    expect(group.getAttribute("aria-invalid")).toBe("true");
    expect(group.getAttribute("aria-errormessage")).toBe(screen.getByRole("alert").id);
    expect(container.querySelector("label")).toBeNull();
    for (const input of screen.getAllByRole("textbox")) {
      expect(input.hasAttribute("aria-invalid")).toBe(false);
    }
  });

  it("restores the latest caller error attributes after native validity recovers", async () => {
    const user = userEvent.setup();
    function Harness() {
      const [updated, setUpdated] = useState(false);
      return (
        <>
          <UiField htmlFor="draft" label="Draft">
            <UiInput aria-errormessage={updated ? "latest-error" : "original-error"} aria-invalid={updated ? "grammar" : "spelling"} id="draft" required />
          </UiField>
          <button onClick={() => setUpdated(true)}>Update validation</button>
          <p id="original-error">Original error</p>
          <p id="latest-error">Latest error</p>
        </>
      );
    }
    renderWithI18n(<Harness />);
    const input = screen.getByLabelText("Draft");
    fireEvent.invalid(input);
    expect(input.getAttribute("aria-errormessage")).toBe(screen.getByRole("alert").id);
    await user.click(screen.getByText("Update validation"));
    expect(input.getAttribute("aria-invalid")).toBe("true");
    await user.type(input, "Nexus");
    expect(input.getAttribute("aria-invalid")).toBe("grammar");
    expect(input.getAttribute("aria-errormessage")).toBe("latest-error");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("clears an obsolete native error when a controlled field is reset programmatically", async () => {
    const user = userEvent.setup();
    function Harness() {
      const [value, setValue] = useState("");
      return (
        <>
          <UiField htmlFor="controlled-draft" label="Draft">
            <UiInput id="controlled-draft" onChange={(event) => setValue(event.target.value)} required value={value} />
          </UiField>
          <button onClick={() => setValue("Restored draft")}>Restore</button>
        </>
      );
    }
    renderWithI18n(<Harness />);
    const input = screen.getByLabelText("Draft");
    fireEvent.invalid(input);
    expect(screen.getByRole("alert")).toBeTruthy();
    await user.click(screen.getByText("Restore"));
    expect(input.getAttribute("aria-invalid")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("keeps an explicit business error after the native input becomes valid", async () => {
    const user = userEvent.setup();
    renderWithI18n(
      <UiField error="名称已被占用" htmlFor="unique-name" label="Name">
        <UiInput id="unique-name" required />
      </UiField>,
    );
    const input = screen.getByLabelText("Name");
    fireEvent.invalid(input);
    await user.type(input, "Nexus");
    expect((input as HTMLInputElement).validity.valid).toBe(true);
    expect(input.getAttribute("aria-invalid")).toBe("true");
    expect(input.getAttribute("aria-errormessage")).toBe(screen.getByRole("alert").id);
    expect(screen.getByRole("alert").textContent).toBe("名称已被占用");
  });

  it("keeps compound and nested field validation attached to the exact native control", () => {
    const onInvalid = vi.fn();
    renderWithI18n(
      <form onInvalid={onInvalid}>
        <UiField label="Compound">
          <UiInput aria-label="First" />
          <UiField label="Nested">
            <UiInput aria-label="Second" required />
          </UiField>
        </UiField>
      </form>,
    );
    const input = screen.getByLabelText("Second");
    fireEvent.invalid(input);
    expect(screen.getAllByRole("alert")).toHaveLength(1);
    expect(input.getAttribute("aria-errormessage")).toBe(screen.getByRole("alert").id);
    expect(screen.getByLabelText("First").hasAttribute("aria-invalid")).toBe(false);
    expect(onInvalid).toHaveBeenCalledOnce();
    expect(document.activeElement).toBe(input);
  });

  it("skips controls barred from validation when focusing the first invalid field", () => {
    renderWithI18n(
      <form>
        <UiInput aria-label="Disabled" disabled ref={(input) => { input?.setCustomValidity("Unavailable"); }} />
        <UiField htmlFor="first-invalid" label="First" required><UiInput id="first-invalid" required /></UiField>
        <UiField htmlFor="second-invalid" label="Second" required><UiInput id="second-invalid" required /></UiField>
      </form>,
    );
    fireEvent.invalid(screen.getByRole("textbox", { name: "First" }));
    fireEvent.invalid(screen.getByRole("textbox", { name: "Second" }));
    expect(screen.getAllByRole("alert")).toHaveLength(1);
    expect(document.activeElement).toBe(screen.getByRole("textbox", { name: "First" }));
  });

  it("keeps technical text and verification codes as exact native form values", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    const codeRef = createRef<HTMLInputElement>();
    render(
      <form onSubmit={(event) => {
        event.preventDefault();
        onSubmit(Object.fromEntries(new FormData(event.currentTarget)));
      }}>
        <UiInput aria-label="Command" name="command" textRole="code" />
        <UiTextarea aria-label="Template" name="template" textRole="code" />
        <UiInput ref={codeRef} aria-label="Verification code" autoComplete="one-time-code" inputMode="numeric" maxLength={6} name="verification" textRole="verification" />
        <button type="submit">Save</button>
      </form>,
    );

    await user.type(screen.getByRole("textbox", { name: "Command" }), "review-work");
    await user.type(screen.getByRole("textbox", { name: "Template" }), "# Agent{Enter}Keep scope.");
    const verification = screen.getByRole("textbox", { name: "Verification code" });
    await user.type(verification, "0012049");
    expect(codeRef.current).toBe(verification);
    expect(verification.getAttribute("type")).toBe("text");
    expect(verification.getAttribute("autocomplete")).toBe("one-time-code");
    expect(verification.hasAttribute("textrole")).toBe(false);
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(onSubmit).toHaveBeenCalledWith({
      command: "review-work", template: "# Agent\nKeep scope.", verification: "001204",
    });
  });

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

  it("keeps sidebar search, clear and creation independent with a concise named field", async () => {
    const user = userEvent.setup();
    const onCreate = vi.fn();
    const onSubmit = vi.fn();
    function Harness({ disabled = false }: { disabled?: boolean }) {
      const [query, setQuery] = useState("");
      return (
        <form onSubmit={(event) => { event.preventDefault(); onSubmit(); }}>
          <SidebarSearchField
            action={<SidebarSearchAction disabled={disabled} onClick={onCreate} title="新建智能体"><List /></SidebarSearchAction>}
            label="搜索联系人"
            onChange={setQuery}
            value={query}
          />
        </form>
      );
    }
    const { rerender } = renderWithI18n(<Harness />);
    const search = screen.getByRole("searchbox", { name: "搜索联系人" }) as HTMLInputElement;
    const create = screen.getByRole("button", { name: "新建智能体" }) as HTMLButtonElement;
    expect(search.placeholder).toBe("搜索");
    expect(create.type).toBe("button");
    expect(create.hasAttribute("title")).toBe(false);

    await user.type(search, "Research");
    expect(search.value).toBe("Research");
    await user.click(screen.getByRole("button", { name: "清除" }));
    expect(search.value).toBe("");
    expect(document.activeElement).toBe(search);
    expect(onCreate).not.toHaveBeenCalled();

    await user.tab();
    expect(document.activeElement).toBe(create);
    await user.keyboard("{Enter}");
    expect(onCreate).toHaveBeenCalledOnce();
    expect(onSubmit).not.toHaveBeenCalled();
    expect(search.value).toBe("");

    rerender(<Harness disabled />);
    expect(create.disabled).toBe(true);
    await user.click(create);
    await user.type(search, "Nexus");
    expect(search.value).toBe("Nexus");
    expect(onCreate).toHaveBeenCalledOnce();
    expect(onSubmit).not.toHaveBeenCalled();
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

  it("projects an indeterminate checkbox as one native mixed state", () => {
    render(
      <label htmlFor="mixed-checkbox">
        <UiCheckbox id="mixed-checkbox" indeterminate />
        部分选择
      </label>,
    );

    const checkbox = screen.getByRole("checkbox", { name: "部分选择" }) as HTMLInputElement;
    expect(checkbox.indeterminate).toBe(true);
    expect(checkbox.getAttribute("aria-checked")).toBe("mixed");
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
        <UiChoiceButton active onClick={onChoice} tone="neutral">当前来源</UiChoiceButton>
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
    expect(choice.className).toContain("bg-(--surface-interactive-active-background)");
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

  it("keeps radio choice native semantics while sharing selection styling", async () => {
    const user = userEvent.setup();

    function Harness() {
      const [scope, setScope] = useState("once");
      return (
        <div aria-label="授权范围" role="radiogroup">
          <UiRadioChoice
            checked={scope === "once"}
            choiceSize="xs"
            name="scope"
            onChange={() => setScope("once")}
          >
            本次
          </UiRadioChoice>
          <UiRadioChoice
            checked={scope === "session"}
            choiceSize="xs"
            name="scope"
            onChange={() => setScope("session")}
          >
            会话
          </UiRadioChoice>
        </div>
      );
    }

    render(<Harness />);
    const once = screen.getByRole("radio", { name: "本次" }) as HTMLInputElement;
    const session = screen.getByRole("radio", { name: "会话" }) as HTMLInputElement;
    expect(once.checked).toBe(true);
    expect(once.parentElement?.getAttribute("data-active")).toBe("true");

    await user.click(session);
    expect(once.checked).toBe(false);
    expect(session.checked).toBe(true);
    expect(session.parentElement?.getAttribute("data-active")).toBe("true");
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
