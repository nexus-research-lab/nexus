// INPUT: Select/Action Menu 的触发器、选项、disabled 项与用户键盘/点击事件。
// OUTPUT: 证明 Portal 菜单的 ARIA、选择、输入法/已处理事件边界、遍历、关闭和焦点归还合同。
// POS: Menu pattern DOM 行为测试；定位数学和业务菜单内容分别由模型/feature 测试负责。

import { act, fireEvent, render as renderReact, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createRef, useRef, useState, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { UiActionMenu, UiActionMenuContent } from "@/shared/ui/menu/action-menu";
import { UiDialogBackdrop, UiDialogPortal, UiDialogShell } from "@/shared/ui/dialog/dialog";
import { UiMenuActionRow } from "@/shared/ui/menu/menu-action-row";
import { useI18n } from "@/shared/i18n/i18n-context";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { SelectMenuOptionRow } from "@/shared/ui/menu/select-menu-primitives";

function render(ui: ReactNode) { return renderReact(ui, { wrapper: I18nProvider }); }

// jsdom 不计算布局；为焦点目录提供可见控件的矩形，不模拟浏览器视觉验收。
beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([new DOMRect(0, 0, 100, 32)] as unknown as DOMRectList);
});
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

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

  it("opens a named listbox, selects an option with its badge, and returns focus", async () => {
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
            { badge: "Room", label: "Gamma", value: "gamma" },
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

    await user.click(screen.getByRole("option", { name: "Gamma Room" }));
    expect(screen.queryByRole("listbox", { name: "选择模型" })).toBeNull();
    expect(trigger.textContent).toContain("Gamma");
    expect(trigger.textContent).toContain("Room");
    expect(document.activeElement).toBe(trigger);
  });

  it.each(["composing", "legacy-ime", "handled"] as const)("ignores %s trigger keys while preserving normal selection and toggling", async (mode) => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    let blockKeyboard = true;
    const ignoredEvent = mode === "composing" ? { isComposing: true }
      : mode === "legacy-ime" ? { keyCode: 229 } : {};
    function Harness() {
      const [value, setValue] = useState("alpha");
      return (
        <div onKeyDownCapture={(event) => {
          if (mode === "handled" && blockKeyboard) event.preventDefault();
        }}>
          <UiSelectMenu ariaLabel="输入边界" value={value}
            options={[{ label: "Alpha", value: "alpha" }, { label: "Gamma", value: "gamma" }]}
            onChange={(nextValue) => { onChange(nextValue); setValue(nextValue); }} />
        </div>
      );
    }
    render(<Harness />);
    const trigger = screen.getByRole("button", { name: "输入边界" });
    trigger.focus();
    for (const key of ["Enter", " ", "ArrowDown", "ArrowUp"]) {
      fireEvent.keyDown(trigger, { key, ...ignoredEvent });
      expect(screen.queryByRole("listbox")).toBeNull();
    }
    expect(trigger.textContent).toContain("Alpha");
    expect(onChange).not.toHaveBeenCalled();

    blockKeyboard = false;
    await user.keyboard("{ArrowDown}");
    expect(onChange.mock.calls).toEqual([["gamma"]]);
    expect(screen.getByRole("listbox")).toBeTruthy();

    blockKeyboard = true;
    fireEvent.keyDown(trigger, { key: "Enter", ...ignoredEvent });
    expect(screen.getByRole("listbox")).toBeTruthy();
    blockKeyboard = false;
    await user.keyboard("{Enter}");
    expect(screen.queryByRole("listbox")).toBeNull();
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

describe("UiSelectMenu listbox navigation", () => {
  const options = [
    { label: "Alpha", value: "alpha" },
    { label: "Disabled", value: "disabled", disabled: true },
    { label: "Gamma with a complete model name", value: "gamma" },
  ];

  it.each(["click", "Enter", "Space"])("focuses the selected item on %s and commits only explicit option activation", async (opening) => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<UiSelectMenu ariaLabel="Model" options={options} value="gamma" onChange={onChange} />);
    const trigger = screen.getByRole("button", { name: "Model" });
    if (opening === "click") await user.click(trigger);
    else { trigger.focus(); await user.keyboard(opening === "Enter" ? "{Enter}" : " "); }
    const gamma = screen.getByRole("option", { name: options[2].label });
    expect(document.activeElement).toBe(gamma);
    expect(gamma.querySelector("[title]")?.getAttribute("title")).toBe(options[2].label);
    await user.keyboard("{ArrowUp}");
    expect(document.activeElement).toBe(screen.getByRole("option", { name: "Alpha" }));
    await user.keyboard("{End}{Home}{ArrowUp}");
    expect(document.activeElement).toBe(gamma);
    expect(onChange).not.toHaveBeenCalled();
    fireEvent(window, new Event("resize"));
    expect(document.activeElement).toBe(gamma);
    await user.keyboard("{Home}{Enter}");
    expect(onChange.mock.calls).toEqual([["alpha"]]);
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });

  it.each([false, true])("exits the popup with Shift=%s to the adjacent page control", async (shift) => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<><button>Before</button><UiSelectMenu ariaLabel="Model" options={options} value="alpha" onChange={onChange} /><button>After</button></>);
    await user.click(screen.getByRole("button", { name: "Model" }));
    await user.tab({ shift });
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: shift ? "Before" : "After" }));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("uses current-language placeholder copy and respects an explicit override", async () => {
    const user = userEvent.setup();
    function Harness({ placeholder }: { placeholder?: string }) {
      const { setLocale } = useI18n();
      return <><button onClick={() => setLocale("en")}>English</button><button onClick={() => setLocale("zh")}>中文</button>
        <UiSelectMenu ariaLabel="Model" options={options} value="missing" onChange={vi.fn()} placeholder={placeholder} /></>;
    }
    const view = render(<Harness />);
    const trigger = screen.getByRole("button", { name: "Model" });
    await user.click(screen.getByRole("button", { name: "English" }));
    expect(trigger.textContent).toBe("Select…");
    await user.click(screen.getByRole("button", { name: "中文" }));
    expect(trigger.textContent).toBe("请选择");
    view.rerender(<Harness placeholder="Choose a model" />);
    expect(trigger.textContent).toBe("Choose a model");
  });

  it.each(["composing", "legacy-ime", "handled"] as const)("does not navigate options for %s events", async (mode) => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<div onKeyDownCapture={(event) => { if (mode === "handled") event.preventDefault(); }}>
      <UiSelectMenu ariaLabel="Model" options={options} value="alpha" onChange={onChange} />
    </div>);
    await user.click(screen.getByRole("button", { name: "Model" }));
    const alpha = screen.getByRole("option", { name: "Alpha" });
    const extras = mode === "composing" ? { isComposing: true } : mode === "legacy-ime" ? { keyCode: 229 } : {};
    for (const key of ["ArrowDown", "End", "Tab"]) fireEvent.keyDown(alpha, { key, ...extras });
    expect(document.activeElement).toBe(alpha);
    expect(screen.getByRole("listbox")).toBeTruthy();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("keeps trigger arrow selection and natural Tab exit", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<><UiSelectMenu ariaLabel="Model" options={options} value="alpha" onChange={onChange} /><button>After</button></>);
    const trigger = screen.getByRole("button", { name: "Model" });
    trigger.focus();
    await user.keyboard("{ArrowDown}");
    expect(onChange.mock.calls).toEqual([["gamma"]]);
    expect(document.activeElement).toBe(trigger);
    await user.tab();
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "After" }));
  });

  it("allows exit from an all-disabled list and closes when its catalog becomes empty", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const view = render(<UiSelectMenu ariaLabel="Model" options={options.map((option) => ({ ...option, disabled: true }))} value="alpha" onChange={onChange} />);
    const trigger = screen.getByRole("button", { name: "Model" });
    await user.click(trigger);
    expect(document.activeElement).toBe(screen.getByRole("listbox"));
    await user.keyboard("{ArrowDown}{Enter}{Escape}");
    expect(onChange).not.toHaveBeenCalled();
    expect(document.activeElement).toBe(trigger);
    await user.click(trigger);
    view.rerender(<UiSelectMenu ariaLabel="Model" options={[]} value="alpha" onChange={onChange} />);
    expect(screen.queryByRole("listbox")).toBeNull();
    expect((trigger as HTMLButtonElement).disabled).toBe(true);
    view.rerender(<UiSelectMenu ariaLabel="Model" options={options} value="alpha" onChange={onChange} />);
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(onChange).not.toHaveBeenCalled();
  });
});

