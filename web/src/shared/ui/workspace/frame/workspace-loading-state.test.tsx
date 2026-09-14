// INPUT: 工作区加载标签。
// OUTPUT: 证明工作区占位暴露可访问的忙碌状态与装饰图标。
// POS: WorkspaceLoadingState DOM 行为测试；资源读取语义仍由页面控制器负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { WorkspaceLoadingState } from "@/shared/ui/workspace/frame/workspace-loading-state";

describe("WorkspaceLoadingState", () => {
  it("announces its loading state", () => {
    const { container } = render(<WorkspaceLoadingState label="加载成员…" />);

    const status = screen.getByRole("status");
    expect(status.getAttribute("aria-busy")).toBe("true");
    expect(status.textContent).toContain("加载成员…");
    const spinner = container.querySelector("svg");
    expect(spinner?.getAttribute("aria-hidden")).toBe("true");
  });
});
