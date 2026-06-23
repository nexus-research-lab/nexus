"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, Eye, FileText, FileWarning, LoaderCircle } from "lucide-react";

import { get_workspace_file_preview_url } from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { ConversationResizeHandle } from "./conversation-resize-handle";
import { parse_pptx } from "./presentation-pptx-parser";
import {
  MAX_PPTX_PREVIEW_BYTES,
  type PresentationPreviewStatus,
  type PresentationSlide,
} from "./presentation-preview-model";
import { PresentationSlideCanvas } from "./presentation-slide-canvas";
import { revoke_object_urls } from "./presentation-xml-utils";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "./workspace-file-preview-chrome";

interface PresentationFilePreviewProps {
  agent_id: string;
  embedded?: boolean;
  file_name: string;
  is_preview_focused?: boolean;
  on_resize_start: () => void;
  on_toggle_preview_focus?: () => void;
  path: string;
}

export function PresentationFilePreview({
  agent_id,
  embedded,
  file_name,
  is_preview_focused,
  on_resize_start,
  on_toggle_preview_focus,
  path,
}: PresentationFilePreviewProps) {
  const cleanup_urls_ref = useRef<() => void>(() => undefined);
  const [slides, set_slides] = useState<PresentationSlide[]>([]);
  const [active_slide_index, set_active_slide_index] = useState(0);
  const [status, set_status] = useState<PresentationPreviewStatus>({
    state: "loading",
    message: "加载演示文稿预览中",
  });

  useEffect(() => {
    const abort_controller = new AbortController();
    let cancelled = false;

    cleanup_urls_ref.current();
    cleanup_urls_ref.current = () => undefined;
    set_slides([]);
    set_active_slide_index(0);

    async function load_preview() {
      set_status({ state: "loading", message: "读取 pptx 文件中" });

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
        if (content_length && Number(content_length) > MAX_PPTX_PREVIEW_BYTES) {
          throw new Error("pptx 文件超过 15MB，当前无法内置预览，请使用上方按钮处理");
        }

        const buffer = await response.arrayBuffer();
        if (cancelled) {
          return;
        }
        if (buffer.byteLength > MAX_PPTX_PREVIEW_BYTES) {
          throw new Error("pptx 文件超过 15MB，当前无法内置预览，请使用上方按钮处理");
        }

        set_status({ state: "loading", message: "解析 pptx 文件中" });
        const result = await parse_pptx(buffer);
        if (cancelled) {
          revoke_object_urls(result.object_urls);
          return;
        }

        cleanup_urls_ref.current = () => revoke_object_urls(result.object_urls);
        set_slides(result.slides);
        set_active_slide_index(0);
        set_status({ state: "loaded", slide_count: result.slides.length });
      } catch (preview_error) {
        if (cancelled || abort_controller.signal.aborted) {
          return;
        }
        const message = preview_error instanceof Error ? preview_error.message : "pptx 预览失败";
        cleanup_urls_ref.current();
        cleanup_urls_ref.current = () => undefined;
        set_slides([]);
        set_status({ state: "error", message });
      }
    }

    void load_preview();

    return () => {
      cancelled = true;
      abort_controller.abort();
      cleanup_urls_ref.current();
      cleanup_urls_ref.current = () => undefined;
    };
  }, [agent_id, path]);

  const is_loaded = status.state === "loaded";
  const is_loading = status.state === "loading";
  const has_error = status.state === "error";
  const active_slide = slides[Math.min(active_slide_index, Math.max(slides.length - 1, 0))];

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
              pptx 预览
            </span>
            {has_error ? (
              <span className="flex items-center gap-1 text-destructive">
                <FileWarning className="h-3 w-3" />
                加载失败
              </span>
            ) : is_loaded ? (
              <span className="flex items-center gap-1 text-(--success)">
                <Eye className="h-3 w-3" />
                已加载 {status.slide_count} 页
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

      <div className="min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)]">
        {has_error ? (
          <div className="flex h-full items-center justify-center p-8 text-center">
            <div className="max-w-sm">
              <FileWarning className="mx-auto h-12 w-12 text-(--icon-muted)" />
              <p className="mt-4 text-sm font-medium text-(--text-strong)">pptx 预览失败</p>
              <p className="mt-2 text-xs leading-5 text-(--text-soft)">{status.message}</p>
            </div>
          </div>
        ) : active_slide ? (
          <div className="flex h-full min-h-0">
            {slides.length > 1 ? (
              <aside className="soft-scrollbar hidden w-36 shrink-0 overflow-auto border-r divider-subtle bg-(--surface-panel-background) p-3 md:block">
                <div className="space-y-2">
                  {slides.map((slide, index) => (
                    <button
                      className={cn(
                        "w-full rounded-[6px] border p-1 text-left transition-colors",
                        index === active_slide_index
                          ? "border-primary/45 bg-primary/8"
                          : "border-(--divider-subtle-color) bg-(--surface-panel-subtle-background) hover:border-primary/30",
                      )}
                      key={slide.id}
                      onClick={() => set_active_slide_index(index)}
                      type="button"
                    >
                      <PresentationSlideCanvas class_name="rounded-[2px] shadow-none" slide={slide} thumbnail />
                      <span className="mt-1 block truncate text-[10px] font-medium text-(--text-muted)">
                        {index + 1}. {slide.title}
                      </span>
                    </button>
                  ))}
                </div>
              </aside>
            ) : null}

            <div className="soft-scrollbar min-h-0 flex-1 overflow-auto p-5">
              <div className="mx-auto flex w-full max-w-6xl flex-col gap-3">
                <div className="flex items-center justify-between gap-3 text-xs text-(--text-muted)">
                  <span className="min-w-0 truncate">
                    {active_slide_index + 1} / {slides.length} · {active_slide.title}
                  </span>
                  {slides.length > 1 ? (
                    <div className="flex shrink-0 items-center gap-1">
                      <button
                        aria-label="上一页幻灯片"
                        className="inline-flex h-8 w-8 items-center justify-center rounded-[6px] border border-(--divider-subtle-color) bg-(--surface-panel-background) text-(--text-default) transition-colors hover:border-primary/30 hover:text-primary disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
                        disabled={active_slide_index <= 0}
                        onClick={() => set_active_slide_index((index) => Math.max(index - 1, 0))}
                        type="button"
                      >
                        <ChevronLeft className="h-4 w-4" />
                      </button>
                      <button
                        aria-label="下一页幻灯片"
                        className="inline-flex h-8 w-8 items-center justify-center rounded-[6px] border border-(--divider-subtle-color) bg-(--surface-panel-background) text-(--text-default) transition-colors hover:border-primary/30 hover:text-primary disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
                        disabled={active_slide_index >= slides.length - 1}
                        onClick={() => set_active_slide_index((index) => Math.min(index + 1, slides.length - 1))}
                        type="button"
                      >
                        <ChevronRight className="h-4 w-4" />
                      </button>
                    </div>
                  ) : null}
                </div>
                <PresentationSlideCanvas slide={active_slide} />
              </div>
            </div>
          </div>
        ) : (
          <div className="flex h-full items-center justify-center p-8 text-center">
            <div className="max-w-xs">
              <LoaderCircle className="mx-auto h-8 w-8 animate-spin text-primary" />
              <p className="mt-3 text-sm font-medium text-(--text-strong)">
                {is_loading ? status.message : "正在加载 pptx 预览"}
              </p>
            </div>
          </div>
        )}
      </div>
    </>
  );
}
