// INPUT: 工作区 PPTX 标识、预览聚焦状态与文件动作。
// OUTPUT: 可重试的幻灯片预览、共享标题栏、缩略图选择与本地化翻页动作。
// POS: 演示文稿预览视图；解析归 presentation parser，通用动作与排版归 shared/ui。
"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, Eye, FileWarning, LoaderCircle } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { fetchOfficePreviewBuffer } from "../office-preview-resource";
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
  const previewKey = `${agentId}\x1f${path}`;
  const [slides, setSlides] = useResettableState<PresentationSlide[]>([], previewKey);
  const [activeSlideIndex, setActiveSlideIndex] = useResettableState(0, previewKey);
  const [status, setStatus] = useResettableState<PresentationPreviewStatus>({
    state: "loading",
  }, previewKey);
  const [retryRevision, setRetryRevision] = useState(0);
  const retryPreview = useCallback(() => {
    setRetryRevision((current) => current + 1);
  }, []);

  useEffect(() => {
    const abortController = new AbortController();
    let cancelled = false;

    cleanupUrlsRef.current();
    cleanupUrlsRef.current = () => undefined;
    setStatus({ state: "loading" });

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

        setStatus({ state: "loading" });
        const result = await parsePptx(buffer);
        if (cancelled) {
          revokeObjectUrls(result.objectUrls);
          return;
        }

        cleanupUrlsRef.current = () => revokeObjectUrls(result.objectUrls);
        setSlides(result.slides);
        setActiveSlideIndex(0);
        setStatus({ state: "loaded", slideCount: result.slides.length });
      } catch {
        if (cancelled || abortController.signal.aborted) {
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
  }, [agentId, path, retryRevision, setActiveSlideIndex, setSlides, setStatus]);

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
        meta={(
          hasError ? (
            <span className="flex items-center gap-1 text-destructive">
              <FileWarning className="h-3 w-3" />
              {t("workspace_file.preview_failed_status")}
            </span>
          ) : isLoaded ? (
            <span className="flex items-center gap-1 text-(--success)">
              <Eye className="h-3 w-3" />
              {t("workspace_file.presentation_loaded", {
                count: status.slideCount,
              })}
            </span>
          ) : (
            <span className="flex items-center gap-1">
              <LoaderCircle className={getUiSpinnerClassName({ size: "xs" })} />
              {t("workspace_file.preview_loading")}
            </span>
          )
        )}
        title={fileName}
      />

      <div className="min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)]">
        {hasError ? (
          <div className="flex h-full items-center justify-center p-8 text-center">
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
          <div className="flex h-full items-center justify-center p-8 text-center">
            <div className="max-w-xs">
              <LoaderCircle
                className={getUiSpinnerClassName(
                  { size: "2xl", tone: "primary" },
                  "mx-auto",
                )}
              />
              <p className={cn(
                "mt-3",
                getUiTypographyClassName({ role: "body", tone: "strong", weight: "medium" }),
              )}>
                {t("workspace_file.preview_loading")}
              </p>
            </div>
          </div>
        )}
      </div>
    </>
  );
}
