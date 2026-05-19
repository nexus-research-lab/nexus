"use client";

import { cn } from "@/lib/utils";
import ReactMarkdown from "react-markdown";

import "katex/dist/katex.min.css";
import {
  create_markdown_components,
  create_markdown_summary_components,
  MARKDOWN_BODY_CLASS_NAME,
  MARKDOWN_SUMMARY_CLASS_NAME,
  MARKDOWN_PLUGINS,
  normalize_markdown_content,
  REHYPE_PLUGINS,
  useMarkdownCurrentAgentID,
  useMarkdownFileResolver,
} from "./markdown-renderer-shared";

interface MarkdownRendererProps {
  content: string;
  class_name?: string;
  is_streaming?: boolean;
  on_open_workspace_file?: (path: string) => void;
  workspace_agent_id?: string | null;
  variant?: "body" | "summary";
}

export function MarkdownRendererContent({
  content,
  class_name,
  is_streaming = false,
  on_open_workspace_file,
  workspace_agent_id,
  variant = "body",
}: MarkdownRendererProps) {
  const resolve_file_path = useMarkdownFileResolver(workspace_agent_id);
  const current_agent_id = useMarkdownCurrentAgentID(workspace_agent_id);
  const markdown_components = variant === "summary"
    ? create_markdown_summary_components(resolve_file_path, on_open_workspace_file, current_agent_id)
    : create_markdown_components(resolve_file_path, on_open_workspace_file, current_agent_id);
  const normalized_content = normalize_markdown_content(content, resolve_file_path, on_open_workspace_file);

  return (
    <div
      className={cn(
        variant === "summary" ? MARKDOWN_SUMMARY_CLASS_NAME : MARKDOWN_BODY_CLASS_NAME,
        is_streaming && "animate-in fade-in-0",
        class_name,
      )}
    >
      <ReactMarkdown
        components={markdown_components}
        rehypePlugins={REHYPE_PLUGINS}
        remarkPlugins={MARKDOWN_PLUGINS}
      >
        {normalized_content}
      </ReactMarkdown>
    </div>
  );
}