describe("UiActionMenu", () => {
  it("keeps complete action copy in semantic, content-sized rows and hides decorative icons", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const label = "An action with a complete and unusually long label";
    const description = "A long permission scope or failure reason must remain readable before choosing this action.";
    render(<div role="menu"><UiActionMenuContent items={[{
      value: "approve", label, description, icon: <svg role="img" aria-label="Decorative symbol" />,
    }, { value: "disabled", label: "Unavailable", description, disabled: true, tone: "danger" }]} onSelect={onSelect} /></div>);
    const action = screen.getByRole("menuitem", { name: `${label} ${description}` });
    expect(screen.getByText(label).className).toContain("whitespace-normal");
    expect(screen.getByText(label).className).not.toContain("truncate");
    expect(action.className).toContain("min-h-12");
    expect(action.className).toContain("ui-type-control");
    for (const copy of screen.getAllByText(description)) {
      expect(copy.className).toContain("ui-type-metadata");
      expect(copy.className).toContain("ui-type-tone-muted");
      expect(copy.className).not.toContain("truncate");
    }
    expect(screen.queryByRole("img")).toBeNull();
    const disabled = screen.getByRole("menuitem", { name: `Unavailable ${description}` });
    expect(disabled.className).toContain("[&:not(:disabled):hover]:");
    await user.click(disabled);
    expect(onSelect).not.toHaveBeenCalled();
    await user.click(action);
    expect(onSelect).toHaveBeenCalledExactlyOnceWith("approve");
  });

  it("renders footer-only actions without an orphan separator and respects content-level disabled", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const view = render(<div role="menu"><UiActionMenuContent disabled items={[]} footerItems={[{ label: "Reset", value: "reset" }]} onSelect={onSelect} /></div>);
    expect(screen.queryByRole("separator")).toBeNull();
    await user.click(screen.getByRole("menuitem", { name: "Reset" }));
    expect(onSelect).not.toHaveBeenCalled();
    view.rerender(<div role="menu"><UiActionMenuContent items={[{ label: "Open", value: "open" }]} footerItems={[{ label: "Reset", value: "reset" }]} onSelect={onSelect} /></div>);
    expect(screen.getAllByRole("separator")).toHaveLength(1);
    await user.click(screen.getByRole("menuitem", { name: "Reset" }));
    expect(onSelect).toHaveBeenCalledExactlyOnceWith("reset");
  });

  it("measures unclipped content including actual frame spacing, grows and shrinks without resetting focus", async () => {
    const user = userEvent.setup();
    let notifyResize: () => void = () => undefined;
    const disconnect = vi.fn();
    vi.stubGlobal("ResizeObserver", class {
      constructor(callback: () => void) { notifyResize = callback; }
      observe() {}
      disconnect = disconnect;
    });
    function Harness() {
      const anchorRef = useRef<HTMLButtonElement>(null);
      const [isOpen, setIsOpen] = useState(false);
      return <><button ref={anchorRef} onClick={() => setIsOpen(true)}>Measured menu</button>
        <UiActionMenu anchorRef={anchorRef} ariaLabel="Measured actions" isOpen={isOpen}
          items={[{ value: "a", label: "Alpha" }, { value: "b", label: "Beta" }]}
          onClose={() => setIsOpen(false)} onSelect={() => undefined} /></>;
    }
    const view = render(<Harness />);
    await user.click(screen.getByRole("button", { name: "Measured menu" }));
    const menu = screen.getByRole("menu");
    menu.style.padding = "4px";
    menu.style.border = "1px solid black";
    const content = menu.firstElementChild!;
    const height = vi.spyOn(content, "scrollHeight", "get").mockReturnValue(170);
    await user.keyboard("{End}");
    const beta = screen.getByRole("menuitem", { name: "Beta" });
    act(() => notifyResize());
    expect(menu.style.maxHeight).toBe("180px");
    expect(document.activeElement).toBe(beta);
    height.mockReturnValue(600);
    act(() => notifyResize());
    expect(menu.style.maxHeight).toBe("320px");
    height.mockReturnValue(70);
    act(() => notifyResize());
    expect(menu.style.maxHeight).toBe("80px");
    expect(document.activeElement).toBe(beta);
    view.unmount();
    expect(disconnect).toHaveBeenCalled();
  });

  it("navigates mixed actions and checked items and activates each toggle once", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    function Harness() {
      const anchorRef = useRef<HTMLButtonElement>(null);
      const [isOpen, setIsOpen] = useState(false);
      const [checked, setChecked] = useState(false);
      return <>
        <button ref={anchorRef} onClick={() => setIsOpen(true)} type="button">选项</button>
        <UiActionMenu anchorRef={anchorRef} ariaLabel="混合动作" isOpen={isOpen}
          items={[
            { label: "切换", value: "toggle", checked },
            { label: "锁定", value: "locked", checked: true, disabled: true },
            { label: "打开", value: "open" },
          ]}
          onClose={() => setIsOpen(false)}
          onSelect={(value) => { onSelect(value); if (value === "toggle") setChecked((current) => !current); }} />
      </>;
    }
    render(<Harness />);
    const anchor = screen.getByRole("button", { name: "选项" });
    await user.click(anchor);
    const toggle = screen.getByRole("menuitemcheckbox", { name: "切换", checked: false });
    expect(document.activeElement).toBe(toggle);
    expect(toggle.querySelector("button, input")).toBeNull();
    await user.click(screen.getByRole("menuitemcheckbox", { name: "锁定", checked: true }));
    expect(onSelect).not.toHaveBeenCalled();
    toggle.focus();
    await user.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "打开" }));
    await user.keyboard("{ArrowDown}{Enter}");
    expect(onSelect.mock.calls).toEqual([["toggle"]]);
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(anchor);
    await user.click(anchor);
    expect(screen.getByRole("menuitemcheckbox", { name: "切换", checked: true })).toBe(document.activeElement);
    await user.keyboard(" ");
    expect(onSelect.mock.calls).toEqual([["toggle"], ["toggle"]]);
    expect(screen.queryByRole("menu")).toBeNull();
  });

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
    const actionRef = createRef<HTMLButtonElement>();

    render(
      <div aria-label="文件操作" role="menu">
        <UiMenuActionRow ref={actionRef} active onClick={onClick} tone="danger">
          删除
        </UiMenuActionRow>
        <UiMenuActionRow disabled>不可用</UiMenuActionRow>
      </div>,
    );

    const action = screen.getByRole("menuitem", { name: "删除" });
    const disabledAction = screen.getByRole("menuitem", { name: "不可用" });
    expect(actionRef.current).toBe(action);
    expect(action.className).toContain("bg-(--surface-interactive-active-background)");
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

  it.each([false, true])("exits on Tab with shift=%s and continues from the anchor", async (shift) => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    function Harness() {
      const anchorRef = useRef<HTMLButtonElement>(null);
      const [isOpen, setIsOpen] = useState(false);
      return <>
        <button type="button">之前</button>
        <button type="button" ref={anchorRef} onClick={() => setIsOpen(true)}>菜单</button>
        <button type="button">之后</button>
        <UiActionMenu anchorRef={anchorRef} ariaLabel="动作" isOpen={isOpen}
          items={[{ label: "打开", value: "open" }, { label: "删除", value: "delete" }]}
          onClose={() => setIsOpen(false)} onSelect={onSelect} />
      </>;
    }
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "菜单" }));
    await user.keyboard("{End}");
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "删除" }));
    await user.tab({ shift });
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: shift ? "之前" : "之后" }));
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("keeps Tab exit inside the enclosing real modal, including its wrap boundary", async () => {
    const user = userEvent.setup();
    const onDialogClose = vi.fn();
    function Harness() {
      const anchorRef = useRef<HTMLButtonElement>(null);
      const [isOpen, setIsOpen] = useState(false);
      return <>
        <button type="button">背景动作</button>
        <UiDialogPortal><UiDialogBackdrop aria-label="设置" onClose={onDialogClose}><UiDialogShell>
          <button type="button">弹窗首项</button>
          <button ref={anchorRef} type="button" onClick={() => setIsOpen(true)}>菜单</button>
          <UiActionMenu anchorRef={anchorRef} ariaLabel="动作" isOpen={isOpen}
            items={[{ label: "打开", value: "open" }]}
            onClose={() => setIsOpen(false)} onSelect={vi.fn()} />
        </UiDialogShell></UiDialogBackdrop></UiDialogPortal>
      </>;
    }
    render(<Harness />);
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("button", { name: "弹窗首项" })));
    await user.click(screen.getByRole("button", { name: "菜单" }));
    await user.tab();
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "弹窗首项" }));
    expect(onDialogClose).not.toHaveBeenCalled();
  });

  it("keeps an all-disabled menu focusable and lets Escape return to its trigger", async () => {
    const user = userEvent.setup();
    function Harness() {
      const anchorRef = useRef<HTMLButtonElement>(null);
      const [isOpen, setIsOpen] = useState(false);
      return <>
        <button ref={anchorRef} type="button" onClick={() => setIsOpen(true)}>菜单</button>
        <UiActionMenu anchorRef={anchorRef} ariaLabel="不可用动作" isOpen={isOpen}
          items={[{ label: "等待中", value: "wait", disabled: true }]}
          onClose={() => setIsOpen(false)} onSelect={vi.fn()} />
      </>;
    }
    render(<Harness />);
    const trigger = screen.getByRole("button", { name: "菜单" });
    await user.click(trigger);
    expect(document.activeElement).toBe(screen.getByRole("menu"));
    await user.keyboard("{ArrowDown}{Escape}");
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(trigger);
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

describe("UiSelectMenu context changes", () => {
  it("discards an open context without replacing the trigger or stealing outside focus", async () => {
    const user = userEvent.setup();
    const element = (resetKey: string) => <>
      <UiSelectMenu ariaLabel="Scoped choice" value="alpha" resetKey={resetKey} onChange={vi.fn()}
        options={[{ label: "Alpha", value: "alpha" }]} />
      <button type="button">Outside</button>
    </>;
    const { rerender } = render(element("first"));
    const trigger = screen.getByRole("button", { name: "Scoped choice" });
    await user.click(trigger);
    expect(screen.getByRole("listbox")).toBeTruthy();
    const outside = screen.getByRole("button", { name: "Outside" }); outside.focus();
    rerender(element("second"));
    expect(screen.getByRole("button", { name: "Scoped choice" })).toBe(trigger);
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(trigger.getAttribute("aria-controls")).toBeNull();
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(document.activeElement).toBe(outside);
    rerender(element("first"));
    expect(screen.queryByRole("listbox")).toBeNull();
    await user.click(trigger);
    await user.click(screen.getByRole("option", { name: "Alpha" }));
    expect(document.activeElement).toBe(trigger);
  });
});
