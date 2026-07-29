"use client";

/**
 * INPUT: 已平滑显示的 Markdown 正文、当前流式态与静态/流式组件集。
 * OUTPUT: 组件身份稳定的 Markdown 子树；本次挂载一旦流式过，终态继续保留原分块。
 * POS: 防止打字机排空后重挂载 Markdown 块，同时让初次加载的历史消息直接走静态单块。
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
  const blocks = useMemo(
    () => shouldKeepStreamBlocks
      ? splitStreamingMarkdownBlocks(content)
      : [{
        content,
        start_offset: 0,
        state: "revealed" as const,
      }],
    [content, shouldKeepStreamBlocks],
  );

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
