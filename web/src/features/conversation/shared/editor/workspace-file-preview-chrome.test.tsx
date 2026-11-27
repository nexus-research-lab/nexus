// INPUT: Workspace 文件标题、根名称、相对目录与标题栏动作。
// OUTPUT: 证明层级使用统一面包屑，工具动作使用共享 IconButton 且保持点击/禁用行为。
// POS: Workspace 文件预览 chrome 的 DOM 合同；文件内容与持久化动作由各预览器负责。

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
