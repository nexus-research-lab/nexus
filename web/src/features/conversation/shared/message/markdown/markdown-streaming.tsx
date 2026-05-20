"use client";

import {
  memo,
  useMemo,
  type ComponentProps,
} from "react";
import ReactMarkdown from "react-markdown";

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

interface StreamingMarkdownBlock {
  content: string;
  start_offset: number;
}

function get_lines_with_endings(content: string): string[] {
  return content.match(/[^\n]*(?:\n|$)/g)?.filter((line) => line.length > 0) ?? [];
}

function is_blank_line(line: string): boolean {
  return line.trim().length === 0;
}

function is_standalone_block_line(line: string): boolean {
  return /^ {0,3}#{1,6}\s+\S/.test(line) || /^ {0,3}(?:-{3,}|\*{3,}|_{3,})\s*$/.test(line);
}

function read_fence_marker(line: string): { marker: "`" | "~"; length: number } | null {
  const match = /^ {0,3}(`{3,}|~{3,})/.exec(line);
  if (!match) {
    return null;
  }
  const marker_text = match[1];
  return {
    marker: marker_text[0] as "`" | "~",
    length: marker_text.length,
  };
}

function split_streaming_markdown_blocks(content: string): StreamingMarkdownBlock[] {
  const blocks: StreamingMarkdownBlock[] = [];
  const buffer: string[] = [];
  let block_start_offset = 0;
  let cursor_offset = 0;
  let open_fence: { marker: "`" | "~"; length: number } | null = null;

  const flush_buffer = () => {
    if (buffer.length === 0) {
      block_start_offset = cursor_offset;
      return;
    }

    blocks.push({
      content: buffer.join(""),
      start_offset: block_start_offset,
    });
    buffer.length = 0;
    block_start_offset = cursor_offset;
  };

  for (const line of get_lines_with_endings(content)) {
    const fence_marker = read_fence_marker(line);

    buffer.push(line);
    cursor_offset += line.length;

    if (open_fence) {
      if (
        fence_marker &&
        fence_marker.marker === open_fence.marker &&
        fence_marker.length >= open_fence.length
      ) {
        open_fence = null;
        flush_buffer();
      }
      continue;
    }

    if (fence_marker) {
      open_fence = fence_marker;
      continue;
    }

    if (is_blank_line(line) || (buffer.length === 1 && is_standalone_block_line(line))) {
      flush_buffer();
    }
  }

  flush_buffer();
  return blocks;
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
      {blocks.map((block, index) => {
        const is_tail_block = index === blocks.length - 1;
        return (
          <MarkdownTextBlock
            key={block.start_offset}
            content={block.content}
            components={is_tail_block ? streaming_components : components}
            rehype_plugins={rehype_plugins}
            remark_plugins={remark_plugins}
          />
        );
      })}
    </>
  );
}
