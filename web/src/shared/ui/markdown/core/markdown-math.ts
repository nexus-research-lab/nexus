// INPUT: Markdown 数学 token、原始正文、摘要标记和 KaTeX 生成树。
// OUTPUT: 正文完整公式/未闭合原文、纯文本公式摘要及可聚焦的正文横向公式视口。
// POS: 共享公式兼容/降级策略；不修补公式内容或改变模型/历史数据。

import type { Nodes, Root } from "mdast";
import type { Extension as FromMarkdownExtension } from "mdast-util-from-markdown";
import type { Extension } from "micromark-util-types";
import type { Processor } from "unified";
import { LATEX_MATH_SYNTAX } from "./markdown-latex-syntax";

declare module "mdast" {
  interface Data {
    /** An ambiguous dollar token retained as literal prose, not a pending formula. */
    nexusMathLiteral?: boolean;
  }
}

const latexFromMarkdown: FromMarkdownExtension = {
  enter: { nexusMath(token) {
    const math = token._nexusMath!;
    const raw = this.sliceSerialize(token);
    const value = math.flow ? math.value : raw.slice(2, math.closed ? -2 : undefined);
    this.enter({
      type: math.flow ? "math" : "inlineMath",
      value,
      data: {
        hName: "span",
        hProperties: { className: [math.closed ? (math.display ? "math-display" : "math-inline") : "nexus-math-pending"] },
        hChildren: [{ type: "text", value: math.closed ? value : raw }],
      },
    }, token);
    this.buffer();
  } },
  exit: { nexusMath(token) {
    this.resume();
    this.exit(token);
  } },
};

export function remarkLatexMath(this: Processor) {
  const data = this.data() as { micromarkExtensions?: Extension[]; fromMarkdownExtensions?: FromMarkdownExtension[] };
  (data.micromarkExtensions ??= []).push(LATEX_MATH_SYNTAX);
  (data.fromMarkdownExtensions ??= []).push(latexFromMarkdown);
  return (tree: Root, file: { value: unknown }) => {
    const source = String(file.value);
    function visit(node: Nodes) {
      if ((node.type === "math" || node.type === "inlineMath") && node.position && !node.data?.hProperties?.className?.toString().includes("nexus-math-pending")) {
        const raw = source.slice(node.position.start.offset, node.position.end.offset);
        // remark-math accepts unterminated block fences. Streaming keeps these literal until closed.
        const ambiguousPrice = node.type === "inlineMath" && /^\$\d/.test(raw) && /\d/.test(source[node.position.end.offset ?? source.length] ?? "");
        if (ambiguousPrice || (node.type === "math" && raw.startsWith("$$") && !hasClosingDollarFence(raw))) {
          node.data = { nexusMathLiteral: ambiguousPrice, hName: "span", hProperties: { className: ["nexus-math-pending"] }, hChildren: [{ type: "text", value: raw }] };
        } else if (node.type === "inlineMath" && raw.startsWith("$$")) {
          node.data!.hProperties = { className: ["math-display"] };
        }
      }
      if ("children" in node) node.children.forEach(visit);
    }
    visit(tree);
  };
}

// 摘要复用解析结果，省去 KaTeX 排版；价格歧义与普通代码保持文本语义。
export function remarkMathSummary({ label }: { label: string }) {
  return (tree: Root) => {
    function visit(node: Nodes) {
      if (!("children" in node)) return;
      node.children.forEach((child, index) => {
        const isMath = child.type === "math" || child.type === "inlineMath";
        if (isMath || (child.type === "code" && child.lang === "math")) {
          const literal = isMath && child.data?.nexusMathLiteral ? child.data.hChildren?.[0] : undefined;
          // Literal prices also lose the body's pending-math wrapping style in a compact summary.
          node.children[index] = { type: "text", value: literal?.type === "text" ? literal.value : label };
        } else {
          visit(child);
        }
      });
    }
    visit(tree);
  };
}

function hasClosingDollarFence(raw: string): boolean {
  const opening = raw.match(/^\$+/)?.[0].length ?? 2;
  return raw.split(/\r?\n/).slice(1).some((line) => {
    const close = line.match(/^(?:[ \t]*>[ \t]?)*[ \t]*(\${2,})[ \t]*$/);
    return Boolean(close && close[1].length >= opening);
  });
}

interface MathElement {
  type: string;
  properties?: Record<string, unknown>;
  children?: MathElement[];
}

export function rehypeMathViewport() {
  return (tree: MathElement) => {
    function visit(node: MathElement) {
      if (Array.isArray(node.properties?.className) && node.properties.className.includes("katex-display")) {
        node.properties.tabIndex = 0;
      }
      node.children?.forEach(visit);
    }
    visit(tree);
  };
}
