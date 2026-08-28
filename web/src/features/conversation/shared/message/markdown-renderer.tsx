/**
 * INPUT: 消息 Markdown、Workspace 文件解析与服务端 Agent mention spans。
 * OUTPUT: 文件产物链接和携带 handoff_id 的稳定 mention 链接。
 * POS: 会话消息协议到共享 Markdown 渲染器的适配层。
 */
"use client";

import { useMemo } from "react";
import type { Components } from "react-markdown";

import { cn } from "@/shared/ui/class-name";

import { createMarkdownComponents } from "@/shared/ui/markdown/core/markdown-components";
import {
  MARKDOWN_BODY_CLASS_NAME,
  MARKDOWN_PLUGINS,
  normalizeMarkdownContent,
  REHYPE_PLUGINS,
  transformMarkdownUrl,
} from "@/shared/ui/markdown/core/markdown-renderer-shared";
import { MarkdownText } from "@/shared/ui/markdown/streaming/markdown-streaming";
import { useSmoothStreamingMarkdownState } from "@/shared/ui/markdown/streaming/use-smooth-streaming-markdown-content";
import {
  type MarkdownContentSegment,
  type ResolveWorkspaceFilePath,
  splitMarkdownFileArtifacts,
} from "@/shared/ui/markdown/workspace/markdown-workspace-artifact-model";
import {
  useMarkdownCurrentAgentID,
  useMarkdownFileResolver,
} from "@/shared/ui/markdown/workspace/use-markdown-workspace-files";

import "katex/dist/katex.min.css";

import { FileArtifactBlock } from "./blocks/artifact/file/file-artifact-block";
import type { AgentMention } from "@/types/conversation/message/entity";
import type { AgentMentionDirectory } from "./agent-mention-chip";
import { decorateLeadingSlashCommand } from "../slash-command-presentation";

interface MarkdownRendererProps {
  content: string;
  className?: string;
  initialRevealFromEmpty?: boolean;
  isStreaming?: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
	workspaceAgentId?: string | null;
	agentMentions?: AgentMention[];
	agentMentionDirectory?: AgentMentionDirectory;
	onOpenAgentContact?: (agentId: string) => void;
  renderLeadingSlashCommand?: boolean;
}

export function MarkdownRenderer({
  content,
  className,
  initialRevealFromEmpty = false,
  isStreaming = false,
  onOpenWorkspaceFile,
	workspaceAgentId,
	agentMentions = [],
	agentMentionDirectory,
	onOpenAgentContact,
  renderLeadingSlashCommand = false,
}: MarkdownRendererProps) {
  const resolveFilePath = useMarkdownFileResolver(workspaceAgentId);
  const currentAgentId = useMarkdownCurrentAgentID(workspaceAgentId);
  const shouldStream = isStreaming;
  const smoothStreaming = useSmoothStreamingMarkdownState(
    content,
    shouldStream,
    initialRevealFromEmpty,
  );
  const displayedContent = smoothStreaming.content;
  const shouldRenderStreaming = smoothStreaming.isStreaming;
  const components = useMemo(
    () => ({
      stable: createMarkdownComponents(
        resolveFilePath,
        onOpenWorkspaceFile,
        currentAgentId,
        { agentMentionDirectory, onOpenAgentContact },
      ),
      streaming: createMarkdownComponents(
        resolveFilePath,
        onOpenWorkspaceFile,
        currentAgentId,
        {
          agentMentionDirectory,
          onOpenAgentContact,
          streamCodeBlocks: true,
          streamMermaid: true,
        },
      ),
    }),
    [agentMentionDirectory, currentAgentId, onOpenAgentContact, onOpenWorkspaceFile, resolveFilePath],
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
        className,
      )}
      data-markdown-streaming={shouldRenderStreaming || undefined}
    >
      {contentSegments.map((segment, index) => (
        <MessageMarkdownSegment
          components={components.stable}
          key={`${segment.type}:${index}`}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          agentMentions={agentMentions}
          renderLeadingSlashCommand={renderLeadingSlashCommand && index === 0}
          resolveFilePath={resolveFilePath}
          segment={segment}
          shouldStream={shouldRenderStreaming}
          streamingComponents={components.streaming}
          workspaceAgentId={workspaceAgentId}
        />
      ))}
    </div>
  );
}

interface MessageMarkdownSegmentProps {
	agentMentions: AgentMention[];
	components: Components;
  onOpenWorkspaceFile?: (path: string) => void;
  resolveFilePath: ResolveWorkspaceFilePath;
  segment: MarkdownContentSegment;
  shouldStream: boolean;
  streamingComponents: Components;
  workspaceAgentId?: string | null;
  renderLeadingSlashCommand: boolean;
}

function MessageMarkdownSegment({
	agentMentions,
	components,
  onOpenWorkspaceFile,
  resolveFilePath,
  segment,
  shouldStream,
  streamingComponents,
  workspaceAgentId,
  renderLeadingSlashCommand,
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

  const contentWithMentions = decorateMarkdownMentions(
    segment.text,
    agentMentions,
  );
  const sharedProps = {
    components,
    content: normalizeMarkdownContent(
      renderLeadingSlashCommand
        ? decorateLeadingSlashCommand(contentWithMentions)
        : contentWithMentions,
      resolveFilePath,
      onOpenWorkspaceFile,
      { is_streaming: shouldStream },
    ),
    rehypePlugins: REHYPE_PLUGINS,
    remarkPlugins: MARKDOWN_PLUGINS,
    urlTransform: transformMarkdownUrl,
  };
  return (
    <MarkdownText
      {...sharedProps}
      isStreaming={shouldStream}
      streamingComponents={streamingComponents}
    />
  );
}

function decorateMarkdownMentions(content: string, mentions: AgentMention[]): string {
  const matches = mentions
    .filter((mention) => mention.content_block_index === 0)
    .filter((mention) => mention.end_rune > mention.start_rune)
    .sort((left, right) => left.start_rune - right.start_rune);
  if (matches.length === 0) {
    return content;
  }
  const runes = Array.from(content);
  let cursor = 0;
  let result = "";
  for (const mention of matches) {
    const start = Math.max(cursor, Math.min(mention.start_rune, runes.length));
    const end = Math.max(start, Math.min(mention.end_rune, runes.length));
    if (end <= start) {
      continue;
    }
    result += runes.slice(cursor, start).join("");
    const label = runes.slice(start, end).join("").replaceAll("\\", "\\\\").replaceAll("]", "\\]");
    const handoffQuery = mention.handoff_id?.trim()
      ? `?handoff_id=${encodeURIComponent(mention.handoff_id.trim())}`
      : "";
    result += `[${label}](agent-mention://${encodeURIComponent(mention.agent_id)}${handoffQuery})`;
    cursor = end;
  }
  return result + runes.slice(cursor).join("");
}
