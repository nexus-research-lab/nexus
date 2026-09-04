// INPUT: 已打开的 Workspace 文件右键菜单、桌面应用目录与命令回调。
// OUTPUT: 证明主/级联菜单复用共享 menuitem，并保留桌面打开命令与关闭语义。
// POS: Workspace context menu 行为测试；共享菜单的键盘和状态矩阵由 menu.test.tsx 负责。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { WorkspaceContextMenu } from "./workspace-context-menu";

vi.mock("@/config/desktop-runtime", () => ({
  getDesktopRuntimeConfig: () => ({ platform: "macos" }),
  isDesktopRuntime: () => true,
}));

describe("WorkspaceContextMenu", () => {
  it("uses shared action rows and preserves nested desktop open commands", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onOpen = vi.fn();

    render(
      <I18nProvider>
        <WorkspaceContextMenu
          canCreateChildren={false}
          entry={{
            depth: 1,
            is_dir: false,
            modified_at: "2026-09-04T00:00:00Z",
            name: "notes.md",
            path: "notes.md",
          }}
          isLoadingOpenApplications={false}
          onAddToChat={vi.fn()}
          onClose={onClose}
          onCopyPath={vi.fn()}
          onCreateFile={vi.fn()}
          onCreateFolder={vi.fn()}
          onDelete={vi.fn()}
          onDownload={vi.fn()}
          onOpen={onOpen}
          onRename={vi.fn()}
          onUpload={vi.fn()}
          openApplications={{
            applications: [{ name: "Visual Studio Code", path: "/Applications/Visual Studio Code.app" }],
            default_application: { name: "TextEdit", path: "/System/Applications/TextEdit.app" },
          }}
          position={{ x: 24, y: 24 }}
        />
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
});
