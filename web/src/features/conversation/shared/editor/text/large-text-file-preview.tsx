// INPUT: 已确认超过整文件读取上限的 UTF-8 workspace 文本。
// OUTPUT: 每次至多 512KiB 的只读 Range 分段预览。
// POS: 大型文本预览边界；不把片段拼接成整文件，也不提供编辑语义。
"use client";

import { useCallback, useEffect, useState } from "react";
import { ChevronLeft, ChevronRight, LoaderCircle } from "lucide-react";

import { getWorkspaceFileTextChunkApi } from "@/lib/api/agent/agent-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import type { WorkspaceFileTextChunk } from "@/types/agent/agent";

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
  const { t } = useI18n();
  const [offsets, setOffsets] = useState([0]);
  const [pageIndex, setPageIndex] = useState(0);
  const [chunk, setChunk] = useState<WorkspaceFileTextChunk | null>(null);
  const [loadState, setLoadState] = useState<"error" | "loaded" | "loading">("loading");
  const [retryRevision, setRetryRevision] = useState(0);
  const offset = offsets[pageIndex] ?? 0;

  useEffect(() => {
    const controller = new AbortController();
    setChunk(null);
    setLoadState("loading");
    void getWorkspaceFileTextChunkApi(agentId, path, offset, controller.signal)
      .then((value) => {
        setChunk(value);
        setLoadState("loaded");
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") {
          return;
        }
        setLoadState("error");
      });
    return () => controller.abort();
  }, [agentId, offset, path, retryRevision]);

  const loadFromStart = useCallback(() => {
    setOffsets([0]);
    setPageIndex(0);
    setRetryRevision((current) => current + 1);
  }, []);
  const loadPrevious = useCallback(() => {
    setPageIndex((current) => Math.max(0, current - 1));
  }, []);
  const loadNext = useCallback(() => {
    const nextOffset = chunk?.nextOffset;
    if (nextOffset === null || nextOffset === undefined) {
      return;
    }
    setOffsets((current) => [
      ...current.slice(0, pageIndex + 1),
      nextOffset,
    ]);
    setPageIndex((current) => current + 1);
  }, [chunk?.nextOffset, pageIndex]);

  const rangeLabel = chunk
    ? `${chunk.offset.toLocaleString()}–${(chunk.nextOffset ?? chunk.size).toLocaleString()} / ${chunk.size.toLocaleString()} B`
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
        meta={rangeLabel ?? t("workspace_file.chunked_preview")}
        title={fileName}
      />
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden bg-[var(--surface-panel-subtle-background)]">
        {loadState === "error" ? (
          <div className="flex h-full items-center justify-center p-6">
            <UiResourceState
              className="min-h-0 w-full max-w-lg py-5"
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
        ) : loadState === "loading" || !chunk ? (
          <div className="flex h-full items-center justify-center text-sm text-(--text-soft)">
            <LoaderCircle className="mr-2 h-4 w-4 animate-spin motion-reduce:animate-none" />
            {t("workspace_file.loading")}
          </div>
        ) : (
          <>
            <div className="flex shrink-0 items-center justify-between gap-3 border-b divider-subtle px-4 py-2">
              <span className="truncate text-xs text-(--text-soft)">
                {t("workspace_file.chunked_preview")}
              </span>
              <div className="flex shrink-0 items-center gap-2">
                <UiButton
                  disabled={pageIndex === 0}
                  onClick={loadPrevious}
                  size="sm"
                  variant="surface"
                >
                  <ChevronLeft className="h-3.5 w-3.5" />
                  {t("workspace_file.previous_chunk")}
                </UiButton>
                <span className="min-w-8 text-center text-xs text-(--text-soft)">
                  {pageIndex + 1}
                </span>
                <UiButton
                  disabled={chunk.nextOffset === null}
                  onClick={loadNext}
                  size="sm"
                  variant="surface"
                >
                  {t("workspace_file.next_chunk")}
                  <ChevronRight className="h-3.5 w-3.5" />
                </UiButton>
              </div>
            </div>
            <pre className="soft-scrollbar min-h-0 flex-1 overflow-auto whitespace-pre-wrap break-words p-4 font-mono text-xs leading-5 text-(--text-default)">
              {chunk.content}
            </pre>
          </>
        )}
      </div>
    </>
  );
}
