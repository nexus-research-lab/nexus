// INPUT: 模型公式、普通 Markdown 和连续到达的正文快照。
// OUTPUT: 验证公共公式兼容、局部失败、原文边界及流式/历史一致性。
// POS: #262 的真实 Markdown DOM 回归；不验证像素或宿主外观。

import { render as renderView, screen } from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { THEME_CONTEXT } from "@/shared/theme/theme-context";
import { describe, expect, it } from "vitest";
import { UiMarkdownContent } from "../markdown-content";
import { MarkdownText } from "../streaming/markdown-streaming";
import { MARKDOWN_PLUGINS, REHYPE_PLUGINS } from "./markdown-renderer-shared";

function Providers({ children }: { children: ReactNode }) {
  return <I18nProvider><THEME_CONTEXT.Provider value={{ theme: "light", setTheme: () => undefined }}>{children}</THEME_CONTEXT.Provider></I18nProvider>;
}
const render = (ui: ReactElement) => renderView(ui, { wrapper: Providers });

const sources = (container: HTMLElement) => Array.from(container.querySelectorAll('annotation[encoding="application/x-tex"]'), (node) => node.textContent);

describe("shared model math rendering", () => {
  it("keeps identifier escaping and workspace auto-linking outside formula source", () => {
    const formula = String.raw`f*(x) + \text{report.md}`;
    const { container } = render(<UiMarkdownContent content={String.raw`价格 $20，file*(x) \(` + formula + String.raw`\) report.md`} onOpenWorkspaceFile={() => undefined} resolveFilePath={(path) => path === "report.md" ? path : null} />);
    expect(sources(container)).toEqual([formula]);
    expect(screen.getAllByRole("button", { name: "report.md" })).toHaveLength(1);
    expect(container.textContent).toContain("file*(x)");
  });

  it.each([
    [String.raw`前文 $x_i$ 后文`, "x_i", false],
    [String.raw`前文 \[x_i\] 后文`, "x_i", true],
    ["$$\nx_i\n$$", "x_i", true],
    ["$$x_i$$", "x_i", true],
    ["> $$\n> x_i\n> $$", "x_i", true],
    ["\\[\nx_i\n\n+ y_i\n\\]", "x_i\n\n+ y_i", true],
    [String.raw`- **定义**：\(x_i\)
- 其他内容`, "x_i", false],
    [String.raw`> \[
> \begin{aligned}
> x &= 1 \\
> y &= 2
> \end{aligned}
> \]`, String.raw`\begin{aligned}
x &= 1 \\
y &= 2
\end{aligned}`, true],
  ])("keeps existing math and container semantics: %s", (content, value, display) => {
    const { container } = render(<UiMarkdownContent content={content} />);
    expect(sources(container)).toEqual([value]);
    expect(Boolean(container.querySelector(".katex-display"))).toBe(display);
    if (display) expect(container.querySelector(".katex-display")?.getAttribute("tabindex")).toBe("0");
  });

  it("leaves code, escaped delimiters, links and ordinary prose outside compatibility conversion", () => {
    const content = [String.raw`普通 (K_D)，金额 $20，转义 \\(x_i\\)。`, "`\\(x_i\\)`", "```text\n\\[x_i\\]\n```", String.raw`[文档](https://example.com/\(x\))`].join("\n\n");
    const { container } = render(<UiMarkdownContent content={content} />);
    expect(sources(container)).toEqual([]);
    expect(screen.getByRole("link", { name: "文档" }).getAttribute("href")).toBe("https://example.com/(x)");
    expect(container.textContent).toContain("(K_D)");
    expect(container.textContent).toContain("$20");
    expect(container.textContent).toContain(String.raw`\(x_i\)`);
    expect(container.textContent).toContain(String.raw`\[x_i\]`);
  });

  it("keeps list-contained display math with blank lines in the streaming block", () => {
    const content = "- 定义\n\n  \\[\n  x_i\n\n  + y_i\n  \\]\n\n- 后续";
    const { container } = render(<MarkdownText components={undefined} remarkPlugins={MARKDOWN_PLUGINS} rehypePlugins={REHYPE_PLUGINS} urlTransform={undefined} content={content} isStreaming />);
    expect(sources(container)).toEqual(["  x_i\n\n  + y_i"]);
    expect(container.querySelectorAll("li")).toHaveLength(2);
  });
});
