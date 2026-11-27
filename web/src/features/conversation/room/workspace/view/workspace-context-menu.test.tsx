// INPUT: 已打开的 Workspace 文件右键菜单、桌面应用目录与命令回调。
// OUTPUT: 证明主/级联菜单的真实键盘进入、逐层返回、Tab/外部关闭与桌面命令。
// POS: Workspace context menu DOM 回归；精确文件操作结果由控制器测试负责。

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ComponentProps } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { WorkspaceContextMenu } from "./workspace-context-menu";

vi.mock("@/config/desktop-runtime", () => ({
  getDesktopRuntimeConfig: () => ({ platform: "macos" }),
  isDesktopRuntime: () => true,
}));

function makeProps(): ComponentProps<typeof WorkspaceContextMenu> {
  return {
    anchor: document.body,
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

function MenuRegion({ menuProps, hasFollowingAction = true }: {
  menuProps?: Partial<ComponentProps<typeof WorkspaceContextMenu>>;
  hasFollowingAction?: boolean;
}) {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const [position, setPosition] = useState<{ x: number; y: number } | null>(null);
  return <I18nProvider>
    <section aria-label="Workspace region" onContextMenu={(event) => {
      event.preventDefault();
      setAnchor(event.currentTarget);
      setPosition({ x: event.clientX, y: event.clientY });
    }}>
      <button type="button">Source</button>
      {hasFollowingAction && <button type="button">Region action</button>}
    </section>
    <WorkspaceContextMenu {...makeProps()} {...menuProps} anchor={anchor} position={position} onClose={() => setPosition(null)} />
  </I18nProvider>;
}

// jsdom 不计算布局；为焦点目录提供可见控件的矩形，不模拟浏览器视觉验收。
beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([new DOMRect(0, 0, 100, 32)] as unknown as DOMRectList);
});
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

describe("WorkspaceContextMenu", () => {
  it("exits all portal menu levels on Tab without focusing a closing parent row", async () => {
    const user = userEvent.setup();
    render(<MenuRegion hasFollowingAction={false} />);
    const source = screen.getByRole("button", { name: "Source" });
    await user.click(source);
    fireEvent.contextMenu(source, { button: 2, clientX: 100, clientY: 100 });
    const trigger = screen.getAllByRole("menuitem").find((item) => item.getAttribute("aria-haspopup") === "menu")!;
    trigger.focus();
    await user.keyboard("{ArrowRight}");
    expect(screen.getAllByRole("menu")).toHaveLength(2);
    await user.tab();
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(source);
  });

  it.each(["pointer", "resize", "Escape"])("dismisses a menu whose invoking region was removed on %s", (event) => {
    render(<MenuRegion />);
    fireEvent.contextMenu(screen.getByRole("button", { name: "Source" }), { button: 2, clientX: 100, clientY: 100 });
    expect(screen.getByRole("menu", { name: "notes.md" })).toBeTruthy();
    const region = screen.getByRole("region", { name: "Workspace region" });
    const parent = region.parentElement!;
    region.remove();
    try {
      if (event === "pointer") fireEvent.pointerDown(document.body);
      else if (event === "resize") fireEvent.resize(window);
      else expect(fireEvent.keyDown(document, { key: "Escape" })).toBe(true);
      expect(screen.queryByRole("menu")).toBeNull();
    } finally {
      // 还原被夹具移出的节点，让 React 按自己的树执行测试清理。
      parent.appendChild(region);
    }
  });

  it("keeps a portal submenu open across its pointer gap and updates a long application list in place", async () => {
    const user = userEvent.setup();
    vi.stubGlobal("innerWidth", 720);
    vi.stubGlobal("innerHeight", 480);
    const onOpen = vi.fn();
    const { rerender } = render(<MenuRegion menuProps={{ isLoadingOpenApplications: true, onOpen }} />);
    const source = screen.getByRole("button", { name: "Source" });
    await user.click(source);
    fireEvent.contextMenu(source, { button: 2, clientX: 710, clientY: 470 });
    const parent = screen.getByRole("menu", { name: "notes.md" });
    const first = within(parent).getAllByRole("menuitem")[0];
    const trigger = within(parent).getAllByRole("menuitem").find((item) => item.getAttribute("aria-haspopup") === "menu")!;
    const bounds = vi.spyOn(trigger, "getBoundingClientRect").mockReturnValue(new DOMRect(488, 254, 216, 36));
    await user.hover(trigger);
    expect(document.activeElement).toBe(first);
    const child = screen.getAllByRole("menu").find((menu) => menu !== parent)!;
    expect(parent.contains(child)).toBe(false);
    expect(child.style.left).toBe("258px");
    expect(child.style.top).toBe("254px");
    expect(child.style.maxHeight).toBe("158px");
    await user.unhover(trigger);
    expect(child.isConnected).toBe(true);
    await user.hover(screen.getByRole("menuitem", { name: "Finder" }));
    expect(child.isConnected).toBe(true);
    const apps = Array.from({ length: 40 }, (_, index) => ({ name: `App ${index}`, path: `/Applications/App ${index}.app` }));
    rerender(<MenuRegion menuProps={{ onOpen, openApplications: { applications: apps } }} />);
    expect(child.style.maxHeight).toBe("320px");
    expect(child.style.top).toBe("148px");
    expect(within(child).getAllByRole("menuitem")).toHaveLength(43);

    bounds.mockReturnValue(new DOMRect(300, 110, 216, 36));
    fireEvent.scroll(parent);
    expect(child.style.left).toBe("70px");
    expect(child.style.top).toBe("110px");
    expect(document.activeElement).toBe(first);
    await user.click(screen.getByRole("menuitem", { name: "App 39" }));
    expect(onOpen).toHaveBeenCalledExactlyOnceWith("application", "/Applications/App 39.app");
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(source);
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

});
