// INPUT: WorkspaceSurfaceView 的 mobile Header 插槽、标题与正文。
// OUTPUT: 证明移动 Surface 复用平台页头几何、排版和拖窗合同。
// POS: Workspace Surface DOM 行为测试；业务导航由消费者测试负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { WorkspaceSurfaceView } from "./workspace-surface-view";

describe("WorkspaceSurfaceView", () => {
  it("projects a platform-aware mobile header without duplicating the title", () => {
    const { container } = render(
      <WorkspaceSurfaceView
        header={{
          kind: "mobile",
          leading: <button type="button">返回</button>,
        }}
        title="子智能体"
      >
        <p>任务目录</p>
      </WorkspaceSurfaceView>,
    );

    const header = container.querySelector("header");
    const heading = screen.getByRole("heading", { name: "子智能体" });
    expect(header?.className).toContain("h-[var(--mobile-shell-header-height,52px)]");
    expect(header?.hasAttribute("data-desktop-window-drag-region")).toBe(true);
    expect(heading.className).toContain("ui-type-section-title");
    expect(screen.getAllByText("子智能体")).toHaveLength(1);
    expect(screen.getByText("任务目录")).toBeTruthy();
  });

  it("uses the shared page-title role for ordinary Workspace pages", () => {
    render(
      <WorkspaceSurfaceView header={{ kind: "page" }} title="连接器">
        <p>连接器目录</p>
      </WorkspaceSurfaceView>,
    );

    const heading = screen.getByRole("heading", { name: "连接器" });
    expect(heading.className).toContain("ui-type-page-title");
    expect(heading.className).toContain("ui-type-tone-strong");
  });
});
