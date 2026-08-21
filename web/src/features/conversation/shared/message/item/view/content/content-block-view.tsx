/**
 * INPUT: 单个 ContentBlock、流式身份与消息级渲染上下文。
 * OUTPUT: 稳定内容块视图；live 空文本预挂载，同帧首批非空正文从空显示态平滑追赶。
 * POS: 结构化内容注册表的穷尽分派边界，不负责消息投影或时间线排序。
 */
import type { ReactNode } from "react";

import type { ContentBlock } from "@/types/conversation/message/content";
import type { AgentMention } from "@/types/conversation/message/entity";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";
import { hasLiveStreamRevealMarker } from "@/lib/conversation/live-stream-reveal";
import { isHiddenSystemEvent } from "../../../message-content-model";

import { ImageBlock } from "../../../blocks/artifact/image/image-block";
import { WorkspaceFileArtifactBlock } from "../../../blocks/artifact/workspace-file-artifacts";
import { ThinkingBlock } from "../../../blocks/thinking-block";
import { ToolUseErrorBlock } from "../../../blocks/tool/tool-use-error-block";
import { MarkdownRenderer } from "../../../markdown-renderer";
import {
  shouldMountTextContentBlock,
  type StructuredContentProjection,
} from "./content-renderer-model";
import { ContentSystemEvent } from "./content-system-event";
import { ContentToolBlock } from "./content-tool-block";
import { TimelineBlock } from "./content-renderer-timeline";
import type { AgentMentionDirectory } from "../../../agent-mention-chip";
import type { PendingInteractionOwner } from "../../message-item-projection";
import type { UnresolvedToolStatus } from "./content-renderer-contract";

export interface ContentBlockRenderContext {
  canRespondToPermissions: boolean;
  defaultToolDetailsExpanded: boolean;
  hiddenToolNames: ReadonlySet<string>;
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  pendingInteractionOwner: PendingInteractionOwner;
  pendingPermissionsByToolUseId?: ReadonlyMap<string, PendingPermission>;
  permissionReadOnlyReason?: string;
  projection: StructuredContentProjection;
  unresolvedToolStatus?: UnresolvedToolStatus;
  workspaceAgentId?: string | null;
  agentMentions?: AgentMention[];
  agentMentionDirectory?: AgentMentionDirectory;
  onOpenAgentContact?: (agentId: string) => void;
}

type ContentBlockType = ContentBlock["type"];
type ContentBlockOf<Type extends ContentBlockType> = Extract<
  ContentBlock,
  { type: Type }
>;
type ContentBlockRenderer<Type extends ContentBlockType> = (
  block: ContentBlockOf<Type>,
  context: ContentBlockRenderContext,
  streaming: boolean,
  blockIndex: number,
) => ReactNode;
type ContentBlockRendererMap = {
  [Type in ContentBlockType]: ContentBlockRenderer<Type>;
};
type ErasedContentBlockRenderer = (
  block: ContentBlock,
  context: ContentBlockRenderContext,
  streaming: boolean,
  blockIndex: number,
) => ReactNode;

const CONTENT_BLOCK_RENDERERS = {
  document: renderHiddenBlock,
  image: renderImageBlock,
  progress_update: renderHiddenBlock,
  redacted_thinking: renderHiddenBlock,
  resource_link: renderHiddenBlock,
  search_result: renderHiddenBlock,
  system_event: renderSystemEventBlock,
  task_progress: renderHiddenBlock,
  text: renderTextBlock,
  thinking: renderThinkingBlock,
  tool_result: renderHiddenBlock,
  tool_use: renderToolUseBlock,
  tool_use_error: renderToolUseErrorBlock,
  unsupported: renderHiddenBlock,
  workspace_file_artifact: renderWorkspaceFileArtifactBlock,
} satisfies ContentBlockRendererMap;

