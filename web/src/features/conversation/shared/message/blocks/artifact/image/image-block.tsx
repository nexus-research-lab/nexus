"use client";

import { ImageIcon } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import {
  useMarkdownCurrentAgentID,
  useMarkdownFileResolver,
} from "@/shared/ui/markdown/workspace/markdown-workspace-artifacts";
import type { ImageContent } from "@/types/conversation/message/content";

import { WorkspaceArtifactExternalActionButton } from "../workspace-artifact-external-action";
import {
  type ImageArtifactProjection,
  projectImageArtifact,
} from "./image-artifact-model";

interface ImageBlockProps {
  block: ImageContent;
  onOpenWorkspaceFile?: (
    path: string,
    workspaceAgentId?: string | null,
  ) => void;
  workspaceAgentId?: string | null;
}

export function ImageBlock({
  block,
  onOpenWorkspaceFile,
  workspaceAgentId,
}: ImageBlockProps) {
  const resolveFilePath = useMarkdownFileResolver(workspaceAgentId);
  const currentAgentId = useMarkdownCurrentAgentID(workspaceAgentId);
  const projection = projectImageArtifact({
    block,
    currentAgentId,
    hasOpenHandler: Boolean(onOpenWorkspaceFile),
    resolveFilePath,
  });
  if (!projection.source.src) {
    return <MissingImageArtifact />;
  }
  return (
    <figure className="my-3 min-w-0 max-w-full">
      <button
        className={cn(
          "block w-fit max-w-full rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background) p-1 text-left shadow-[0_1px_0_rgba(0,0,0,0.03)]",
          projection.openClassName,
        )}
        disabled={!projection.canOpen}
        onClick={() => openImageArtifact(projection, onOpenWorkspaceFile)}
        title={projection.source.workspacePath || projection.alt}
        type="button"
      >
        <img
          alt={projection.alt}
          className="max-h-[420px] w-auto max-w-full rounded-[6px] object-contain sm:max-w-[560px]"
          loading="lazy"
          src={projection.source.src}
        />
      </button>
      <ImageArtifactCaption caption={block.alt} />
      <WorkspaceArtifactExternalActionButton
        action={projection.action}
        className="mt-2 inline-flex items-center gap-1 rounded-[6px] border border-(--divider-subtle-color) px-2 py-1 text-[11px] font-medium text-(--text-muted) transition-colors hover:border-primary/25 hover:bg-primary/8 hover:text-primary"
        iconClassName="h-3.5 w-3.5"
      />
    </figure>
  );
}

function openImageArtifact(
  projection: ImageArtifactProjection,
  onOpenWorkspaceFile: ImageBlockProps["onOpenWorkspaceFile"],
): void {
  if (!projection.canOpen || !projection.source.workspacePath) {
    return;
  }
  onOpenWorkspaceFile?.(
    projection.source.workspacePath,
    projection.action?.agentId,
  );
}

function MissingImageArtifact() {
  return (
    <div className="my-2 flex max-w-md items-center gap-2 rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background) px-3 py-2 text-[13px] text-(--text-muted)">
      <ImageIcon className="h-4 w-4 shrink-0" />
      图片内容缺少可展示的数据
    </div>
  );
}

function ImageArtifactCaption({
  caption,
}: {
  caption: string | null | undefined;
}) {
  if (!caption) {
    return null;
  }
  return (
    <figcaption className="mt-1.5 text-[12px] leading-4 text-(--text-muted)">
      {caption}
    </figcaption>
  );
}
