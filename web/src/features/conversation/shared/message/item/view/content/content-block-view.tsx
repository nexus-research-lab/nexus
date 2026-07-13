import type { ReactNode } from "react";

import type { ContentBlock } from "@/types/conversation/message/content";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import { ImageBlock } from "../../../blocks/artifact/image/image-block";
import { WorkspaceFileArtifactBlock } from "../../../blocks/artifact/workspace-file-artifacts";
import { ThinkingBlock } from "../../../blocks/thinking-block";
import { ToolUseErrorBlock } from "../../../blocks/tool/tool-use-error-block";
import { MarkdownRenderer } from "../../../markdown-renderer";
import {
  isHiddenSystemEvent,
  type StructuredContentProjection,
} from "./content-renderer-model";
import { ContentSystemEvent } from "./content-system-event";
import { ContentToolBlock } from "./content-tool-block";
import { TimelineBlock } from "./content-renderer-timeline";

export interface ContentBlockRenderContext {
  canRespondToPermissions: boolean;
  hiddenToolNames: ReadonlySet<string>;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  pendingPermissionsByToolUseId?: ReadonlyMap<string, PendingPermission>;
  permissionReadOnlyReason?: string;
  projection: StructuredContentProjection;
  workspaceAgentId?: string | null;
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
) => ReactNode;
type ContentBlockRendererMap = {
  [Type in ContentBlockType]: ContentBlockRenderer<Type>;
};
type ErasedContentBlockRenderer = (
  block: ContentBlock,
  context: ContentBlockRenderContext,
  streaming: boolean,
) => ReactNode;

const CONTENT_BLOCK_RENDERERS = {
  image: renderImageBlock,
  system_event: renderSystemEventBlock,
  task_progress: renderHiddenBlock,
  text: renderTextBlock,
  thinking: renderThinkingBlock,
  tool_result: renderHiddenBlock,
  tool_use: renderToolUseBlock,
  tool_use_error: renderToolUseErrorBlock,
  workspace_file_artifact: renderWorkspaceFileArtifactBlock,
} satisfies ContentBlockRendererMap;

export function ContentBlockView({
  block,
  context,
  showTimelineDots,
  streaming,
}: {
  block: ContentBlock;
  context: ContentBlockRenderContext;
  showTimelineDots: boolean;
  streaming: boolean;
}) {
  // 判别字段同时决定注册表索引和参数类型，类型擦除只发生在这个穷尽边界。
  const renderer = CONTENT_BLOCK_RENDERERS[
    block.type
  ] as ErasedContentBlockRenderer;
  const node = renderer(block, context, streaming);
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
) {
  if (!block.text.trim()) {
    return null;
  }
  return (
    <MarkdownRenderer
      content={block.text}
      isStreaming={streaming}
      onOpenWorkspaceFile={context.onOpenWorkspaceFile}
      workspaceAgentId={context.workspaceAgentId}
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
        onOpenWorkspaceFile: context.onOpenWorkspaceFile,
        onPermissionResponse: context.onPermissionResponse,
        pendingPermission: context.pendingPermissionsByToolUseId?.get(block.id),
        permissionReadOnlyReason: context.permissionReadOnlyReason,
        projection: context.projection,
        workspaceAgentId: context.workspaceAgentId,
      }}
    />
  );
}

function renderHiddenBlock() {
  return null;
}
