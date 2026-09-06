// INPUT: 目录面尺寸、布局断点和用户拖动事件。
// OUTPUT: 宽度边界、失焦/停用结束与恢复后不续拖的回归。
// POS: 文件列表尺寸控制器功能测试，不运行浏览器布局。

import { act, fireEvent, renderHook } from "@testing-library/react";
import { expect, it, vi } from "vitest";

import { useMediaQuery } from "@/shared/lib/react/use-media-query";

import { useWorkspaceFileListLayout } from "./use-workspace-file-list-layout";

vi.mock("@/shared/lib/react/use-media-query", () => ({ useMediaQuery: vi.fn(() => false) }));

it("preserves width limits and stops resizing after blur or pane deactivation", () => {
  vi.mocked(useMediaQuery).mockReturnValue(false);
  const { result, rerender } = renderHook(({ enabled }) => useWorkspaceFileListLayout(enabled), {
    initialProps: { enabled: true },
  });
  const panel = document.createElement("div");
  const bounds = vi.spyOn(panel, "getBoundingClientRect").mockReturnValue({ right: 1000, width: 800 } as DOMRect);
  result.current.panelRef.current = panel;
  act(() => result.current.startResizing());
  fireEvent.mouseMove(window, { buttons: 1, clientX: 600 });
  expect(result.current.width).toBe(360);
  fireEvent.mouseMove(window, { buttons: 1, clientX: 950 });
  expect(result.current.width).toBe(200);
  fireEvent.blur(window);
  expect(result.current.isResizing).toBe(false);
  fireEvent.mouseMove(window, { buttons: 1, clientX: 700 });
  expect(result.current.width).toBe(200);

  vi.mocked(useMediaQuery).mockReturnValue(true);
  rerender({ enabled: true });
  act(() => result.current.startResizing());
  fireEvent.mouseMove(window, { buttons: 1, clientX: 950 });
  expect(result.current.width).toBe(160);
  fireEvent.mouseMove(window, { buttons: 1, clientX: 600 });
  expect(result.current.width).toBe(280);
  bounds.mockReturnValue({ right: 0, width: 0 } as DOMRect);
  fireEvent.mouseMove(window, { buttons: 1, clientX: 0 });
  expect(result.current.width).toBe(280);
  rerender({ enabled: false });
  expect(result.current.isResizing).toBe(false);
  expect(result.current.resizeControl).toBeNull();
  rerender({ enabled: true });
  expect(result.current.isResizing).toBe(false);
  act(() => result.current.resizeControl?.onChange(230));
  expect(result.current.width).toBe(230);
  act(() => result.current.resizeControl?.onChange(100));
  expect(result.current.width).toBe(160);
  act(() => result.current.resizeControl?.onChange(Number.NaN));
  expect(result.current.width).toBe(160);
});
