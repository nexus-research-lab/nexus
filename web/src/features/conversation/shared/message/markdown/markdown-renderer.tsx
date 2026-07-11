"use client";

import { useMemo } from "react";
import { cn } from "@/lib/utils";

import "katex/dist/katex.min.css";

import { createMarkdownComponents } from "./core/markdown-components";
import {
  MARKDOWN_BODY_CLASS_NAME,
  MARKDOWN_PLUGINS,
  normalizeMarkdownContent,
  REHYPE_PLUGINS,
} from "./core/markdown-renderer-shared";
import {
  splitMarkdownFileArtifacts,
  useMarkdownCurrentAgentID,
  useMarkdownFileResolver,
} from "./workspace/markdown-workspace-artifacts";
import {
  StableMarkdownText,
  StreamingMarkdownText,
} from "./streaming/markdown-streaming";
import { useSmoothStreamingMarkdownContent } from "./streaming/use-smooth-streaming-markdown-content";
import { FileArtifactBlock } from "../blocks/artifact/file-artifact-block";

interface MarkdownRendererProps {
  content: string;
  className?: string;
  isStreaming?: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  workspaceAgentId?: string | null;
}

export function MarkdownRenderer(props: MarkdownRendererProps) {
  const { content, className: className, isStreaming: isStreaming, onOpenWorkspaceFile: onOpenWorkspaceFile, workspaceAgentId: workspaceAgentId } = props;
  const resolveFilePath = useMarkdownFileResolver(workspaceAgentId);
  const currentAgentId = useMarkdownCurrentAgentID(workspaceAgentId);
  const shouldStream = Boolean(isStreaming);
  const displayedContent = useSmoothStreamingMarkdownContent(content, shouldStream);
  const markdownComponents = useMemo(
    () => createMarkdownComponents(resolveFilePath, onOpenWorkspaceFile, currentAgentId),
    [currentAgentId, onOpenWorkspaceFile, resolveFilePath],
  );
  const streamingMarkdownComponents = useMemo(
    () => createMarkdownComponents(
      resolveFilePath,
      onOpenWorkspaceFile,
      currentAgentId,
      { streamCodeBlocks: true, streamMermaid: true },
    ),
    [currentAgentId, onOpenWorkspaceFile, resolveFilePath],
  );
  const contentSegments = useMemo(
    () => onOpenWorkspaceFile
      ? splitMarkdownFileArtifacts(displayedContent, resolveFilePath)
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
              displayPath={segment.display_path}
              workspaceAgentId={workspaceAgentId}
              onOpenWorkspaceFile={onOpenWorkspaceFile}
            />
          );
        }

        if (!segment.text.trim()) {
          return null;
        }

        const normalizedText = normalizeMarkdownContent(
          segment.text,
          resolveFilePath,
          onOpenWorkspaceFile,
          { is_streaming: shouldStream },
        );
        const key = `text-${index}`;
        const sharedProps = {
          components: markdownComponents,
          content: normalizedText,
          rehypePlugins: REHYPE_PLUGINS,
          remarkPlugins: MARKDOWN_PLUGINS,
        };

        if (shouldStream) {
          return (
            <StreamingMarkdownText
              key={key}
              {...sharedProps}
              streamingComponents={streamingMarkdownComponents}
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
