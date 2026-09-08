// INPUT: 工作区 PPTX 标识、预览聚焦状态与文件动作。
// OUTPUT: 可重试的幻灯片预览、共享标题栏/状态、缩略图选择与本地化翻页动作。
// POS: 演示文稿预览与资源释放；Office scope 隔离迟到结果，解析归 parser，动作与排版归 shared/ui。
"use client";

import { useEffect, useRef } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { WorkspaceFilePreviewLoading } from "../workspace-file-preview-loading";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { fetchOfficePreviewBuffer } from "../office-preview-resource";
import { useOfficePreviewScope } from "../use-office-preview-scope";
import { OfficePreviewFailureState } from "../office-preview-fallbacks";
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
  agentId,
  fileName,
  isPreviewFocused,
  onTogglePreviewFocus,
  path,
}: WorkspaceFilePreviewProps) {
  const { t } = useI18n();
  const cleanupUrlsRef = useRef<() => void>(() => undefined);
  const { scopeKey, requestKey, isCurrent, retryPreview } = useOfficePreviewScope(agentId, path);
  const [slides, setSlides] = useResettableState<PresentationSlide[]>([], scopeKey);
  const [activeSlideIndex, setActiveSlideIndex] = useResettableState(0, scopeKey);
  const [status, setStatus] = useResettableState<PresentationPreviewStatus>({
    state: "loading",
  }, requestKey);
  useEffect(() => {
    if (!isCurrent()) return;
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
        if (cancelled || !isCurrent()) {
          return;
        }

        const result = await parsePptx(buffer);
        if (cancelled || !isCurrent()) {
          revokeObjectUrls(result.objectUrls);
          return;
        }

        cleanupUrlsRef.current = () => revokeObjectUrls(result.objectUrls);
        setSlides(result.slides);
        setActiveSlideIndex(0);
        setStatus({ state: "loaded", slideCount: result.slides.length });
      } catch {
        if (cancelled || !isCurrent() || abortController.signal.aborted) {
          return;
        }
        cleanupUrlsRef.current();
        cleanupUrlsRef.current = () => undefined;
        setSlides([]);
        setStatus({ state: "error" });
      }
    }

    void loadPreview();

    return () => {
      cancelled = true;
      abortController.abort();
      cleanupUrlsRef.current();
      cleanupUrlsRef.current = () => undefined;
    };
  }, [agentId, isCurrent, path, setActiveSlideIndex, setSlides, setStatus]);

  const isLoaded = status.state === "loaded";
  const hasError = status.state === "error";
  const activeSlide = slides[Math.min(activeSlideIndex, Math.max(slides.length - 1, 0))];

  return (
    <>
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
        meta={isLoaded ? t("workspace_file.presentation_loaded", { count: status.slideCount }) : undefined}
        title={fileName}
      />

      <div className="min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)]">
        {hasError ? (
          <div className="soft-scrollbar h-full min-h-0 min-w-0 overflow-auto overscroll-contain p-4">
            <OfficePreviewFailureState
              kind="presentation"
              onRetry={retryPreview}
            />
          </div>
        ) : activeSlide ? (
          <div className="flex h-full min-h-0">
            {slides.length > 1 ? (
              <aside className="soft-scrollbar hidden w-36 shrink-0 overflow-auto border-r divider-subtle bg-(--surface-panel-background) p-3 md:block">
                <div className="space-y-2">
                  {slides.map((slide, index) => (
                    <UiChoiceButton
                      active={index === activeSlideIndex}
                      className={cn(
                        "h-auto w-full flex-col items-stretch justify-start gap-0 p-1 text-left",
                      )}
                      choiceSize="xs"
                      key={slide.id}
                      onClick={() => setActiveSlideIndex(index)}
                      tone="neutral"
                    >
                      <PresentationSlideCanvas className="shadow-none" slide={slide} thumbnail />
                      <span className={cn(
                        "mt-1 block truncate text-left",
                        getUiTypographyClassName({
                          role: "caption",
                          tone: "muted",
                          weight: "medium",
                        }),
                      )}>
                        {index + 1}. {slide.title}
                      </span>
                    </UiChoiceButton>
                  ))}
                </div>
              </aside>
            ) : null}

            <div className="soft-scrollbar min-h-0 flex-1 overflow-auto p-5">
              <div className="mx-auto flex w-full max-w-6xl flex-col gap-3">
                <div className={cn(
                  "flex items-center justify-between gap-3",
                  getUiTypographyClassName({ role: "metadata", tone: "muted" }),
                )}>
                  <span className="min-w-0 truncate">
                    {activeSlideIndex + 1} / {slides.length} · {activeSlide.title}
                  </span>
                  {slides.length > 1 ? (
                    <div className="flex shrink-0 items-center gap-1">
                      <UiIconButton
                        aria-label={t("workspace_file.previous_slide")}
                        disabled={activeSlideIndex <= 0}
                        onClick={() => setActiveSlideIndex((index) => Math.max(index - 1, 0))}
                        size="md"
                        tooltip={t("workspace_file.previous_slide")}
                        variant="surface"
                      >
                        <ChevronLeft className="h-4 w-4" />
                      </UiIconButton>
                      <UiIconButton
                        aria-label={t("workspace_file.next_slide")}
                        disabled={activeSlideIndex >= slides.length - 1}
                        onClick={() => setActiveSlideIndex((index) => Math.min(index + 1, slides.length - 1))}
                        size="md"
                        tooltip={t("workspace_file.next_slide")}
                        variant="surface"
                      >
                        <ChevronRight className="h-4 w-4" />
                      </UiIconButton>
                    </div>
                  ) : null}
                </div>
                <PresentationSlideCanvas slide={activeSlide} />
              </div>
            </div>
          </div>
        ) : (
          <WorkspaceFilePreviewLoading className="h-full" />
        )}
      </div>
    </>
  );
}
