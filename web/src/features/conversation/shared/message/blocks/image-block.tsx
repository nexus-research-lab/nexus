"use client";

import { ImageIcon } from "lucide-react";

import { get_workspace_file_preview_url } from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { type ImageContent } from "@/types/conversation/message";

import {
  resolve_workspace_artifact_path,
  useMarkdownCurrentAgentID,
  useMarkdownFileResolver,
} from "../markdown/markdown-renderer-shared";

interface ImageBlockProps {
  block: ImageContent;
  on_open_workspace_file?: (path: string) => void;
  workspace_agent_id?: string | null;
}

interface ImageSource {
  src: string;
  workspace_path: string | null;
}

function first_non_empty(...values: Array<string | null | undefined>): string {
  for (const value of values) {
    const trimmed = value?.trim();
    if (trimmed) {
      return trimmed;
    }
  }
  return "";
}

function data_url(data: string, mime_type?: string | null): string {
  const trimmed = data.trim();
  if (!trimmed) {
    return "";
  }
  if (/^data:/i.test(trimmed)) {
    return trimmed;
  }
  return `data:${mime_type?.trim() || "image/png"};base64,${trimmed}`;
}

function resolve_image_source(
  block: ImageContent,
  resolve_file_path: (value: string) => string | null,
  current_agent_id?: string | null,
): ImageSource {
  const source = block.source;
  const source_data = first_non_empty(source?.data, block.data);
  if (source_data) {
    return { src: data_url(source_data, first_non_empty(block.mime_type, source?.mime_type, source?.media_type)), workspace_path: null };
  }

  const raw_path = first_non_empty(block.path, block.url, block.uri, source?.path, source?.url, source?.uri);
  if (!raw_path) {
    return { src: "", workspace_path: null };
  }
  if (/^(https?:|data:|blob:)/i.test(raw_path)) {
    return { src: raw_path, workspace_path: null };
  }

  const workspace_path = resolve_workspace_artifact_path(raw_path, resolve_file_path);
  if (workspace_path && current_agent_id) {
    return {
      src: get_workspace_file_preview_url(current_agent_id, workspace_path),
      workspace_path,
    };
  }
  return { src: raw_path, workspace_path: null };
}

export function ImageBlock({ block, on_open_workspace_file, workspace_agent_id }: ImageBlockProps) {
  const resolve_file_path = useMarkdownFileResolver(workspace_agent_id);
  const current_agent_id = useMarkdownCurrentAgentID(workspace_agent_id);
  const { src, workspace_path } = resolve_image_source(block, resolve_file_path, current_agent_id);
  const can_open = Boolean(workspace_path && on_open_workspace_file);

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
          "block max-w-full rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background) p-1 text-left shadow-[0_1px_0_rgba(0,0,0,0.03)]",
          can_open ? "cursor-pointer transition-colors hover:border-primary/30 hover:bg-primary/5" : "cursor-default",
        )}
        disabled={!can_open}
        onClick={() => workspace_path && on_open_workspace_file?.(workspace_path)}
        title={workspace_path || block.alt || "generated image"}
        type="button"
      >
        <img
          alt={block.alt || "generated image"}
          className="max-h-[520px] max-w-full rounded-[6px] object-contain"
          loading="lazy"
          src={src}
        />
      </button>
      {block.alt ? (
        <figcaption className="mt-1.5 text-[12px] leading-4 text-(--text-muted)">
          {block.alt}
        </figcaption>
      ) : null}
    </figure>
  );
}
