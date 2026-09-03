// INPUT: Workspace 文件标题、根名称与相对目录组成的面包屑。
// OUTPUT: 证明每一级位置都使用统一箭头分隔，而不是混用文本斜杠。
// POS: Workspace 文件预览标题栏的结构测试；文件内容与动作行为由各预览器负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import {
  WorkspaceFilePreviewHeader,
  WorkspaceFilePreviewHeaderProvider,
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
});
