// INPUT: WorkspaceSurfaceView 的 mobile Header 插槽、标题与正文。
// OUTPUT: 证明移动 Surface 只显示一个标题并保留拖窗合同。
// POS: Workspace Surface DOM 行为测试；业务导航由消费者测试负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { WorkspaceSurfaceView } from "./workspace-surface-view";

describe("WorkspaceSurfaceView", () => {
  it("projects a draggable mobile header without duplicating the title", () => {
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
    screen.getByRole("heading", { name: "子智能体" });
    expect(header?.hasAttribute("data-desktop-window-drag-region")).toBe(true);
    expect(screen.getAllByText("子智能体")).toHaveLength(1);
    expect(screen.getByText("任务目录")).toBeTruthy();
  });

  it("keeps an accessible title when the caller owns its visible header", () => {
    render(
      <WorkspaceSurfaceView title="连接器">
        <p>连接器目录</p>
      </WorkspaceSurfaceView>,
    );

    screen.getByRole("heading", { name: "连接器" });
  });
});
