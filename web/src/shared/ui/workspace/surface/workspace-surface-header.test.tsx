// INPUT: Workspace Header 的身份、标题、会话/视图导航与动作。
// OUTPUT: 标准身份/返回动作、真实视图选择、可见性关闭与焦点/上下文的回归。
// POS: Workspace Header DOM 行为测试；业务导航和动作事务由消费者负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { UiButton } from "@/shared/ui/button/button";

import { WorkspaceSurfaceHeader } from "./workspace-surface-header";

describe("WorkspaceSurfaceHeader", () => {

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

  it("keeps a return action at its own button geometry and disables a selector without a change command", () => {
    const onBack = vi.fn();
    const { container } = render(<I18nProvider><WorkspaceSurfaceHeader leadingVariant="action"
      leading={<UiButton onClick={onBack}>Directory</UiButton>} compactTabsLabel="Views" tabs={TABS} /></I18nProvider>);
    const back = screen.getByRole("button", { name: "Directory" });

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
