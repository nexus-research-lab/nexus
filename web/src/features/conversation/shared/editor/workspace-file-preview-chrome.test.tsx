// INPUT: Workspace 文件标题、根名称、相对目录与标题栏动作。
// OUTPUT: 证明层级使用统一面包屑，工具动作使用共享 IconButton 且保持点击/禁用行为。
// POS: Workspace 文件预览 chrome 的 DOM 合同；文件内容与持久化动作由各预览器负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Download } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import {
  WorkspaceFilePreviewHeader,
  WorkspaceFilePreviewHeaderProvider,
  WorkspaceFileToolbarButton,
} from "./workspace-file-preview-chrome";

describe("WorkspaceFilePreviewHeader", () => {
  it("renders one shared chevron between every breadcrumb level", () => {
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: () => undefined, t: (key) => key }}
      >
        <WorkspaceFilePreviewHeaderProvider locationSegments={["nexus", "output"]}>
          <WorkspaceFilePreviewHeader actions={null} title="counter.txt" />
        </WorkspaceFilePreviewHeaderProvider>
      </I18N_CONTEXT.Provider>,
    );

    expect(screen.getByRole("navigation", { name: "common.location_aria" })).toBeTruthy();
    expect(screen.getByText("nexus")).toBeTruthy();
    expect(screen.getByText("output")).toBeTruthy();
    expect(screen.getByText("counter.txt")).toBeTruthy();
    expect(container.querySelectorAll("svg")).toHaveLength(2);
    expect(screen.getByText("counter.txt").getAttribute("aria-current")).toBe("page");
  });

  it("uses the shared compact IconButton for file actions", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    const { rerender } = render(
      <WorkspaceFileToolbarButton onClick={onClick} title="下载文件">
        <Download />
      </WorkspaceFileToolbarButton>,
    );

    const action = screen.getByRole("button", { name: "下载文件" });
    expect(action.className).toContain("h-7");
    expect(action.className).toContain("w-7");
    expect(action.className).toContain("radius-control-sm");
    expect(action.className).not.toContain("rounded-[");
    await user.click(action);
    expect(onClick).toHaveBeenCalledTimes(1);

    rerender(
      <WorkspaceFileToolbarButton disabled onClick={onClick} title="下载文件">
        <Download />
      </WorkspaceFileToolbarButton>,
    );
    await user.click(screen.getByRole("button", { name: "下载文件" }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});
