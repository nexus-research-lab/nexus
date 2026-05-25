"use client";

import { useEffect, useRef, useState } from "react";
import { Eye, FileText, FileWarning, LoaderCircle } from "lucide-react";
import type { Options as DocxPreviewOptions } from "docx-preview";

import { get_workspace_file_preview_url } from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { ConversationResizeHandle } from "./conversation-resize-handle";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "./workspace-file-preview-chrome";

const MAX_DOCX_PREVIEW_BYTES = 15 * 1024 * 1024;

type DocumentPreviewStatus =
  | { state: "loading"; message: string }
  | { state: "loaded" }
  | { state: "error"; message: string };

interface DocumentFilePreviewProps {
  agent_id: string;
  embedded?: boolean;
  file_name: string;
  is_preview_focused?: boolean;
  on_resize_start: () => void;
  on_toggle_preview_focus?: () => void;
  path: string;
}

const DOCX_RENDER_OPTIONS: Partial<DocxPreviewOptions> = {
  breakPages: true,
  className: "nexus-docx-preview",
  debug: false,
  experimental: false,
  ignoreFonts: false,
  ignoreHeight: false,
  ignoreLastRenderedPageBreak: false,
  ignoreWidth: false,
  inWrapper: true,
  renderAltChunks: false,
  renderChanges: false,
  renderComments: false,
  renderEndnotes: true,
  renderFooters: true,
  renderFootnotes: true,
  renderHeaders: true,
  trimXmlDeclaration: true,
  useBase64URL: true,
};

export function DocumentFilePreview({
  agent_id,
  embedded,
  file_name,
  is_preview_focused,
  on_resize_start,
  on_toggle_preview_focus,
  path,
}: DocumentFilePreviewProps) {
  const container_ref = useRef<HTMLDivElement>(null);
  const style_container_ref = useRef<HTMLDivElement>(null);
  const [status, set_status] = useState<DocumentPreviewStatus>({
    state: "loading",
    message: "加载文档预览中",
  });

  useEffect(() => {
    const container = container_ref.current;
    const style_container = style_container_ref.current;
    const abort_controller = new AbortController();
    let cancelled = false;

    if (container) {
      container.innerHTML = "";
    }
    if (style_container) {
      style_container.innerHTML = "";
    }

    async function load_preview() {
      if (!container || !style_container) {
        return;
      }

      set_status({ state: "loading", message: "读取 docx 文件中" });

      try {
        const preview_url = get_workspace_file_preview_url(agent_id, path);
        const response = await fetch(preview_url, {
          credentials: "include",
          signal: abort_controller.signal,
        });

        if (!response.ok) {
          throw new Error(`读取失败: ${response.status}`);
        }

        const content_length = response.headers.get("content-length");
        if (content_length && Number(content_length) > MAX_DOCX_PREVIEW_BYTES) {
          throw new Error("docx 文件超过 15MB，建议下载后查看");
        }

        const buffer = await response.arrayBuffer();
        if (cancelled) {
          return;
        }
        if (buffer.byteLength > MAX_DOCX_PREVIEW_BYTES) {
          throw new Error("docx 文件超过 15MB，建议下载后查看");
        }

        set_status({ state: "loading", message: "解析 docx 文件中" });
        const { renderAsync } = await import("docx-preview");
        if (cancelled) {
          return;
        }

        await renderAsync(buffer, container, style_container, DOCX_RENDER_OPTIONS);
        if (cancelled) {
          return;
        }

        set_status({ state: "loaded" });
      } catch (preview_error) {
        if (cancelled || abort_controller.signal.aborted) {
          return;
        }
        const message = preview_error instanceof Error ? preview_error.message : "docx 预览失败";
        if (container) {
          container.innerHTML = "";
        }
        if (style_container) {
          style_container.innerHTML = "";
        }
        set_status({ state: "error", message });
      }
    }

    void load_preview();

    return () => {
      cancelled = true;
      abort_controller.abort();
      if (container) {
        container.innerHTML = "";
      }
      if (style_container) {
        style_container.innerHTML = "";
      }
    };
  }, [agent_id, path]);

  const is_loading = status.state === "loading";
  const is_loaded = status.state === "loaded";
  const has_error = status.state === "error";

  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          aria_label="调整编辑器宽度"
          class_name="flex"
          on_mouse_down={on_resize_start}
        />
      ) : null}

      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agent_id={agent_id} file_name={file_name} path={path} />
            <WorkspaceFilePreviewFocusButton
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
            />
          </>
        )}
        embedded={embedded}
        meta={(
          <>
            <span className="flex items-center gap-1">
              <FileText className="h-3 w-3" />
              docx 预览
            </span>
            {has_error ? (
              <span className="flex items-center gap-1 text-destructive">
                <FileWarning className="h-3 w-3" />
                加载失败
              </span>
            ) : is_loaded ? (
              <span className="flex items-center gap-1 text-emerald-600">
                <Eye className="h-3 w-3" />
                已加载
              </span>
            ) : (
              <span className="flex items-center gap-1">
                <LoaderCircle className="h-3 w-3 animate-spin" />
                {is_loading ? status.message : "加载中"}
              </span>
            )}
          </>
        )}
        title={file_name}
      />

      <div className="soft-scrollbar relative min-h-0 flex-1 overflow-auto bg-[var(--surface-panel-subtle-background)] p-5">
        <style>
          {`
            .nexus-docx-preview-host .nexus-docx-preview-wrapper {
              align-items: center;
              background: transparent !important;
              box-sizing: border-box;
              display: flex;
              flex-direction: column;
              gap: 18px;
              min-width: max-content;
              padding: 0 !important;
            }

            .nexus-docx-preview-host section.nexus-docx-preview {
              background: #ffffff;
              box-shadow: 0 18px 36px rgba(15, 23, 42, 0.14);
              box-sizing: border-box;
              color: #111827;
              overflow: hidden;
            }

            .nexus-docx-preview-host section.nexus-docx-preview table {
              border-collapse: collapse;
            }

            .nexus-docx-preview-host section.nexus-docx-preview img,
            .nexus-docx-preview-host section.nexus-docx-preview svg {
              max-width: 100%;
            }
          `}
        </style>
        <div ref={style_container_ref} aria-hidden="true" className="contents" />
        {has_error ? (
          <div className="flex h-full min-h-[240px] items-center justify-center text-center">
            <div className="max-w-sm">
              <FileWarning className="mx-auto h-12 w-12 text-(--icon-muted)" />
              <p className="mt-4 text-sm font-medium text-(--text-strong)">docx 预览失败</p>
              <p className="mt-2 text-xs leading-5 text-(--text-soft)">{status.message}</p>
            </div>
          </div>
        ) : (
          <div
            ref={container_ref}
            className={cn(
              "nexus-docx-preview-host mx-auto min-h-full w-max min-w-full",
              is_loaded ? "opacity-100" : "opacity-0",
            )}
          />
        )}
        {is_loading ? (
          <div className="absolute inset-x-0 top-24 flex justify-center pointer-events-none">
            <div className="inline-flex items-center gap-2 rounded-full border border-(--divider-subtle-color) bg-(--surface-panel-background) px-3 py-1.5 text-xs text-(--text-muted) shadow-sm">
              <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
              <span>{status.message}</span>
            </div>
          </div>
        ) : null}
      </div>
    </>
  );
}
