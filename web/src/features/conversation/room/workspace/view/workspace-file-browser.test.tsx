// INPUT: 文件目录初始加载、刷新中的既有快照与空目录。
// OUTPUT: 验证唯一具名加载、刷新保留目录和共享空态互斥。
// POS: 工作区文件面装配回归；读取和写入策略归控制器。

import { render, screen } from "@testing-library/react";
import { type ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { WorkspaceFileBrowser } from "./workspace-file-browser";

describe("WorkspaceFileBrowser", () => {
  it("announces initial loading, keeps files during refresh and shows one empty guide", () => {
    const controller: ComponentProps<typeof WorkspaceFileBrowser>["controller"] = {
      files: [], isLoadingFiles: true, isMutating: false, isUploading: false,
      hasLoadError: false, handleReloadFiles: vi.fn(),
      focusedDirectoryPath: null,
      handleClickFile: vi.fn(), handleClickDirectory: vi.fn(), handleUploadClick: vi.fn(),
      openCreatePrompt: vi.fn(), openDeletePrompt: vi.fn(), openRenamePrompt: vi.fn(),
      handleContextMenu: vi.fn(), handleRootContextMenu: vi.fn(),
    };
    const node = (current = controller) => <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
      <WorkspaceFileBrowser activePath={null} controller={current} onResizeStart={vi.fn()} resizeControl={null} stacked width={240} />
    </I18N_CONTEXT.Provider>;
    const { rerender } = render(node());
    expect(screen.getByRole("status").textContent).toBe("common.loading");
    expect(screen.queryByText("room.no_files")).toBeNull();
    rerender(node({ ...controller, files: [{ name: "readme.md", path: "readme.md", is_dir: false, depth: 0, modified_at: "2026-09-06" }] }));
    expect(screen.getByRole("button", { name: "readme.md" })).toBeTruthy();
    expect(screen.queryByRole("status")).toBeNull();
    rerender(node({ ...controller, isLoadingFiles: false }));
    expect(screen.getAllByText("room.no_files")).toHaveLength(1);
    expect(screen.getByText("room.workspace_empty_description")).toBeTruthy();
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.queryByRole("list")).toBeNull();
  });
});
