/**
 * INPUT: 流式 Markdown 区块是否仍在增长。
 * OUTPUT: 代码块与 Mermaid 读取的区块级流式状态。
 * POS: 让静态与流式区块共用同一组件表；区块收口只改变状态，不替换元素类型、不重挂载 DOM。
 */
import { createContext, useContext } from "react";

const MarkdownBlockStreamingContext = createContext(false);

export const MarkdownBlockStreamingProvider = MarkdownBlockStreamingContext.Provider;

export function useMarkdownBlockStreaming(): boolean {
  return useContext(MarkdownBlockStreamingContext);
}
