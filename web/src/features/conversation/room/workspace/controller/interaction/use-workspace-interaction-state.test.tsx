// INPUT: 真实 contextmenu 事件、源元素、原始坐标与 Workspace scope。
// OUTPUT: 保存调用身份/坐标而不猜菜单尺寸，键盘调用沿源元素，切换 scope 清空菜单。
// POS: Workspace 本地交互 DOM 回归；指针与子菜单视口限制由公共 Overlay 测试负责。

import { fireEvent, render, renderHook, screen } from "@testing-library/react";
import { createRef } from "react";
import { afterEach, expect, it, vi } from "vitest";

import { useWorkspaceInteractionState } from "./use-workspace-interaction-state";

afterEach(() => vi.restoreAllMocks());

it("preserves raw pointer and source identity, supports keyboard invocation and clears the old scope", () => {
  const fileInputRef = createRef<HTMLInputElement>();
  const file = { path: "notes.md", name: "notes.md", depth: 0, is_dir: false, modified_at: "" };
  const { result, rerender } = renderHook(({ scopeKey }) => useWorkspaceInteractionState({
    fileInputRef, focusedDirectoryPath: null, scopeKey,
  }), { initialProps: { scopeKey: "one" } });
  render(<>
    <button type="button" onContextMenu={(event) => result.current.openContextMenu(event, file)}>File</button>
    <button type="button" onContextMenu={(event) => result.current.openRootContextMenu(event)}>Root</button>
  </>);
  const source = screen.getByRole("button", { name: "File" });
  expect(fireEvent.contextMenu(source, { button: 2, clientX: 1200, clientY: 900 })).toBe(false);
  expect(result.current.contextMenu).toEqual({ anchor: source, entry: file, position: { x: 1200, y: 900 } });
  const root = screen.getByRole("button", { name: "Root" });
  vi.spyOn(root, "getBoundingClientRect").mockReturnValue(new DOMRect(30, 40, 100, 36));
  fireEvent.contextMenu(root, { button: 0, clientX: 0, clientY: 0 });
  expect(result.current.contextMenu).toEqual({ anchor: root, entry: null, position: { x: 30, y: 76 } });
  rerender({ scopeKey: "two" });
  expect(result.current.contextMenu).toEqual({ anchor: null, entry: null, position: null });
});