export function ContentBlockView({
  block,
  context,
  showTimelineDots,
  streaming,
  blockIndex,
}: {
  block: ContentBlock;
  context: ContentBlockRenderContext;
  showTimelineDots: boolean;
  streaming: boolean;
  blockIndex: number;
}) {
  // 判别字段同时决定注册表索引和参数类型，类型擦除只发生在这个穷尽边界。
  const renderer = CONTENT_BLOCK_RENDERERS[
    block.type
  ] as ErasedContentBlockRenderer | undefined;
  if (!renderer) {
    return null;
  }
  const node = renderer(block, context, streaming, blockIndex);
  if (node === null || node === undefined || node === false) {
    return null;
  }
  if (!showTimelineDots) {
    return <div>{node}</div>;
  }
  return <TimelineBlock active={streaming}>{node}</TimelineBlock>;
}

function renderTextBlock(
  block: ContentBlockOf<"text">,
  context: ContentBlockRenderContext,
  streaming: boolean,
  blockIndex: number,
) {
  if (!shouldMountTextContentBlock(block.text, streaming)) {
    return null;
  }
  return (
    <MarkdownRenderer
      content={block.text}
      initialRevealFromEmpty={hasLiveStreamRevealMarker(block)}
      isStreaming={streaming}
      onOpenWorkspaceFile={context.onOpenWorkspaceFile}
      workspaceAgentId={context.workspaceAgentId}
      agentMentions={context.agentMentions
        ?.filter((mention) => mention.content_block_index === blockIndex)
        .map((mention) => ({ ...mention, content_block_index: 0 }))}
      agentMentionDirectory={context.agentMentionDirectory}
      onOpenAgentContact={context.onOpenAgentContact}
    />
  );
}

function renderToolUseErrorBlock(block: ContentBlockOf<"tool_use_error">) {
  return <ToolUseErrorBlock content={block.content} />;
}

function renderThinkingBlock(
  block: ContentBlockOf<"thinking">,
  context: ContentBlockRenderContext,
  streaming: boolean,
) {
  if (!block.thinking.trim()) {
    return null;
  }
  return (
    <ThinkingBlock
      initialRevealFromEmpty={hasLiveStreamRevealMarker(block)}
      isStreaming={streaming}
      thinking={block.thinking}
      workspaceAgentId={context.workspaceAgentId}
    />
  );
}

function renderImageBlock(
  block: ContentBlockOf<"image">,
  context: ContentBlockRenderContext,
) {
  return (
    <ImageBlock
      block={block}
      onOpenWorkspaceFile={context.onOpenWorkspaceFile}
      workspaceAgentId={context.workspaceAgentId}
    />
  );
}

function renderSystemEventBlock(block: ContentBlockOf<"system_event">) {
  return isHiddenSystemEvent(block) ? null : <ContentSystemEvent block={block} />;
}

function renderWorkspaceFileArtifactBlock(
  block: ContentBlockOf<"workspace_file_artifact">,
  context: ContentBlockRenderContext,
) {
  return (
    <WorkspaceFileArtifactBlock
      artifact={block}
      onOpenWorkspaceFile={context.onOpenWorkspaceFile}
    />
  );
}

function renderToolUseBlock(
  block: ContentBlockOf<"tool_use">,
  context: ContentBlockRenderContext,
) {
  if (context.hiddenToolNames.has(block.name)) {
    return null;
  }
  return (
    <ContentToolBlock
      block={block}
      context={{
        canRespondToPermissions: context.canRespondToPermissions,
        defaultToolDetailsExpanded: context.defaultToolDetailsExpanded,
        onOpenSubagentTask: context.onOpenSubagentTask,
        onOpenWorkspaceFile: context.onOpenWorkspaceFile,
        onPermissionResponse: context.onPermissionResponse,
        pendingInteractionOwner: context.pendingInteractionOwner,
        pendingPermission: context.pendingPermissionsByToolUseId?.get(block.id),
        permissionReadOnlyReason: context.permissionReadOnlyReason,
        projection: context.projection,
        unresolvedToolStatus: context.unresolvedToolStatus,
        workspaceAgentId: context.workspaceAgentId,
      }}
    />
  );
}

function renderHiddenBlock() {
  return null;
}
