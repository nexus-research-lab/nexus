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

  it("renders issue 262 scientific notation without losing LaTeX escapes", () => {
    const { container } = render(<UiMarkdownContent content={String.raw`解离常数 \(K_D\)，解离速率 \(k_{\mathrm{off}}\)，单位 \(M^{-1}s^{-1}\)。`} />);
    expect(sources(container)).toEqual([String.raw`K_D`, String.raw`k_{\mathrm{off}}`, String.raw`M^{-1}s^{-1}`]);
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

  it("continues workspace linking after ordinary currency text", () => {
    const { container } = render(<UiMarkdownContent content="价格 $20 到 $30，参见 report.md" onOpenWorkspaceFile={() => undefined} resolveFilePath={(path) => path === "report.md" ? path : null} />);
    expect(screen.getByRole("button", { name: "report.md" })).toBeTruthy();
    expect(sources(container)).toEqual([]);
    expect(container.textContent).toContain("$20 到 $30");
  });

  it("preserves incomplete source and replaces it only once the delimiter closes", () => {
    const prefix = String.raw`公式 \(k_{\mathrm{off}}`;
    const { container, rerender } = render(<UiMarkdownContent content={prefix} />);
    expect(sources(container)).toEqual([]);
    expect(container.textContent).toBe(prefix);
    rerender(<UiMarkdownContent content={prefix + String.raw`\) 结束`} />);
    expect(sources(container)).toEqual([String.raw`k_{\mathrm{off}}`]);
    expect(container.textContent).toContain("结束");
  });

  it("contains malformed and untrusted LaTeX without losing surrounding prose", () => {
    const { container } = render(<UiMarkdownContent content={String.raw`前文 \(\frac{1}{\) 中间 \(x_i\) 后文 \(\href{javascript:alert(1)}{link}\)`} />);
    expect(container.querySelector(".katex-error")?.textContent).toContain(String.raw`\frac{1}{`);
    expect(sources(container)).toContain("x_i");
    expect(container.textContent).toContain("前文");
    expect(container.textContent).toContain("后文");
    expect(container.querySelector("a")).toBeNull();
  });

  it("keeps an invalid block fence literal without parser failure", () => {
    const content = "\\[\nx_i\n\\] **后文**";
    const { container } = render(<UiMarkdownContent content={content} />);
    expect(sources(container)).toEqual([]);
    expect(container.textContent).toBe(content);
  });

  it("accepts every incremental prefix of a block formula without inventing a close", () => {
    const content = "\\[\nx_i\n\\]";
    const { container, rerender } = render(<UiMarkdownContent content="" />);
    for (let size = 1; size < content.length; size += 1) {
      rerender(<UiMarkdownContent content={content.slice(0, size)} />);
      expect(sources(container)).toEqual([]);
      expect(container.textContent).toBe(content.slice(0, size));
    }
    rerender(<UiMarkdownContent content={content} />);
    expect(sources(container)).toEqual(["x_i"]);
  });

  it("keeps list-contained display math with blank lines in the streaming block", () => {
    const content = "- 定义\n\n  \\[\n  x_i\n\n  + y_i\n  \\]\n\n- 后续";
    const { container } = render(<MarkdownText components={undefined} streamingComponents={undefined} remarkPlugins={MARKDOWN_PLUGINS} rehypePlugins={REHYPE_PLUGINS} urlTransform={undefined} content={content} isStreaming />);
    expect(sources(container)).toEqual(["  x_i\n\n  + y_i"]);
    expect(container.querySelectorAll("li")).toHaveLength(2);
  });

  it.each(["\\[", "$$"])("keeps display blocks with blank lines intact across streaming completion: %s", (opening) => {
    const closing = opening === "$$" ? "$$" : "\\]";
    const body = "x_i\n\n+ y_i";
    const prefix = `Before\n\n${opening}\n${body}`;
    const props = { components: undefined, streamingComponents: undefined, remarkPlugins: MARKDOWN_PLUGINS, rehypePlugins: REHYPE_PLUGINS, urlTransform: undefined };
    const { container, rerender } = render(<MarkdownText {...props} content={prefix} isStreaming />);
    expect(sources(container)).toEqual([]);
    const content = `${prefix}\n${closing}\n\nAfter`;
    rerender(<MarkdownText {...props} content={content} isStreaming />);
    expect(sources(container)).toEqual([body]);
    const formula = container.querySelector(".katex");
    rerender(<MarkdownText {...props} content={content} isStreaming={false} />);
    expect(container.querySelector(".katex")).toBe(formula);
    expect(container.textContent).toContain("After");
  });
});
