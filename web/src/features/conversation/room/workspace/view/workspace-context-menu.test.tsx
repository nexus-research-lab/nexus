// INPUT: 已打开的 Workspace 文件右键菜单、桌面应用目录与命令回调。
// OUTPUT: 证明主/级联菜单的真实键盘进入、逐层返回、Tab/外部关闭与桌面命令。
// POS: Workspace context menu DOM 回归；精确文件操作结果由控制器测试负责。

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode, useState, type ComponentProps } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { WorkspaceContextMenu } from "./workspace-context-menu";

vi.mock("@/config/desktop-runtime", () => ({
  getDesktopRuntimeConfig: () => ({ platform: "macos" }),
  isDesktopRuntime: () => true,
}));

function makeProps(): ComponentProps<typeof WorkspaceContextMenu> {
  return {
    canCreateChildren: false,
    entry: { depth: 1, is_dir: false, modified_at: "2026-09-04T00:00:00Z", name: "notes.md", path: "notes.md" },
    isLoadingOpenApplications: false,
    onAddToChat: vi.fn(), onClose: vi.fn(), onCopyPath: vi.fn(), onCreateFile: vi.fn(),
    onCreateFolder: vi.fn(), onDelete: vi.fn(), onDownload: vi.fn(), onOpen: vi.fn(),
    onRename: vi.fn(), onUpload: vi.fn(),
    openApplications: {
      applications: [{ name: "Visual Studio Code", path: "/Applications/Visual Studio Code.app" }],
      default_application: { name: "TextEdit", path: "/System/Applications/TextEdit.app" },
    },
    position: { x: 24, y: 24 },
  };
}

// jsdom 不计算布局；为焦点目录提供可见控件的矩形，不模拟浏览器视觉验收。
beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([new DOMRect(0, 0, 100, 32)] as unknown as DOMRectList);
});
afterEach(() => vi.restoreAllMocks());

describe("WorkspaceContextMenu", () => {
  it("uses shared action rows and preserves nested desktop open commands", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onOpen = vi.fn();

    render(
      <I18nProvider>
        <WorkspaceContextMenu {...makeProps()} onClose={onClose} onOpen={onOpen} />
      </I18nProvider>,
    );

    const submenuTrigger = screen.getAllByRole("menuitem").find(
      (item) => item.getAttribute("aria-haspopup") === "menu",
    );
    expect(submenuTrigger).toBeTruthy();
    expect(submenuTrigger?.tagName).toBe("BUTTON");
    expect(submenuTrigger?.className).toContain("radius-control-lg");

    await user.click(submenuTrigger!);
    expect(submenuTrigger?.getAttribute("aria-expanded")).toBe("true");
    const fileManagerAction = screen.getByRole("menuitem", { name: "Finder" });
    expect(fileManagerAction.tagName).toBe("BUTTON");
    expect(screen.getAllByRole("menu")).toHaveLength(2);

    fireEvent.click(fileManagerAction);
    expect(onOpen).toHaveBeenCalledWith("file_manager");
    expect(onClose).toHaveBeenCalledTimes(1);
  });
  it("enters and leaves the submenu with keyboard without traversing its parent rows", async () => {
    const user = userEvent.setup();
    const props = makeProps();
    render(<I18nProvider><WorkspaceContextMenu {...props} /></I18nProvider>);
    const trigger = screen.getAllByRole("menuitem").find((item) => item.getAttribute("aria-haspopup") === "menu")!;
    trigger.focus();
    fireEvent.keyDown(trigger, { key: "ArrowRight", keyCode: 229 });
    expect(screen.getAllByRole("menu")).toHaveLength(1);
    await user.keyboard("{ArrowRight}");
    const submenu = screen.getAllByRole("menu")[1];
    const actions = within(submenu).getAllByRole("menuitem");
    expect(document.activeElement).toBe(actions[0]);
    await user.keyboard("{End}");
    expect(document.activeElement).toBe(actions.at(-1));
    await user.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(actions[0]);
    await user.keyboard("{ArrowLeft}");
    expect(screen.getAllByRole("menu")).toHaveLength(1);
    expect(document.activeElement).toBe(trigger);
    await user.keyboard("{ArrowRight}{Escape}");
    expect(screen.getAllByRole("menu")).toHaveLength(1);
    expect(document.activeElement).toBe(trigger);
    expect(props.onClose).not.toHaveBeenCalled();
    await user.keyboard("{Escape}");
    expect(props.onClose).toHaveBeenCalledTimes(1);
    expect(props.onOpen).not.toHaveBeenCalled();
  });

  it("returns to the opening control on Escape and follows its Tab order without stealing outside focus", async () => {
    const user = userEvent.setup();
    const props = makeProps();
    function Harness() {
      const [isOpen, setIsOpen] = useState(false);
      return <I18nProvider>
        <button type="button">Before</button>
        <button type="button" onClick={() => setIsOpen(true)}>Open</button>
        <button type="button">After</button>
        {isOpen && <WorkspaceContextMenu {...props} onClose={() => setIsOpen(false)} />}
      </I18nProvider>;
    }
    render(<StrictMode><Harness /></StrictMode>);
    const trigger = screen.getByRole("button", { name: "Open" });
    await user.click(trigger);
    expect(document.activeElement).toBe(screen.getAllByRole("menuitem")[0]);
    fireEvent.keyDown(document.activeElement!, { key: "Escape", isComposing: true });
    expect(screen.getByRole("menu")).toBeTruthy();
    await user.keyboard("{Escape}");
    expect(document.activeElement).toBe(trigger);
    expect(screen.queryByRole("menu")).toBeNull();
    await user.click(trigger);
    await user.tab();
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "After" }));
    await user.click(trigger);
    await user.tab({ shift: true });
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Before" }));
    await user.click(trigger);
    await user.click(screen.getByRole("button", { name: "After" }));
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "After" }));
  });

});
