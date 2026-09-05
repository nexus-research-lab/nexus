// INPUT: WorkGraph 视口命令、搜索状态与共享本地化。
// OUTPUT: 证明画布控制条复用标准动作/浮层/排版，并保留搜索键盘与禁用行为。
// POS: WorkGraph 控制条 DOM 行为测试；缩放和搜索算法由 interaction model 测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { ExecutionWorkGraphControls } from "./execution-workgraph-controls";

function renderControls(overrides: Partial<Parameters<typeof ExecutionWorkGraphControls>[0]> = {}) {
  const callbacks = {
    onCollapseAll: vi.fn(),
    onExpandAll: vi.fn(),
    onFit: vi.fn(),
    onLocateCurrent: vi.fn(),
    onNextResult: vi.fn(),
    onOpenExpanded: vi.fn(),
    onPreviousResult: vi.fn(),
    onQueryChange: vi.fn(),
    onResetZoom: vi.fn(),
    onZoomIn: vi.fn(),
    onZoomOut: vi.fn(),
  };
  const result = render(
    <I18nProvider>
      <ExecutionWorkGraphControls
        collapsibleCount={2}
        collapsedCount={0}
        currentResultIndex={0}
        query=""
        resultCount={0}
        zoom={1}
        {...callbacks}
        {...overrides}
      />
    </I18nProvider>,
  );
  return { ...callbacks, ...result };
}

describe("ExecutionWorkGraphControls", () => {
  it("uses shared compact action and popover geometry", () => {
    const { container, onResetZoom, onZoomOut } = renderControls();
    const controls = container.querySelector("[data-execution-workgraph-controls]");
    const toolbar = controls?.firstElementChild;
    const zoomOut = screen.getByRole("button", { name: /zoom out|缩小/i });
    const zoomReset = screen.getByRole("button", { name: /reset to 100%|重置为 100%/i });

    expect(toolbar?.className).toContain("surface-popover");
    expect(toolbar?.className).toContain("surface-radius-sm");
    expect(zoomOut.className).toContain("h-7");
    expect(zoomOut.className).toContain("radius-control-sm");
    expect(zoomReset.className).toContain("ui-type-caption");
    expect(zoomReset.className).toContain("radius-control-xs");

    fireEvent.click(zoomOut);
    fireEvent.click(zoomReset);
    expect(onZoomOut).toHaveBeenCalledOnce();
    expect(onResetZoom).toHaveBeenCalledOnce();
  });

  it("focuses search, preserves keyboard navigation, and disables empty result actions", () => {
    const {
      container,
      onNextResult,
      onPreviousResult,
      onQueryChange,
    } = renderControls();

    fireEvent.click(screen.getByRole("button", { name: /search workgraph|搜索工作图/i }));
    const input = screen.getByRole("searchbox", { name: /search workgraph|搜索工作图/i });
    const searchShell = input.parentElement;
    const searchSurface = searchShell?.parentElement;

    expect(document.activeElement).toBe(input);
    expect(searchShell?.className).toContain("ui-type-control");
    expect(searchShell?.className).toContain("h-7");
    expect(searchSurface?.className).toContain("surface-popover");
    expect(container.querySelectorAll("button:disabled")).toHaveLength(2);

    fireEvent.keyDown(input, { key: "Enter" });
    fireEvent.keyDown(input, { key: "Enter", shiftKey: true });
    expect(onNextResult).toHaveBeenCalledOnce();
    expect(onPreviousResult).toHaveBeenCalledOnce();

    fireEvent.keyDown(input, { key: "Escape" });
    expect(onQueryChange).toHaveBeenCalledWith("");
    expect(screen.queryByRole("searchbox", { name: /search workgraph|搜索工作图/i })).toBeNull();
  });
});
