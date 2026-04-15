"use client";

import { cn } from "@/lib/utils";
import ReactMarkdown from "react-markdown";

import "katex/dist/katex.min.css";

import {
  create_markdown_components,
  MARKDOWN_BODY_CLASS_NAME,
  MARKDOWN_PLUGINS,
  REHYPE_PLUGINS,
  useMarkdownFileResolver,
} from "./markdown-renderer-shared";

interface MarkdownRendererProps {
  content: string;
  class_name?: string;
  is_streaming?: boolean;
  on_open_workspace_file?: (path: string) => void;
}

export function MarkdownRenderer(props: MarkdownRendererProps) {
  const { content, class_name, is_streaming, on_open_workspace_file } = props;
  const resolve_file_path = useMarkdownFileResolver();

  return (
    <div
      className={cn(
        MARKDOWN_BODY_CLASS_NAME,
        is_streaming && "animate-in fade-in-0",
        class_name,
      )}
    >
      <ReactMarkdown
        components={create_markdown_components(resolve_file_path, on_open_workspace_file)}
        rehypePlugins={REHYPE_PLUGINS}
        remarkPlugins={MARKDOWN_PLUGINS}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
