import { getWorkspaceFilePreviewUrl } from "@/lib/api/agent/agent-api";
import { resolveWorkspaceImagePath } from "@/shared/ui/markdown/workspace/markdown-workspace-artifact-model";
import type { ImageContent } from "@/types/conversation/message/content";

import {
  firstNonEmptyArtifactValue,
  getArtifactFileName,
} from "../artifact-path-model";
import {
  buildWorkspaceArtifactExternalAction,
  type WorkspaceArtifactExternalAction,
} from "../workspace-artifact-action-model";

interface ImageSource {
  src: string;
  workspacePath: string | null;
}

interface ImageSourceContext {
  currentAgentId: string;
  rawPath: string;
  resolveFilePath: (value: string) => string | null;
  sourceData: string;
  sourceMimeType: string;
}

type ImageSourceResolver = (
  context: ImageSourceContext,
) => ImageSource | null;

export interface ImageArtifactProjection {
  action: WorkspaceArtifactExternalAction | null;
  alt: string;
  canOpen: boolean;
  openClassName: string;
  source: ImageSource;
}

const EXTERNAL_IMAGE_PATTERN = /^(https?:|data:|blob:)/i;
const EMPTY_IMAGE_SOURCE: ImageSource = { src: "", workspacePath: null };
const IMAGE_SOURCE_RESOLVERS: ImageSourceResolver[] = [
  resolveInlineImageSource,
  resolveExternalImageSource,
  resolveWorkspaceImageSource,
  resolveRawImageSource,
];

export function projectImageArtifact({
  block,
  currentAgentId,
  hasOpenHandler,
  resolveFilePath,
}: {
  block: ImageContent;
  currentAgentId: string | null | undefined;
  hasOpenHandler: boolean;
  resolveFilePath: (value: string) => string | null;
}): ImageArtifactProjection {
  const resolvedAgentId = resolveAgentId(currentAgentId);
  const imageSource = resolveImageSource(
    buildImageSourceContext(block, resolvedAgentId, resolveFilePath),
  );
  const canOpen = canOpenImageSource(imageSource, hasOpenHandler);
  return {
    action: buildImageExternalAction(imageSource, resolvedAgentId),
    alt: resolveImageAlt(block),
    canOpen,
    openClassName: resolveImageOpenClassName(canOpen),
    source: imageSource,
  };
}

function buildImageSourceContext(
  block: ImageContent,
  currentAgentId: string,
  resolveFilePath: (value: string) => string | null,
): ImageSourceContext {
  return {
    currentAgentId,
    rawPath: resolveRawImagePath(block),
    resolveFilePath,
    sourceData: resolveInlineImageData(block),
    sourceMimeType: resolveImageMimeType(block),
  };
}

function resolveAgentId(currentAgentId: string | null | undefined): string {
  return currentAgentId?.trim() ?? "";
}

function resolveRawImagePath(block: ImageContent): string {
  return firstNonEmptyArtifactValue(
    block.path,
    block.url,
    block.uri,
    block.source?.path,
    block.source?.url,
    block.source?.uri,
  );
}

function resolveInlineImageData(block: ImageContent): string {
  return firstNonEmptyArtifactValue(block.source?.data, block.data);
}

function resolveImageMimeType(block: ImageContent): string {
  return firstNonEmptyArtifactValue(
    block.mime_type,
    block.source?.mime_type,
    block.source?.media_type,
  );
}

function canOpenImageSource(
  source: ImageSource,
  hasOpenHandler: boolean,
): boolean {
  return [Boolean(source.workspacePath), hasOpenHandler].every(Boolean);
}

function buildImageExternalAction(
  source: ImageSource,
  agentId: string,
): WorkspaceArtifactExternalAction | null {
  return buildWorkspaceArtifactExternalAction({
    agentId,
    fileName: getArtifactFileName(source.workspacePath ?? "", "image"),
    path: source.workspacePath,
  });
}

function resolveImageAlt(block: ImageContent): string {
  return block.alt || "generated image";
}

function resolveImageOpenClassName(canOpen: boolean): string {
  return canOpen
    ? "cursor-pointer transition-colors hover:border-primary/30 hover:bg-primary/5"
    : "cursor-default";
}

function resolveImageSource(context: ImageSourceContext): ImageSource {
  return IMAGE_SOURCE_RESOLVERS
    .map((resolver) => resolver(context))
    .find((source): source is ImageSource => Boolean(source))
    ?? EMPTY_IMAGE_SOURCE;
}

function resolveInlineImageSource(
  context: ImageSourceContext,
): ImageSource | null {
  if (!context.sourceData) {
    return null;
  }
  return {
    src: buildImageDataUrl(context.sourceData, context.sourceMimeType),
    workspacePath: null,
  };
}

function resolveExternalImageSource(
  context: ImageSourceContext,
): ImageSource | null {
  if (!EXTERNAL_IMAGE_PATTERN.test(context.rawPath)) {
    return null;
  }
  return { src: context.rawPath, workspacePath: null };
}

function resolveWorkspaceImageSource(
  context: ImageSourceContext,
): ImageSource | null {
  if (!context.rawPath || !context.currentAgentId) {
    return null;
  }
  const workspacePath = resolveWorkspaceImagePath(
    context.rawPath,
    context.resolveFilePath,
  );
  if (!workspacePath) {
    return null;
  }
  return {
    src: getWorkspaceFilePreviewUrl(context.currentAgentId, workspacePath),
    workspacePath,
  };
}

function resolveRawImageSource(
  context: ImageSourceContext,
): ImageSource | null {
  if (!context.rawPath) {
    return null;
  }
  return { src: context.rawPath, workspacePath: null };
}

function buildImageDataUrl(data: string, mimeType: string): string {
  const trimmedData = data.trim();
  if (!trimmedData) {
    return "";
  }
  if (/^data:/i.test(trimmedData)) {
    return trimmedData;
  }
  return `data:${mimeType || "image/png"};base64,${trimmedData}`;
}
