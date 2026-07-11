"use client";

import { useMemo } from "react";
import type { Components } from "react-markdown";

import { cn } from "@/lib/utils";

import { createMarkdownComponents } from "@/shared/ui/markdown/core/markdown-components";
import {
  MARKDOWN_BODY_CLASS_NAME,
  MARKDOWN_PLUGINS,
  normalizeMarkdownContent,
  REHYPE_PLUGINS,
} from "@/shared/ui/markdown/core/markdown-renderer-shared";
import {
  StableMarkdownText,
  StreamingMarkdownText,
} from "@/shared/ui/markdown/streaming/markdown-streaming";
import { useSmoothStreamingMarkdownContent } from "@/shared/ui/markdown/streaming/use-smooth-streaming-markdown-content";
import {
  type MarkdownContentSegment,
  type ResolveWorkspaceFilePath,
  splitMarkdownFileArtifacts,
  useMarkdownCurrentAgentID,
  useMarkdownFileResolver,
} from "@/shared/ui/markdown/workspace/markdown-workspace-artifacts";

import "katex/dist/katex.min.css";

import { FileArtifactBlock } from "./blocks/artifact/file-artifact-block";

interface MarkdownRendererProps {
  content: string;
  className?: string;
  isStreaming?: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  workspaceAgentId?: string | null;
}

export function MarkdownRenderer({
  content,
  className,
  isStreaming = false,
  onOpenWorkspaceFile,
  workspaceAgentId,
}: MarkdownRendererProps) {
  const resolveFilePath = useMarkdownFileResolver(workspaceAgentId);
  const currentAgentId = useMarkdownCurrentAgentID(workspaceAgentId);
  const shouldStream = isStreaming;
  const displayedContent = useSmoothStreamingMarkdownContent(content, shouldStream);
  const components = useMemo(
    () => ({
      stable: createMarkdownComponents(
        resolveFilePath,
        onOpenWorkspaceFile,
        currentAgentId,
      ),
      streaming: createMarkdownComponents(
        resolveFilePath,
        onOpenWorkspaceFile,
        currentAgentId,
        { streamCodeBlocks: true, streamMermaid: true },
      ),
    }),
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
      {contentSegments.map((segment, index) => (
        <MessageMarkdownSegment
          components={components.stable}
          key={`${segment.type}:${index}`}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          resolveFilePath={resolveFilePath}
          segment={segment}
          shouldStream={shouldStream}
          streamingComponents={components.streaming}
          workspaceAgentId={workspaceAgentId}
        />
      ))}
    </div>
  );
}

interface MessageMarkdownSegmentProps {
  components: Components;
  onOpenWorkspaceFile?: (path: string) => void;
  resolveFilePath: ResolveWorkspaceFilePath;
  segment: MarkdownContentSegment;
  shouldStream: boolean;
  streamingComponents: Components;
  workspaceAgentId?: string | null;
}

function MessageMarkdownSegment({
  components,
  onOpenWorkspaceFile,
  resolveFilePath,
  segment,
  shouldStream,
  streamingComponents,
  workspaceAgentId,
}: MessageMarkdownSegmentProps) {
  if (segment.type === "file_artifact") {
    return (
      <FileArtifactBlock
        displayPath={segment.display_path}
        label={segment.label}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        path={segment.path}
        workspaceAgentId={workspaceAgentId}
      />
    );
  }
  if (!segment.text.trim()) {
    return null;
  }

  const sharedProps = {
    components,
    content: normalizeMarkdownContent(
      segment.text,
      resolveFilePath,
      onOpenWorkspaceFile,
      { is_streaming: shouldStream },
    ),
    rehypePlugins: REHYPE_PLUGINS,
    remarkPlugins: MARKDOWN_PLUGINS,
  };
  return shouldStream ? (
    <StreamingMarkdownText
      {...sharedProps}
      streamingComponents={streamingComponents}
    />
  ) : (
    <StableMarkdownText {...sharedProps} />
  );
}
