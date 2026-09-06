// INPUT: 文件目录快照与用户展开、选择、上下文和独立次动作。
// OUTPUT: 验证层级展开偏好、精确 entry 命令与具名原生操作。
// POS: Workspace 文件树 DOM 回归；文件读取、持久写入与权限归调用方。

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

import { WorkspaceFileTree } from "./workspace-file-tree";

const entries: WorkspaceFileEntry[] = [
  { path: "docs", name: "docs", is_dir: true, size: 0 },
  { path: "docs/nested", name: "nested", is_dir: true, size: 0 },
  { path: "docs/nested/readme.md", name: "readme.md", is_dir: false, size: 24 },
  { path: "notes.txt", name: "notes.txt", is_dir: false, size: 12 },
].map((entry) => ({ ...entry, modified_at: "2026-09-06T00:00:00Z", depth: entry.path.split("/").length - 1 }));

function setup() {
  const actions = {
    onClickDirectory: vi.fn(), onClickFile: vi.fn(), onContextMenu: vi.fn(),
    onDeleteEntry: vi.fn(), onRenameEntry: vi.fn(),
  };
  const onRootContextMenu = vi.fn();
  const node = (files = entries, activePath: string | null = null) => (
    <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
      <div onContextMenu={onRootContextMenu}>
        <WorkspaceFileTree activePath={activePath} entries={files} focusedDirectoryPath={null} {...actions} />
      </div>
    </I18N_CONTEXT.Provider>
  );
  const view = render(node());
  return { actions, onRootContextMenu, rerender: (files = entries, activePath: string | null = null) => view.rerender(node(files, activePath)) };
}

describe("WorkspaceFileTree", () => {
  it("preserves nested expansion when the parent is collapsed and reopened", async () => {
    const user = userEvent.setup();
    const { actions, rerender } = setup();
    await user.click(screen.getByRole("button", { name: "nested" }));
    expect(screen.getByRole("button", { name: "readme.md" })).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "docs" }));
    expect(screen.queryByRole("button", { name: "readme.md" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "docs" }));
    expect(screen.getByRole("button", { name: "readme.md" })).toBeTruthy();
    rerender(entries.map((entry) => ({ ...entry })));
    expect(screen.getByRole("button", { name: "readme.md" })).toBeTruthy();
    expect(actions.onClickDirectory.mock.calls).toEqual([["docs/nested"], ["docs"], ["docs"]]);
    expect(actions.onClickFile).not.toHaveBeenCalled();
  });

  it("exposes a named nested list, disclosure state and the exact active file", async () => {
    const user = userEvent.setup();
    const { actions, rerender } = setup();
    rerender([]);
    rerender(entries, "notes.txt");
    const root = screen.getByRole("list", { name: "common.file_tree" });
    const docs = within(root).getByRole("button", { name: "docs" });
    expect(docs.getAttribute("aria-expanded")).toBe("true");
    const children = screen.getByRole("list", { name: "docs" });
    expect(docs.getAttribute("aria-controls")).toBe(children.id);
    const file = screen.getByRole("button", { name: "notes.txt" });
    expect(file.getAttribute("aria-current")).toBe("true");
    expect(file.hasAttribute("aria-expanded")).toBe(false);
    await user.click(file);
    expect(actions.onClickFile).toHaveBeenCalledWith("notes.txt");
    await user.click(docs);
    expect(docs.getAttribute("aria-expanded")).toBe("false");
    expect(docs.hasAttribute("aria-controls")).toBe(false);
    expect(screen.queryByRole("list", { name: "docs" })).toBeNull();
  });

  it("keeps keyboard rename and delete independent of directory navigation", async () => {
    const user = userEvent.setup();
    const { actions } = setup();
    await user.tab();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "docs" }));
    await user.keyboard("{Enter}");
    expect(actions.onClickDirectory).toHaveBeenCalledOnce();
    await user.tab();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "home.rename docs" }));
    await user.keyboard("{Enter}");
    expect(actions.onRenameEntry).toHaveBeenCalledWith(entries[0]);
    await user.tab();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "common.delete docs" }));
    await user.keyboard(" ");
    expect(actions.onDeleteEntry).toHaveBeenCalledWith(entries[0]);
    expect(actions.onClickDirectory).toHaveBeenCalledOnce();
    expect(actions.onClickFile).not.toHaveBeenCalled();
  });

  it("routes nested context menus and same-name actions to the exact entry only", async () => {
    const user = userEvent.setup();
    const { actions, onRootContextMenu, rerender } = setup();
    const nestedFile = entries[2];
    const siblingFile = { ...nestedFile, name: nestedFile.name, path: "docs/readme.md" };
    rerender([...entries, siblingFile]);
    await user.click(screen.getByRole("button", { name: "nested" }));
    const nested = screen.getByRole("list", { name: "nested" });
    const file = within(nested).getByRole("button", { name: "readme.md" });
    expect(file.getAttribute("title")).toBe("docs/nested/readme.md");
    fireEvent.contextMenu(file, { button: 2, clientX: 40, clientY: 80 });
    expect(actions.onContextMenu).toHaveBeenCalledOnce();
    expect(actions.onContextMenu.mock.calls[0][1]).toBe(nestedFile);
    expect(actions.onContextMenu.mock.calls[0][0].defaultPrevented).toBe(true);
    expect(onRootContextMenu).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "home.rename docs/readme.md" }));
    expect(actions.onRenameEntry).toHaveBeenCalledWith(siblingFile);
    expect(actions.onClickFile).not.toHaveBeenCalled();
  });

  it("drops removed directory preferences before a path appears again", async () => {
    const user = userEvent.setup();
    const { rerender } = setup();
    await user.click(screen.getByRole("button", { name: "nested" }));
    expect(screen.getByRole("button", { name: "nested" }).getAttribute("aria-expanded")).toBe("true");
    rerender(entries.filter((entry) => !entry.path.startsWith("docs/nested")));
    rerender(entries);
    expect(screen.getByRole("button", { name: "nested" }).getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("button", { name: "readme.md" })).toBeNull();
  });

});
