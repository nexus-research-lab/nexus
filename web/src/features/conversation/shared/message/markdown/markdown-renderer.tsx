"use client";

import { useMemo } from "react";
import { cn } from "@/lib/utils";

import "katex/dist/katex.min.css";

import { create_markdown_components } from "./markdown-components";
import {
  MARKDOWN_BODY_CLASS_NAME,
  MARKDOWN_PLUGINS,
  normalize_markdown_content,
  REHYPE_PLUGINS,
} from "./markdown-renderer-shared";
import {
  split_markdown_file_artifacts,
  useMarkdownCurrentAgentID,
  useMarkdownFileResolver,
} from "./markdown-workspace-artifacts";
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
  const { content, class_name: className, is_streaming: isStreaming, on_open_workspace_file: onOpenWorkspaceFile, workspace_agent_id: workspaceAgentId } = props;
  const resolveFilePath = useMarkdownFileResolver(workspaceAgentId);
  const currentAgentId = useMarkdownCurrentAgentID(workspaceAgentId);
  const shouldStream = Boolean(isStreaming);
  const displayedContent = useSmoothStreamingMarkdownContent(content, shouldStream);
  const markdownComponents = useMemo(
    () => create_markdown_components(resolveFilePath, onOpenWorkspaceFile, currentAgentId),
    [currentAgentId, onOpenWorkspaceFile, resolveFilePath],
  );
  const streamingMarkdownComponents = useMemo(
    () => create_markdown_components(
      resolveFilePath,
      onOpenWorkspaceFile,
      currentAgentId,
      { stream_code_blocks: true, stream_mermaid: true },
    ),
    [currentAgentId, onOpenWorkspaceFile, resolveFilePath],
  );
  const contentSegments = useMemo(
    () => onOpenWorkspaceFile
      ? split_markdown_file_artifacts(displayedContent, resolveFilePath)
      : [{ type: "text" as const, text: displayedContent }],
    [displayedContent, onOpenWorkspaceFile, resolveFilePath],
  );

  return (
    <div
      className={cn(
        MARKDOWN_BODY_CLASS_NAME,
        isStreaming && "animate-in fade-in-0",
        className,
      )}
    >
      {contentSegments.map((segment, index) => {
        if (segment.type === "file_artifact") {
          return (
            <FileArtifactBlock
              key={`file-artifact-${index}-${segment.path}`}
              label={segment.label}
              path={segment.path}
              display_path={segment.display_path}
              workspace_agent_id={workspaceAgentId}
              on_open_workspace_file={onOpenWorkspaceFile}
            />
          );
        }

        if (!segment.text.trim()) {
          return null;
        }

        const normalizedText = normalize_markdown_content(
          segment.text,
          resolveFilePath,
          onOpenWorkspaceFile,
          { is_streaming: shouldStream },
        );
        const key = `text-${index}`;
        const sharedProps = {
          components: markdownComponents,
          content: normalizedText,
          rehype_plugins: REHYPE_PLUGINS,
          remark_plugins: MARKDOWN_PLUGINS,
        };

        if (shouldStream) {
          return (
            <StreamingMarkdownText
              key={key}
              {...sharedProps}
              streaming_components={streamingMarkdownComponents}
            />
          );
        }

        return (
          <StableMarkdownText
            key={key}
            {...sharedProps}
          />
        );
      })}
    </div>
  );
}
