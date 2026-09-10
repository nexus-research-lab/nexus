// INPUT: Workspace Header 的身份、标题、会话/视图导航与动作。
// OUTPUT: 标准身份/返回动作、真实视图选择、可见性关闭与焦点/上下文的回归。
// POS: Workspace Header DOM 行为测试；业务导航和动作事务由消费者负责。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiButton } from "@/shared/ui/button/button";

import { WorkspaceSurfaceHeader } from "./workspace-surface-header";

describe("WorkspaceSurfaceHeader", () => {
  it("keeps identity, typography and tab behavior under shared owners", () => {
    const onChangeTab = vi.fn();
    const { container } = render(
      <I18nProvider>
        <WorkspaceSurfaceHeader
          activeTab="files"
          leading={<UiAgentAvatar name="Nova" size="md" />}
          leadingVariant="identity"
          onChangeTab={onChangeTab}
          tabs={[
            { key: "files", label: "文件" },
            { key: "activity", label: "动态" },
          ]}
          title="工作区"
          trailing={(
            <UiButton onClick={() => undefined} size="2xs" variant="text">
              新建
            </UiButton>
          )}
        />
      </I18nProvider>,
    );

    expect(screen.getByText("工作区").className).toContain("ui-type-page-title");
    expect(screen.getByText("工作区").getAttribute("title")).toBeNull();
    expect(screen.getByRole("button", { name: "新建" }).className).toContain("ui-type-caption");
    const identity = container.querySelector(".workspace-surface-header-identity-avatar")!;
    const avatar = screen.getByRole("img", { name: "Nova" });
    expect(identity.className).not.toMatch(/border|radius|shadow|bg-/);
    expect(avatar.parentElement).toBe(identity);
    expect(avatar.className).toContain("h-10 w-10");
    expect(avatar.className).toContain("rounded-(--radius-control-md)");

    fireEvent.click(screen.getByRole("button", { name: "动态" }));
    expect(onChangeTab).toHaveBeenCalledWith("activity");
  });

  it("uses the public Select for pointer selection, keyboard dismissal and focus return", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const { container } = render(<I18nProvider><WorkspaceSurfaceHeader activeTab="files" compactTabsLabel="工作区导航"
      onChangeTab={onChange} tabs={TABS} /></I18nProvider>);
    showCompact(container);
    const trigger = screen.getByRole("button", { name: /工作区导航.*文件/ });
    expect(trigger.getAttribute("aria-haspopup")).toBe("listbox");
    expect(trigger.className).toContain("ui-type-supporting");
    await user.click(trigger);
    const list = screen.getByRole("listbox");
    expect(trigger.getAttribute("aria-controls")).toBe(list.id);
    expect(within(list).getByRole("option", { name: "文件" }).getAttribute("aria-selected")).toBe("true");
    await user.click(within(list).getByRole("option", { name: "动态" }));
    expect(onChange).toHaveBeenCalledExactlyOnceWith("activity");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(document.activeElement).toBe(trigger);
    await user.keyboard("{Enter}");
    expect(screen.getByRole("listbox")).toBeTruthy();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });

  it("keeps controlled keyboard selection and never invents the first option as the current view", async () => {
    const user = userEvent.setup();
    function Harness() {
      const [active, setActive] = useState<string>("missing-view");
      return <I18nProvider><WorkspaceSurfaceHeader activeTab={active} compactTabsLabel="工作区导航"
        onChangeTab={setActive} tabs={TABS} /></I18nProvider>;
    }
    const { container } = render(<Harness />);
    showCompact(container);
    const trigger = screen.getByRole("button", { name: "工作区导航" });
    expect(trigger.textContent).toBe("工作区导航");
    trigger.focus();
    await user.keyboard("{ArrowDown}");
    expect(trigger.getAttribute("aria-label")).toMatch(/工作区导航.*文件/);
    await user.keyboard("{ArrowDown}");
    expect(trigger.getAttribute("aria-label")).toMatch(/工作区导航.*动态/);
    expect(document.activeElement).toBe(trigger);
  });

  it("keeps language/name changes live and closes changed candidates without reviving old menus", async () => {
    const user = userEvent.setup();
    const renderHeader = (locale: "en" | "zh", tabs = TABS, activeTab = "files") => (
      <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key, params) => Object.entries(params ?? {}).reduce(
        (text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key],
      ) }}><WorkspaceSurfaceHeader activeTab={activeTab} compactTabsLabel="Views" tabs={tabs} onChangeTab={vi.fn()} /></I18N_CONTEXT.Provider>
    );
    const view = render(renderHeader("en"));
    showCompact(view.container);
    const trigger = screen.getByRole("button", { name: "Views: 文件" });
    await user.click(trigger);
    view.rerender(renderHeader("zh", [{ key: "files", label: "一个很长的文件视图名称" }, TABS[1]]));
    expect(screen.getByRole("listbox")).toBeTruthy();
    expect(trigger.getAttribute("aria-label")).toBe("Views：一个很长的文件视图名称");
    expect(within(trigger).getByText("一个很长的文件视图名称").getAttribute("title")).toBeNull();
    view.rerender(renderHeader("zh", [TABS[0]]));
    expect(screen.queryByRole("listbox")).toBeNull();
    view.rerender(renderHeader("en"));
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(screen.getByRole("button", { name: "Views: 文件" })).toBe(trigger);
    await user.click(trigger);
    view.rerender(renderHeader("en", TABS, "activity"));
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(trigger.getAttribute("aria-label")).toBe("Views: 动态");
  });

  it("closes a menu when CSS hides its trigger and never restores it after the container expands again", async () => {
    let notifyResize!: () => void;
    const disconnect = vi.fn();
    vi.stubGlobal("ResizeObserver", class {
      constructor(callback: () => void) { notifyResize = callback; }
      observe() {}
      disconnect = disconnect;
    });
    const user = userEvent.setup();
    const { container, unmount } = render(<I18nProvider><WorkspaceSurfaceHeader activeTab="files" compactTabsLabel="Views"
      tabs={TABS} onChangeTab={vi.fn()} trailing={<UiButton>Outside</UiButton>} /></I18nProvider>);
    const host = showCompact(container);
    const trigger = screen.getByRole("button", { name: /Views.*文件/ });
    await user.click(trigger);
    const outside = screen.getByRole("button", { name: "Outside" });
    outside.focus();
    act(() => { host.style.display = "none"; notifyResize(); });
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(document.activeElement).toBe(outside);
    act(() => { host.style.display = "inline-flex"; notifyResize(); });
    expect(trigger.hasAttribute("disabled")).toBe(false);
    expect(screen.queryByRole("listbox")).toBeNull();
    unmount();
    expect(disconnect).toHaveBeenCalledOnce();
  });

  it("does not mount a hidden second selector for session headers or an empty navigation", () => {
    const view = render(<I18nProvider><WorkspaceSurfaceHeader activeTab="files" compactTabsLabel="Views"
      tabs={TABS} onChangeTab={vi.fn()} tabsLeading={<span>Session tabs</span>} /></I18nProvider>);
    expect(view.container.querySelector(".workspace-surface-header-compact-tabs")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "动态" }));
    view.rerender(<I18nProvider><WorkspaceSurfaceHeader tabs={[]} title="Identity only" /></I18nProvider>);
    expect(screen.queryByRole("group")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("keeps a return action at its own button geometry and disables a selector without a change command", () => {
    const onBack = vi.fn();
    const { container } = render(<I18nProvider><WorkspaceSurfaceHeader leadingVariant="action"
      leading={<UiButton onClick={onBack}>Directory</UiButton>} compactTabsLabel="Views" tabs={TABS} /></I18nProvider>);
    const back = screen.getByRole("button", { name: "Directory" });
    expect(back.parentElement!.className).not.toMatch(/h-10|w-10|border|radius|bg-/);
    fireEvent.click(back);
    expect(onBack).toHaveBeenCalledOnce();
    showCompact(container);
    expect(screen.getByRole("button", { name: "Views" }).hasAttribute("disabled")).toBe(true);
  });
});

const TABS = [{ key: "files", label: "文件" }, { key: "activity", label: "动态" }];

// Only supplies the CSS visibility result; jsdom does not evaluate container queries.
function showCompact(container: HTMLElement) {
  const host = container.querySelector<HTMLDivElement>(".workspace-surface-header-compact-tabs")!;
  host.style.display = "inline-flex";
  fireEvent(window, new Event("resize"));
  return host;
}

afterEach(() => vi.unstubAllGlobals());
