"use client";

import { useMemo } from "react";
import { cn } from "@/lib/utils";

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
import {
  StableMarkdownText,
  StreamingMarkdownText,
} from "./markdown-streaming";
import { useSmoothStreamingMarkdownContent } from "./use-smooth-streaming-markdown-content";

interface MarkdownRendererProps {
  content: string;
  class_name?: string;
  is_streaming?: boolean;
  mermaid_show_header?: boolean;
  on_open_workspace_file?: (path: string) => void;
  workspace_agent_id?: string | null;
  variant?: "body" | "summary";
}

export function MarkdownRendererContent({
  content,
  class_name,
  is_streaming = false,
  mermaid_show_header = true,
  on_open_workspace_file,
  workspace_agent_id,
  variant = "body",
}: MarkdownRendererProps) {
  const resolve_file_path = useMarkdownFileResolver(workspace_agent_id);
  const current_agent_id = useMarkdownCurrentAgentID(workspace_agent_id);
  const should_stream = Boolean(is_streaming);
  const displayed_content = useSmoothStreamingMarkdownContent(content, should_stream);
  const markdown_components = useMemo(
    () => variant === "summary"
      ? create_markdown_summary_components(resolve_file_path, on_open_workspace_file, current_agent_id)
      : create_markdown_components(
        resolve_file_path,
        on_open_workspace_file,
        current_agent_id,
        { compact_mermaid: false, show_mermaid_header: mermaid_show_header },
      ),
    [current_agent_id, mermaid_show_header, on_open_workspace_file, resolve_file_path, variant],
  );
  const streaming_markdown_components = useMemo(
    () => variant === "summary"
      ? create_markdown_summary_components(resolve_file_path, on_open_workspace_file, current_agent_id)
      : create_markdown_components(
        resolve_file_path,
        on_open_workspace_file,
        current_agent_id,
        {
          compact_mermaid: false,
          show_mermaid_header: mermaid_show_header,
          stream_code_blocks: true,
          stream_mermaid: true,
        },
      ),
    [current_agent_id, mermaid_show_header, on_open_workspace_file, resolve_file_path, variant],
  );
  const normalized_content = normalize_markdown_content(
    displayed_content,
    resolve_file_path,
    on_open_workspace_file,
    { is_streaming: should_stream },
  );
  const shared_props = {
    components: markdown_components,
    content: normalized_content,
    rehype_plugins: REHYPE_PLUGINS,
    remark_plugins: MARKDOWN_PLUGINS,
  };

  return (
    <div
      className={cn(
        variant === "summary" ? MARKDOWN_SUMMARY_CLASS_NAME : MARKDOWN_BODY_CLASS_NAME,
        is_streaming && "animate-in fade-in-0",
        class_name,
      )}
    >
      {should_stream ? (
        <StreamingMarkdownText
          {...shared_props}
          streaming_components={streaming_markdown_components}
        />
      ) : (
        <StableMarkdownText {...shared_props} />
      )}
    </div>
  );
}
