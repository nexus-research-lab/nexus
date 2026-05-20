"use client";

import {
  memo,
  useMemo,
  type ComponentProps,
} from "react";
import ReactMarkdown from "react-markdown";

import { split_streaming_markdown_blocks } from "./markdown-stream-blocks";

type ReactMarkdownProps = ComponentProps<typeof ReactMarkdown>;

interface MarkdownTextBlockProps {
  content: string;
  components: ReactMarkdownProps["components"];
  rehype_plugins: ReactMarkdownProps["rehypePlugins"];
  remark_plugins: ReactMarkdownProps["remarkPlugins"];
}

interface StreamingMarkdownTextProps extends MarkdownTextBlockProps {
  streaming_components: ReactMarkdownProps["components"];
}

const MarkdownTextBlock = memo(
  function MarkdownTextBlock({
    content,
    components,
    rehype_plugins,
    remark_plugins,
  }: MarkdownTextBlockProps) {
    if (!content.trim()) {
      return null;
    }

    return (
      <ReactMarkdown
        components={components}
        rehypePlugins={rehype_plugins}
        remarkPlugins={remark_plugins}
      >
        {content}
      </ReactMarkdown>
    );
  },
  (prev, next) =>
    prev.content === next.content &&
    prev.components === next.components &&
    prev.rehype_plugins === next.rehype_plugins &&
    prev.remark_plugins === next.remark_plugins,
);

export function StableMarkdownText(props: MarkdownTextBlockProps) {
  return <MarkdownTextBlock {...props} />;
}

export function StreamingMarkdownText({
  content,
  components,
  streaming_components,
  rehype_plugins,
  remark_plugins,
}: StreamingMarkdownTextProps) {
  const blocks = useMemo(() => split_streaming_markdown_blocks(content), [content]);

  return (
    <>
      {blocks.map((block) => {
        return (
          <MarkdownTextBlock
            key={block.start_offset}
            content={block.content}
            components={block.state === "streaming" ? streaming_components : components}
            rehype_plugins={rehype_plugins}
            remark_plugins={remark_plugins}
          />
        );
      })}
    </>
  );
}
