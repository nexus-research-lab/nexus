"use client";

/**
 * INPUT: 已平滑显示的 Markdown 正文、流式状态与唯一组件表。
 * OUTPUT: 组件身份稳定、与共享展示帧同步提交的 Markdown 子树；尾块流式态经 Context 下发。
 * POS: 防止区块收口或打字机排空后重挂载 Markdown 块；不再为每条流创建独立延迟提交。
 */
import {
  memo,
  useMemo,
  useRef,
  useState,
  type ComponentProps,
} from "react";
import ReactMarkdown from "react-markdown";

import { MarkdownBlockStreamingProvider } from "../core/markdown-block-streaming-context";
import { MarkdownStreamBlockParser } from "./markdown-stream-blocks";

type ReactMarkdownProps = ComponentProps<typeof ReactMarkdown>;

interface MarkdownTextBlockProps {
  content: string;
  components: ReactMarkdownProps["components"];
  rehypePlugins: ReactMarkdownProps["rehypePlugins"];
  remarkPlugins: ReactMarkdownProps["remarkPlugins"];
  urlTransform: ReactMarkdownProps["urlTransform"];
}

interface MarkdownTextProps extends MarkdownTextBlockProps {
  isStreaming: boolean;
}

const MarkdownTextBlock = memo(
  function MarkdownTextBlock({
    content,
    components,
    rehypePlugins: rehypePlugins,
    remarkPlugins: remarkPlugins,
    urlTransform,
  }: MarkdownTextBlockProps) {
    if (!content.trim()) {
      return null;
    }

    return (
      <ReactMarkdown
        components={components}
        rehypePlugins={rehypePlugins}
        remarkPlugins={remarkPlugins}
        urlTransform={urlTransform}
      >
        {content}
      </ReactMarkdown>
    );
  },
  (prev, next) =>
    prev.content === next.content &&
    prev.components === next.components &&
    prev.rehypePlugins === next.rehypePlugins &&
    prev.remarkPlugins === next.remarkPlugins &&
    prev.urlTransform === next.urlTransform,
);

export function MarkdownText({
  content,
  components,
  isStreaming,
  rehypePlugins: rehypePlugins,
  remarkPlugins: remarkPlugins,
  urlTransform,
}: MarkdownTextProps) {
  const [blockParser] = useState(() => new MarkdownStreamBlockParser());
  const hasEverStreamedRef = useRef(isStreaming);
  if (isStreaming) {
    hasEverStreamedRef.current = true;
  }
  const shouldKeepStreamBlocks = hasEverStreamedRef.current;
  const nextBlocks = useMemo(
    () => shouldKeepStreamBlocks
      ? blockParser.parse(content)
      : [{
        content,
        start_offset: 0,
        state: "revealed" as const,
      }],
    [blockParser, content, shouldKeepStreamBlocks],
  );
  const blocks = nextBlocks;

  return (
    <>
      {blocks.map((block) => (
        // 元素类型在区块收口前后保持不变；只有 Provider 值变化，代码块与图表原位切换。
        <MarkdownBlockStreamingProvider
          key={block.start_offset}
          value={isStreaming && block.state === "streaming"}
        >
          <MarkdownTextBlock
            content={block.content}
            components={components}
            rehypePlugins={rehypePlugins}
            remarkPlugins={remarkPlugins}
            urlTransform={urlTransform}
          />
        </MarkdownBlockStreamingProvider>
      ))}
    </>
  );
}
