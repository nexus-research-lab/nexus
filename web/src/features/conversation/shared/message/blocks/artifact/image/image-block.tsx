"use client";

/**
 * INPUT: 图片块与可选 generation-bound detail 引用。
 * OUTPUT: 带桌面认证、可取消 Blob 生命周期的图片展示。
 * POS: 图片 Artifact 的 React 资源边界。
 */

import { ImageIcon, LoaderCircle } from "lucide-react";
import { useEffect, useState } from "react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  useMarkdownCurrentAgentID,
  useMarkdownFileResolver,
} from "@/shared/ui/markdown/workspace/use-markdown-workspace-files";
import type { ImageContent } from "@/types/conversation/message/content";
import { getSessionMessageImageDetailApi } from "@/lib/api/conversation/session-api";

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
  const { t } = useI18n();
  const resolveFilePath = useMarkdownFileResolver(workspaceAgentId);
  const currentAgentId = useMarkdownCurrentAgentID(workspaceAgentId);
  const deferredImage = useDeferredImageDetail(block);
  const projection = projectImageArtifact({
    block,
    currentAgentId,
    deferredDetailUrl: deferredImage.url,
    hasOpenHandler: Boolean(onOpenWorkspaceFile),
    resolveFilePath,
  });
  if (deferredImage.loading && !projection.source.src) {
    return <LoadingImageArtifact label={t("message.image_detail_loading")} />;
  }
  if (!projection.source.src) {
    return <MissingImageArtifact />;
  }
  return (
    <figure className="my-3 min-w-0 max-w-full">
      <button
        className={cn(
          "content-artifact-image content-media-frame text-left",
          projection.openClassName,
        )}
        disabled={!projection.canOpen}
        onClick={() => openImageArtifact(projection, onOpenWorkspaceFile)}
        title={projection.source.workspacePath || projection.alt}
        type="button"
      >
        <img
          alt={projection.alt}
          className="content-artifact-image-preview h-full w-full object-contain"
          loading="lazy"
          src={projection.source.src}
        />
      </button>
      <ImageArtifactCaption caption={block.alt} />
      <WorkspaceArtifactExternalActionButton
        action={projection.action}
        className="content-artifact-external-action mt-1.5 px-2 py-1 text-xs font-medium"
        iconClassName="h-3.5 w-3.5"
      />
    </figure>
  );
}

function useDeferredImageDetail(block: ImageContent): {
  loading: boolean;
  url: string;
} {
  const detailRef = block.detail_ref?.trim() ?? "";
  const sessionKey = block.detail_session_key?.trim() ?? "";
  const key = `${sessionKey}:${detailRef}`;
  const [state, setState] = useState({ key: "", loading: false, url: "" });

  useEffect(() => {
    if (!sessionKey || !detailRef) {
      return;
    }
    const controller = new AbortController();
    let objectUrl = "";
    setState({ key, loading: true, url: "" });
    void getSessionMessageImageDetailApi(
      sessionKey,
      detailRef,
      controller.signal,
    ).then((blob) => {
      objectUrl = URL.createObjectURL(blob);
      setState({ key, loading: false, url: objectUrl });
    }).catch((error: unknown) => {
      if (!controller.signal.aborted) {
        console.error("[conversation] Failed to load image detail", error);
        setState({ key, loading: false, url: "" });
      }
    });
    return () => {
      controller.abort();
      if (objectUrl) {
        URL.revokeObjectURL(objectUrl);
      }
    };
  }, [detailRef, key, sessionKey]);

  return state.key === key
    ? { loading: state.loading, url: state.url }
    : { loading: Boolean(sessionKey && detailRef), url: "" };
}

function LoadingImageArtifact({ label }: { label: string }) {
  return (
    <div className="content-artifact-empty content-media-frame my-2 flex items-center justify-center gap-2 px-3 py-2 text-sm">
      <LoaderCircle className="h-4 w-4 shrink-0 animate-spin" />
      {label}
    </div>
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
    <div className="content-artifact-empty content-media-frame my-2 flex items-center justify-center gap-2 px-3 py-2 text-sm">
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
    <figcaption className="mt-1.5 text-compact leading-4 text-(--text-muted)">
      {caption}
    </figcaption>
  );
}
