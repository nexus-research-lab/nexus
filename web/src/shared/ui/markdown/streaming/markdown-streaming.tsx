"use client";

import {
  memo,
  useMemo,
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

interface StreamingMarkdownTextProps extends MarkdownTextBlockProps {
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

export function StableMarkdownText(props: MarkdownTextBlockProps) {
  return <MarkdownTextBlock {...props} />;
}

export function StreamingMarkdownText({
  content,
  components,
  streamingComponents: streamingComponents,
  rehypePlugins: rehypePlugins,
  remarkPlugins: remarkPlugins,
  urlTransform,
}: StreamingMarkdownTextProps) {
  const blocks = useMemo(() => splitStreamingMarkdownBlocks(content), [content]);

  return (
    <>
      {blocks.map((block) => {
        return (
          <MarkdownTextBlock
            key={block.start_offset}
            content={block.content}
            components={block.state === "streaming" ? streamingComponents : components}
            rehypePlugins={rehypePlugins}
            remarkPlugins={remarkPlugins}
            urlTransform={urlTransform}
          />
        );
      })}
    </>
  );
}
