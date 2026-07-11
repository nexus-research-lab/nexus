import { getWorkspaceFilePreviewUrl } from "@/lib/api/agent/agent-api";
import { resolveWorkspaceArtifactPath } from "@/shared/ui/markdown/workspace/markdown-workspace-artifacts";
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
  const source = block.source;
  const resolvedAgentId = currentAgentId?.trim() ?? "";
  const imageSource = resolveImageSource({
    currentAgentId: resolvedAgentId,
    rawPath: firstNonEmptyArtifactValue(
      block.path,
      block.url,
      block.uri,
      source?.path,
      source?.url,
      source?.uri,
    ),
    resolveFilePath,
    sourceData: firstNonEmptyArtifactValue(source?.data, block.data),
    sourceMimeType: firstNonEmptyArtifactValue(
      block.mime_type,
      source?.mime_type,
      source?.media_type,
    ),
  });
  const canOpen = [
    Boolean(imageSource.workspacePath),
    hasOpenHandler,
  ].every(Boolean);
  return {
    action: buildWorkspaceArtifactExternalAction({
      agentId: resolvedAgentId,
      fileName: getArtifactFileName(
        imageSource.workspacePath ?? "",
        "image",
      ),
      path: imageSource.workspacePath,
    }),
    alt: block.alt || "generated image",
    canOpen,
    openClassName: canOpen
      ? "cursor-pointer transition-colors hover:border-primary/30 hover:bg-primary/5"
      : "cursor-default",
    source: imageSource,
  };
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
  const workspacePath = resolveWorkspaceArtifactPath(
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
