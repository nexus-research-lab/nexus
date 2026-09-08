// INPUT: 摘要中的模型公式、代码、价格与语言/正文切换。
// OUTPUT: 证明公式摘要只有内联标记，普通文本和正文排版不受影响。
// POS: 共享摘要的内容合同回归；不进行像素或宿主视觉校验。

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { UiMarkdownContent } from "../markdown-content";

describe("compact math summaries", () => {
  it.each([
    "$K_D$",
    "$$K_D$$",
    "$$\nK_D = \\frac{k_d}{k_a}\n$$",
    String.raw`\(K_D\)`,
    String.raw`\[K_D\]`,
    "\\[\nK_D\n\n+ x\n\\]",
    "```math\nK_D\n```",
    String.raw`\(\frac{1}{\)`,
    String.raw`\(unfinished`,
    "$$\nunfinished",
  ])("projects %s to an inline label", (formula) => {
    const { container } = render(<UiMarkdownContent content={`前文\n\n${formula}`} summaryMathLabel="[公式]" variant="summary" />);
    expect(container.textContent).toBe("前文\n[公式]");
    expect(container.querySelector(".katex, math, pre, [tabindex]")).toBeNull();
  });

  it("keeps prices, ordinary brackets, code examples and surrounding prose", () => {
    const content = [String.raw`价格 $20 到 $30，(K_D)，前文 \(x_i\) 后文。`, "`$code$`", "```text\n\\[example\\]\n```"].join("\n\n");
    const { container } = render(<UiMarkdownContent content={content} summaryMathLabel="[公式]" variant="summary" />);
    expect(container.textContent).toBe(["价格 $20 到 $30，(K_D)，前文 [公式] 后文。", "$code$", String.raw`\[example\]`].join("\n"));
    expect(container.querySelector(".katex, math, .nexus-math-pending")).toBeNull();
  });

  it("updates the label and restores full math only when switched to body", () => {
    const content = String.raw`前文 \[K_D = \frac{k_d}{k_a}\] 后文`;
    const { container, rerender } = render(<UiMarkdownContent content={content} summaryMathLabel="[公式]" variant="summary" />);
    expect(container.textContent).toBe("前文 [公式] 后文");
    rerender(<UiMarkdownContent content={content} summaryMathLabel="[Formula]" variant="summary" />);
    expect(container.textContent).toBe("前文 [Formula] 后文");
    rerender(<UiMarkdownContent content={content} variant="body" />);
    expect(container.querySelector(".katex-display")).toBeTruthy();
    expect(container.querySelector("annotation")?.textContent).toBe(String.raw`K_D = \frac{k_d}{k_a}`);
  });
});
