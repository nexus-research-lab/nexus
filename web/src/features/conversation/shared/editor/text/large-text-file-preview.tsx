// INPUT: exact Agent/path 的分段读取事实与文件 chrome 命令。
// OUTPUT: 具名可键盘滚动的单片段正文、可换行分页操作与共享状态/源码度量。
// POS: 大型文本预览边界；不把片段拼接成整文件，也不提供编辑语义。
"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UI_SOURCE_PREVIEW_SCROLL_CLASS_NAME, UI_SOURCE_TEXT_CLASS_NAME } from "@/shared/ui/form/source-text-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { useLargeTextFilePreview } from "./use-large-text-file-preview";
import { WorkspaceFilePreviewLoading } from "../workspace-file-preview-loading";

import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "../workspace-file-preview-chrome";
import type { WorkspaceFilePreviewProps } from "../workspace-file-preview-types";

export function LargeTextFilePreview({
  agentId,
  fileName,
  isPreviewFocused,
  onTogglePreviewFocus,
  path,
}: WorkspaceFilePreviewProps) {
  const { locale, t } = useI18n();
  const { chunk, hasError, isLoading, pageIndex, loadFromStart, loadPrevious, loadNext } = useLargeTextFilePreview(agentId, path);
  const metadataClass = getUiTypographyClassName({ role: "metadata", tone: "soft" });

  const rangeLabel = chunk
    ? `${chunk.offset.toLocaleString(locale)}–${(chunk.nextOffset ?? chunk.size).toLocaleString(locale)} / ${chunk.size.toLocaleString(locale)} B`
    : null;

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
        meta={rangeLabel ?? undefined}
        title={fileName}
      />
      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-[var(--surface-panel-subtle-background)]">
        <div className="flex min-w-0 shrink-0 flex-wrap items-center gap-x-3 gap-y-2 border-b divider-subtle px-4 py-2">
          <span className={cn("min-w-0", metadataClass)}>
            {t("workspace_file.chunked_preview")}
          </span>
          <div className="ml-auto flex max-w-full flex-wrap items-center gap-2">
            <UiButton
              disabled={isLoading || hasError || pageIndex === 0}
              onClick={loadPrevious}
              size="sm"
              variant="surface"
            >
              <ChevronLeft aria-hidden="true" className="h-3.5 w-3.5" />
              {t("workspace_file.previous_chunk")}
            </UiButton>
            <span className={cn("min-w-8 text-center tabular-nums", metadataClass)}>
              {pageIndex + 1}
            </span>
            <UiButton
              disabled={isLoading || hasError || !chunk || chunk.nextOffset === null}
              onClick={loadNext}
              size="sm"
              variant="surface"
            >
              {t("workspace_file.next_chunk")}
              <ChevronRight aria-hidden="true" className="h-3.5 w-3.5" />
            </UiButton>
          </div>
        </div>
        {hasError ? (
          <div className="soft-scrollbar min-h-0 flex-1 overflow-auto overscroll-contain p-4">
            <UiResourceState
              className="min-h-0 w-full py-5"
              impact={t("workspace_file.chunk_load_failed_impact")}
              primaryAction={{
                label: t("workspace_file.load_from_start"),
                onClick: loadFromStart,
              }}
              size="sm"
              state="error"
              title={t("workspace_file.chunk_load_failed")}
              urgency="polite"
              variant="card"
            />
          </div>
        ) : isLoading || !chunk ? (
          <WorkspaceFilePreviewLoading className="h-full" title={t("workspace_file.loading")} />
        ) : (
          // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- This named read-only scroll region needs a Tab stop for native keyboard scrolling.
          <pre tabIndex={0}
            aria-label={fileName}
            className={cn("flex-1 whitespace-pre-wrap break-words p-4 text-(--text-default)", UI_SOURCE_PREVIEW_SCROLL_CLASS_NAME, UI_SOURCE_TEXT_CLASS_NAME)}
            role="region"
          >
            {chunk.content}
          </pre>
        )}
      </div>
    </>
  );
}
