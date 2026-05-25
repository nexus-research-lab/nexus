"use client";

import { useMemo } from "react";
import { cn } from "@/lib/utils";

import "katex/dist/katex.min.css";

import {
  create_markdown_components,
  MARKDOWN_BODY_CLASS_NAME,
  MARKDOWN_PLUGINS,
  normalize_markdown_content,
  REHYPE_PLUGINS,
  split_markdown_file_artifacts,
  useMarkdownCurrentAgentID,
  useMarkdownFileResolver,
} from "./markdown-renderer-shared";
import {
  StableMarkdownText,
  StreamingMarkdownText,
} from "./markdown-streaming";
import { useSmoothStreamingMarkdownContent } from "./use-smooth-streaming-markdown-content";
import { FileArtifactBlock } from "../blocks/file-artifact-block";

interface MarkdownRendererProps {
  content: string;
  class_name?: string;
  is_streaming?: boolean;
  on_open_workspace_file?: (path: string) => void;
  workspace_agent_id?: string | null;
}

export function MarkdownRenderer(props: MarkdownRendererProps) {
  const { content, class_name, is_streaming, on_open_workspace_file, workspace_agent_id } = props;
  const resolve_file_path = useMarkdownFileResolver(workspace_agent_id);
  const current_agent_id = useMarkdownCurrentAgentID(workspace_agent_id);
  const should_stream = Boolean(is_streaming);
  const displayed_content = useSmoothStreamingMarkdownContent(content, should_stream);
  const markdown_components = useMemo(
    () => create_markdown_components(resolve_file_path, on_open_workspace_file, current_agent_id),
    [current_agent_id, on_open_workspace_file, resolve_file_path],
  );
  const streaming_markdown_components = useMemo(
    () => create_markdown_components(
      resolve_file_path,
      on_open_workspace_file,
      current_agent_id,
      { stream_code_blocks: true, stream_mermaid: true },
    ),
    [current_agent_id, on_open_workspace_file, resolve_file_path],
  );
  const content_segments = useMemo(
    () => on_open_workspace_file
      ? split_markdown_file_artifacts(displayed_content, resolve_file_path)
      : [{ type: "text" as const, text: displayed_content }],
    [displayed_content, on_open_workspace_file, resolve_file_path],
  );

  return (
    <div
      className={cn(
        MARKDOWN_BODY_CLASS_NAME,
        is_streaming && "animate-in fade-in-0",
        class_name,
      )}
    >
      {content_segments.map((segment, index) => {
        if (segment.type === "file_artifact") {
          return (
            <FileArtifactBlock
              key={`file-artifact-${index}-${segment.path}`}
              label={segment.label}
              path={segment.path}
              display_path={segment.display_path}
              workspace_agent_id={workspace_agent_id}
              on_open_workspace_file={on_open_workspace_file}
            />
          );
        }

        if (!segment.text.trim()) {
          return null;
        }

        const normalized_text = normalize_markdown_content(
          segment.text,
          resolve_file_path,
          on_open_workspace_file,
          { is_streaming: should_stream },
        );
        const key = `text-${index}`;
        const shared_props = {
          components: markdown_components,
          content: normalized_text,
          rehype_plugins: REHYPE_PLUGINS,
          remark_plugins: MARKDOWN_PLUGINS,
        };

        if (should_stream) {
          return (
            <StreamingMarkdownText
              key={key}
              {...shared_props}
              streaming_components={streaming_markdown_components}
            />
          );
        }

        return (
          <StableMarkdownText
            key={key}
            {...shared_props}
          />
        );
      })}
    </div>
  );
}
