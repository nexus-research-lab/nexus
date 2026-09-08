// INPUT: 工作区加载标签。
// OUTPUT: 证明工作区占位共享可访问状态、语义排版与 Spinner recipe。
// POS: WorkspaceLoadingState DOM 行为测试；资源读取语义仍由页面控制器负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { WorkspaceLoadingState } from "@/shared/ui/workspace/frame/workspace-loading-state";

describe("WorkspaceLoadingState", () => {
  it("fills its frame with one shared loading presentation", () => {
    const { container } = render(<WorkspaceLoadingState label="加载成员…" />);

    const status = screen.getByRole("status");
    expect(status.getAttribute("aria-busy")).toBe("true");
    expect(screen.getByText("加载成员…").className).toContain("ui-type-supporting");
    const spinner = container.querySelector("svg");
    expect(spinner?.getAttribute("aria-hidden")).toBe("true");
    expect(spinner?.getAttribute("class")).toContain("motion-reduce:animate-none");
    expect(spinner?.getAttribute("class")).toContain("text-(--icon-muted)");
  });
});
