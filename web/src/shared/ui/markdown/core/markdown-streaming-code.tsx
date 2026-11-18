/**
 * INPUT: 代码块/Mermaid 原文与所在 Markdown 区块的流式 Context。
 * OUTPUT: 读取区块流式态的代码块与图表视图。
 * POS: 模块级组件身份跨越 streaming → revealed 不变，收口只更新状态、不重挂载外层 DOM。
 */
import { CodeBlock } from "../code/code-block";
import { LazyMermaidView } from "../mermaid/lazy-mermaid-view";
import { useMarkdownBlockStreaming } from "./markdown-block-streaming-context";

export function StreamingAwareCodeBlock({ language, value }: { language: string; value: string }) {
  return <CodeBlock isStreaming={useMarkdownBlockStreaming()} language={language} value={value} />;
}

export function StreamingAwareMermaid({ chart, compact, showHeader }: { chart: string; compact: boolean; showHeader?: boolean }) {
  return <LazyMermaidView chart={chart} compact={compact} isStreaming={useMarkdownBlockStreaming()} showHeader={showHeader} />;
}
