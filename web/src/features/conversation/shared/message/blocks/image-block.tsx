"use client";

import { Download, FolderOpen, ImageIcon } from "lucide-react";

import {
  downloadWorkspaceFileApi,
  getWorkspaceFilePreviewUrl,
} from "@/lib/api/agent-manage-api";
import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import { cn } from "@/lib/utils";
import { type ImageContent } from "@/types/conversation/message";

import {
  resolveWorkspaceArtifactPath,
  useMarkdownCurrentAgentID,
  useMarkdownFileResolver,
} from "../markdown/markdown-workspace-artifacts";

interface ImageBlockProps {
  block: ImageContent;
  onOpenWorkspaceFile?: (path: string) => void;
  workspaceAgentId?: string | null;
}

interface ImageSource {
  src: string;
  workspace_path: string | null;
}

function firstNonEmpty(...values: Array<string | null | undefined>): string {
  for (const value of values) {
    const trimmed = value?.trim();
    if (trimmed) {
      return trimmed;
    }
  }
  return "";
}

function dataUrl(data: string, mimeType?: string | null): string {
  const trimmed = data.trim();
  if (!trimmed) {
    return "";
  }
  if (/^data:/i.test(trimmed)) {
    return trimmed;
  }
  return `data:${mimeType?.trim() || "image/png"};base64,${trimmed}`;
}

function resolveImageSource(
  block: ImageContent,
  resolveFilePath: (value: string) => string | null,
  currentAgentId?: string | null,
): ImageSource {
  const source = block.source;
  const sourceData = firstNonEmpty(source?.data, block.data);
  if (sourceData) {
    return { src: dataUrl(sourceData, firstNonEmpty(block.mime_type, source?.mime_type, source?.media_type)), workspace_path: null };
  }

  const rawPath = firstNonEmpty(block.path, block.url, block.uri, source?.path, source?.url, source?.uri);
  if (!rawPath) {
    return { src: "", workspace_path: null };
  }
  if (/^(https?:|data:|blob:)/i.test(rawPath)) {
    return { src: rawPath, workspace_path: null };
  }

  const workspacePath = resolveWorkspaceArtifactPath(rawPath, resolveFilePath);
  if (workspacePath && currentAgentId) {
    return {
      src: getWorkspaceFilePreviewUrl(currentAgentId, workspacePath),
      workspace_path: workspacePath,
    };
  }
  return { src: rawPath, workspace_path: null };
}

export function ImageBlock({ block, onOpenWorkspaceFile: onOpenWorkspaceFile, workspaceAgentId: workspaceAgentId }: ImageBlockProps) {
  const resolveFilePath = useMarkdownFileResolver(workspaceAgentId);
  const currentAgentId = useMarkdownCurrentAgentID(workspaceAgentId);
  const { src, workspace_path: workspacePath } = resolveImageSource(block, resolveFilePath, currentAgentId);
  const canOpen = Boolean(workspacePath && onOpenWorkspaceFile);
  const canDownload = Boolean(workspacePath && currentAgentId);
  const fileActionCopy = getWorkspaceFileExternalActionCopy(workspacePath?.split("/").at(-1) || "image");
  const handleExternalAction = () => {
    if (!workspacePath || !currentAgentId) {
      return;
    }
    void downloadWorkspaceFileApi(
      currentAgentId,
      workspacePath,
      workspacePath.split("/").at(-1) || "image",
    ).catch((error) => {
      console.error(`[ImageBlock] ${fileActionCopy.label} workspace 图片失败:`, error);
    });
  };

  if (!src) {
    return (
      <div className="my-2 flex max-w-md items-center gap-2 rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background) px-3 py-2 text-[13px] text-(--text-muted)">
        <ImageIcon className="h-4 w-4 shrink-0" />
        图片内容缺少可展示的数据
      </div>
    );
  }

  return (
    <figure className="my-3 min-w-0 max-w-full">
      <button
        className={cn(
          "block w-fit max-w-full rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background) p-1 text-left shadow-[0_1px_0_rgba(0,0,0,0.03)]",
          canOpen ? "cursor-pointer transition-colors hover:border-primary/30 hover:bg-primary/5" : "cursor-default",
        )}
        disabled={!canOpen}
        onClick={() => workspacePath && onOpenWorkspaceFile?.(workspacePath)}
        title={workspacePath || block.alt || "generated image"}
        type="button"
      >
        <img
          alt={block.alt || "generated image"}
          className="max-h-[420px] w-auto max-w-full rounded-[6px] object-contain sm:max-w-[560px]"
          loading="lazy"
          src={src}
        />
      </button>
      {block.alt ? (
        <figcaption className="mt-1.5 text-[12px] leading-4 text-(--text-muted)">
          {block.alt}
        </figcaption>
      ) : null}
      {canDownload ? (
        <button
          aria-label={fileActionCopy.ariaLabel}
          className="mt-2 inline-flex items-center gap-1 rounded-[6px] border border-(--divider-subtle-color) px-2 py-1 text-[11px] font-medium text-(--text-muted) transition-colors hover:border-primary/25 hover:bg-primary/8 hover:text-primary"
          onClick={handleExternalAction}
          title={fileActionCopy.title}
          type="button"
        >
          {fileActionCopy.mode === "reveal" ? (
            <FolderOpen className="h-3.5 w-3.5" />
          ) : (
            <Download className="h-3.5 w-3.5" />
          )}
          {fileActionCopy.label}
        </button>
      ) : null}
    </figure>
  );
}
