"use client";

/**
 * INPUT: 已平滑显示的 Markdown 正文、流式状态与静态/流式组件集。
 * OUTPUT: 组件身份稳定、与共享展示帧同步提交的 Markdown 子树。
 * POS: 防止打字机排空后重挂载 Markdown 块；不再为每条流创建独立延迟提交。
 */
import {
  memo,
  useMemo,
  useRef,
  type ComponentProps,
} from "react";
import ReactMarkdown from "react-markdown";

import { splitStreamingMarkdownBlocks } from "./markdown-stream-blocks";

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
  streamingComponents: ReactMarkdownProps["components"];
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
  streamingComponents: streamingComponents,
  rehypePlugins: rehypePlugins,
  remarkPlugins: remarkPlugins,
  urlTransform,
}: MarkdownTextProps) {
  const hasEverStreamedRef = useRef(isStreaming);
  if (isStreaming) {
    hasEverStreamedRef.current = true;
  }
  const shouldKeepStreamBlocks = hasEverStreamedRef.current;
  const nextBlocks = useMemo(
    () => shouldKeepStreamBlocks
      ? splitStreamingMarkdownBlocks(content)
      : [{
        content,
        start_offset: 0,
        state: "revealed" as const,
      }],
    [content, shouldKeepStreamBlocks],
  );
  const blocks = nextBlocks;

  return (
    <>
      {blocks.map((block) => {
        return (
          <MarkdownTextBlock
            key={block.start_offset}
            content={block.content}
            components={
              isStreaming && block.state === "streaming"
                ? streamingComponents
                : components
            }
            rehypePlugins={rehypePlugins}
            remarkPlugins={remarkPlugins}
            urlTransform={urlTransform}
          />
        );
      })}
    </>
  );
}
