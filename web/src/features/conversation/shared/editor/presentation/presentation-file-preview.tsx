"use client";

import { useEffect, useRef } from "react";
import { ChevronLeft, ChevronRight, Eye, FileText, FileWarning, LoaderCircle } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { cn } from "@/shared/ui/class-name";
import { ConversationResizeHandle } from "../conversation-resize-handle";
import { fetchOfficePreviewBuffer } from "../office-preview-resource";
import { parsePptx } from "./presentation-pptx-parser";
import {
  type PresentationPreviewStatus,
  type PresentationSlide,
} from "./presentation-preview-model";
import { PresentationSlideCanvas } from "./presentation-slide-canvas";
import { revokeObjectUrls } from "./presentation-xml-utils";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "../workspace-file-preview-chrome";
import type { WorkspaceFilePreviewProps } from "../workspace-file-preview-types";

export function PresentationFilePreview({
  agentId: agentId,
  embedded,
  fileName: fileName,
  isPreviewFocused: isPreviewFocused,
  onResizeStart: onResizeStart,
  onTogglePreviewFocus: onTogglePreviewFocus,
  path,
}: WorkspaceFilePreviewProps) {
  const cleanupUrlsRef = useRef<() => void>(() => undefined);
  const previewKey = `${agentId}\x1f${path}`;
  const [slides, setSlides] = useResettableState<PresentationSlide[]>([], previewKey);
  const [activeSlideIndex, setActiveSlideIndex] = useResettableState(0, previewKey);
  const [status, setStatus] = useResettableState<PresentationPreviewStatus>({
    state: "loading",
    message: "加载演示文稿预览中",
  }, previewKey);

  useEffect(() => {
    const abortController = new AbortController();
    let cancelled = false;

    cleanupUrlsRef.current();
    cleanupUrlsRef.current = () => undefined;

    async function loadPreview() {
      try {
        const buffer = await fetchOfficePreviewBuffer({
          agentId,
          fileLabel: "pptx",
          path,
          signal: abortController.signal,
        });
        if (cancelled) {
          return;
        }

        setStatus({ state: "loading", message: "解析 pptx 文件中" });
        const result = await parsePptx(buffer);
        if (cancelled) {
          revokeObjectUrls(result.objectUrls);
          return;
        }

        cleanupUrlsRef.current = () => revokeObjectUrls(result.objectUrls);
        setSlides(result.slides);
        setActiveSlideIndex(0);
        setStatus({ state: "loaded", slideCount: result.slides.length });
      } catch (previewError) {
        if (cancelled || abortController.signal.aborted) {
          return;
        }
        const message = previewError instanceof Error ? previewError.message : "pptx 预览失败";
        cleanupUrlsRef.current();
        cleanupUrlsRef.current = () => undefined;
        setSlides([]);
        setStatus({ state: "error", message });
      }
    }

    void loadPreview();

    return () => {
      cancelled = true;
      abortController.abort();
      cleanupUrlsRef.current();
      cleanupUrlsRef.current = () => undefined;
    };
  }, [agentId, path, setActiveSlideIndex, setSlides, setStatus]);

  const isLoaded = status.state === "loaded";
  const isLoading = status.state === "loading";
  const hasError = status.state === "error";
  const activeSlide = slides[Math.min(activeSlideIndex, Math.max(slides.length - 1, 0))];

  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          ariaLabel="调整编辑器宽度"
          className="flex"
          onMouseDown={onResizeStart}
        />
      ) : null}

      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agentId={agentId} fileName={fileName} path={path} />
            <WorkspaceFilePreviewFocusButton
              isPreviewFocused={isPreviewFocused}
              onTogglePreviewFocus={onTogglePreviewFocus}
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
            {hasError ? (
              <span className="flex items-center gap-1 text-destructive">
                <FileWarning className="h-3 w-3" />
                加载失败
              </span>
            ) : isLoaded ? (
              <span className="flex items-center gap-1 text-(--success)">
                <Eye className="h-3 w-3" />
                已加载 {status.slideCount} 页
              </span>
            ) : (
              <span className="flex items-center gap-1">
                <LoaderCircle className="h-3 w-3 animate-spin" />
                {isLoading ? status.message : "加载中"}
              </span>
            )}
          </>
        )}
        title={fileName}
      />

      <div className="min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)]">
        {hasError ? (
          <div className="flex h-full items-center justify-center p-8 text-center">
            <div className="max-w-sm">
              <FileWarning className="mx-auto h-12 w-12 text-(--icon-muted)" />
              <p className="mt-4 text-sm font-medium text-(--text-strong)">pptx 预览失败</p>
              <p className="mt-2 text-xs leading-5 text-(--text-soft)">{status.message}</p>
            </div>
          </div>
        ) : activeSlide ? (
          <div className="flex h-full min-h-0">
            {slides.length > 1 ? (
              <aside className="soft-scrollbar hidden w-36 shrink-0 overflow-auto border-r divider-subtle bg-(--surface-panel-background) p-3 md:block">
                <div className="space-y-2">
                  {slides.map((slide, index) => (
                    <button
                      className={cn(
                        "w-full rounded-[6px] border p-1 text-left transition-colors",
                        index === activeSlideIndex
                          ? "border-primary/45 bg-primary/8"
                          : "border-(--divider-subtle-color) bg-(--surface-panel-subtle-background) hover:border-primary/30",
                      )}
                      key={slide.id}
                      onClick={() => setActiveSlideIndex(index)}
                      type="button"
                    >
                      <PresentationSlideCanvas className="rounded-[2px] shadow-none" slide={slide} thumbnail />
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
                    {activeSlideIndex + 1} / {slides.length} · {activeSlide.title}
                  </span>
                  {slides.length > 1 ? (
                    <div className="flex shrink-0 items-center gap-1">
                      <button
                        aria-label="上一页幻灯片"
                        className="inline-flex h-8 w-8 items-center justify-center rounded-[6px] border border-(--divider-subtle-color) bg-(--surface-panel-background) text-(--text-default) transition-colors hover:border-primary/30 hover:text-primary disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
                        disabled={activeSlideIndex <= 0}
                        onClick={() => setActiveSlideIndex((index) => Math.max(index - 1, 0))}
                        type="button"
                      >
                        <ChevronLeft className="h-4 w-4" />
                      </button>
                      <button
                        aria-label="下一页幻灯片"
                        className="inline-flex h-8 w-8 items-center justify-center rounded-[6px] border border-(--divider-subtle-color) bg-(--surface-panel-background) text-(--text-default) transition-colors hover:border-primary/30 hover:text-primary disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
                        disabled={activeSlideIndex >= slides.length - 1}
                        onClick={() => setActiveSlideIndex((index) => Math.min(index + 1, slides.length - 1))}
                        type="button"
                      >
                        <ChevronRight className="h-4 w-4" />
                      </button>
                    </div>
                  ) : null}
                </div>
                <PresentationSlideCanvas slide={activeSlide} />
              </div>
            </div>
          </div>
        ) : (
          <div className="flex h-full items-center justify-center p-8 text-center">
            <div className="max-w-xs">
              <LoaderCircle className="mx-auto h-8 w-8 animate-spin text-primary" />
              <p className="mt-3 text-sm font-medium text-(--text-strong)">
                {isLoading ? status.message : "正在加载 pptx 预览"}
              </p>
            </div>
          </div>
        )}
      </div>
    </>
  );
}
