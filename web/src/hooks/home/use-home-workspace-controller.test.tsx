// INPUT: 首页辅助面板容器与鼠标拖动/失焦事件。
// OUTPUT: 百分比边界、零宽容器和拖动终止的接入回归。
// POS: 首页工作区尺寸协调测试；没有 Agent 时不启动文件请求。

import { act, fireEvent, renderHook } from "@testing-library/react";
import { expect, it, vi } from "vitest";

import { useHomeWorkspaceController } from "./use-home-workspace-controller";

it("bounds auxiliary width, ignores zero-width containers and ends on blur", () => {
  const { result } = renderHook(() => useHomeWorkspaceController({ currentAgentId: null }));
  const panel = document.createElement("div");
  const bounds = vi.spyOn(panel, "getBoundingClientRect").mockReturnValue({ right: 1000, width: 1000 } as DOMRect);
  result.current.surfaceSplitRef.current = panel;
  act(() => result.current.handleStartSidePanelResize());
  fireEvent.mouseMove(window, { buttons: 1, clientX: 600 });
  expect(result.current.sidePanelWidthPercent).toBe(40);
  fireEvent.mouseMove(window, { buttons: 1, clientX: 950 });
  expect(result.current.sidePanelWidthPercent).toBe(30);
  fireEvent.mouseMove(window, { buttons: 1, clientX: 0 });
  expect(result.current.sidePanelWidthPercent).toBe(56);
  bounds.mockReturnValue({ right: 0, width: 0 } as DOMRect);
  fireEvent.mouseMove(window, { buttons: 1, clientX: 0 });
  expect(result.current.sidePanelWidthPercent).toBe(56);
  fireEvent.blur(window);
  expect(result.current.isResizingSidePanel).toBe(false);
  bounds.mockReturnValue({ right: 1000, width: 1000 } as DOMRect);
  fireEvent.mouseMove(window, { buttons: 1, clientX: 600 });
  expect(result.current.sidePanelWidthPercent).toBe(56);
});
