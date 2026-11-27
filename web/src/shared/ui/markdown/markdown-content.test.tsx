// INPUT: 通用 Markdown 内容及消费侧注入的文件解析、预览和打开能力。
// OUTPUT: 验证链接安全、文件交互和摘要语义不依赖任何业务 Store。
// POS: 共享 Markdown 公共入口的 DOM 行为测试。

import { render as renderDom, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { ReactNode } from "react";
import { I18nProvider } from "@/shared/i18n/i18n-provider";

function render(node: ReactNode) { return renderDom(<I18nProvider>{node}</I18nProvider>); }

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

it.each([false, true])("keeps headings and table headers regular when summary emphasis is disabled (monochrome=%s)", (monochrome) => {
  const content = "# Heading\n\n**Strong**\n\n| Header |\n| --- |\n| Cell |";
  const { container, rerender } = render(<UiMarkdownContent content={content} variant="summary" summaryMonochrome={monochrome} summaryStrongAsText />);

  expect(container.querySelector("strong")).toBeNull();
  rerender(<I18nProvider><UiMarkdownContent content={content} variant="summary" summaryMonochrome={monochrome} /></I18nProvider>);

  expect(container.querySelector("strong")?.textContent).toBe("Strong");
});
