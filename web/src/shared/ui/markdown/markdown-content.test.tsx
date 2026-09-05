// INPUT: 通用 Markdown 内容及消费侧注入的文件解析、预览和打开能力。
// OUTPUT: 验证链接安全、文件交互和摘要语义不依赖任何业务 Store。
// POS: 共享 Markdown 公共入口的 DOM 行为测试。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UiMarkdownContent } from "./markdown-content";

describe("UiMarkdownContent controlled resources", () => {
  it("renders ordinary links without interpreting conversation links or unsafe URLs", () => {
    render(<UiMarkdownContent content={'[Web](https://example.com/docs) [Section](#section) [Mention](agent-mention://agent-a) [Slash](#nexus-slash-command=goal) [Unsafe](javascript:alert%281%29)'} />);

    expect(screen.getByRole("link", { name: "Web" }).getAttribute("rel")).toBe("noopener noreferrer");
    expect(screen.getByRole("link", { name: "Section" }).getAttribute("href")).toBe("#section");
    expect(screen.getByRole("link", { name: "Slash" }).getAttribute("href")).toBe("#nexus-slash-command=goal");
    expect(screen.queryByRole("link", { name: "Mention" })).toBeNull();
    expect(screen.queryByRole("link", { name: "Unsafe" })).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("uses only injected paths and commands for inline files, linked files and images", async () => {
    const user = userEvent.setup();
    const openFile = vi.fn();
    const resolveFilePath = (path: string) => path === "report.md" ? "reports/report.md" : null;
    const content = '`report.md` [Read](report.md) ![Chart](images/chart.png)';
    const { rerender } = render(
      <UiMarkdownContent content={content} getFilePreviewUrl={(path) => `/preview/a/${path}`} onOpenWorkspaceFile={openFile} resolveFilePath={resolveFilePath} />,
    );

    await user.click(screen.getByRole("button", { name: "report.md" }));
    await user.click(screen.getByRole("button", { name: "Read" }));
    await user.click(screen.getByRole("button", { name: "Chart" }));
    expect(openFile.mock.calls).toEqual([["reports/report.md"], ["reports/report.md"], ["images/chart.png"]]);
    expect(screen.getByRole("img", { name: "Chart" }).getAttribute("src")).toBe("/preview/a/images/chart.png");

    rerender(<UiMarkdownContent content={content} getFilePreviewUrl={(path) => `/preview/b/${path}`} onOpenWorkspaceFile={openFile} resolveFilePath={() => null} />);
    expect(screen.queryByRole("button", { name: "report.md" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Read" })).toBeNull();
    expect(screen.getByRole("img", { name: "Chart" }).getAttribute("src")).toBe("/preview/b/images/chart.png");
  });

  it("keeps summary links and images inert even with file capabilities", () => {
    const openFile = vi.fn();
    render(<UiMarkdownContent content="[Read](report.md) ![Chart](chart.png)" onOpenWorkspaceFile={openFile} resolveFilePath={(path) => path} variant="summary" />);

    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.queryByRole("img")).toBeNull();
    expect(screen.getByText("Read")).toBeTruthy();
    expect(screen.getByText("Chart")).toBeTruthy();
    expect(openFile).not.toHaveBeenCalled();
  });
});
