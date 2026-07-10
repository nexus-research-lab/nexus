"use client";

import type { ReactNode } from "react";
import { Eye, FileSpreadsheet, FileWarning, LoaderCircle } from "lucide-react";

import { ConversationResizeHandle } from "../conversation-resize-handle";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "../workspace-file-preview-chrome";
import type { WorkspaceFilePreviewProps } from "../workspace-file-preview-types";
import { SpreadsheetReadonlyWorkbook } from "./spreadsheet-readonly-workbook";
import {
  useSpreadsheetPreview,
  type SpreadsheetPreviewStatus,
} from "./use-spreadsheet-preview";

export function SpreadsheetFilePreview({
  agentId,
  embedded,
  fileName,
  isPreviewFocused,
  onResizeStart,
  onTogglePreviewFocus,
  path,
}: WorkspaceFilePreviewProps) {
  const preview = useSpreadsheetPreview(agentId, path);
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
            <WorkspaceFileDownloadButton
              agentId={agentId}
              fileName={fileName}
              path={path}
            />
            <WorkspaceFilePreviewFocusButton
              isPreviewFocused={isPreviewFocused}
              onTogglePreviewFocus={onTogglePreviewFocus}
            />
          </>
        )}
        embedded={embedded}
        meta={<SpreadsheetPreviewMeta status={preview.status} />}
        title={fileName}
      />
      <div className="relative min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)]">
        {preview.workbook ? (
          <SpreadsheetReadonlyWorkbook
            activeSheetIndex={preview.activeSheetIndex}
            onSelectSheet={preview.setActiveSheetIndex}
            workbook={preview.workbook}
          />
        ) : null}
        {preview.status.state !== "loaded" ? (
          <SpreadsheetPreviewOverlay status={preview.status} />
        ) : null}
      </div>
    </>
  );
}

function SpreadsheetPreviewMeta({
  status,
}: {
  status: SpreadsheetPreviewStatus;
}) {
  const statusContent = {
    error: (
      <span className="flex min-w-0 items-center gap-1 text-destructive">
        <FileWarning className="h-3 w-3 shrink-0" />
        <span className="truncate">
          {status.state === "error" ? status.message : ""}
        </span>
      </span>
    ),
    loaded: (
      <span className="flex items-center gap-1 text-(--success)">
        <Eye className="h-3 w-3" />
        已加载 {status.state === "loaded" ? status.sheetCount : 0} 个工作表
      </span>
    ),
    loading: (
      <span className="flex min-w-0 items-center gap-1">
        <LoaderCircle className="h-3 w-3 shrink-0 animate-spin" />
        <span className="truncate">
          {status.state === "loading" ? status.message : ""}
        </span>
      </span>
    ),
  } satisfies Record<SpreadsheetPreviewStatus["state"], ReactNode>;
  return (
    <>
      <span className="flex items-center gap-1">
        <FileSpreadsheet className="h-3 w-3" />
        xlsx 预览
      </span>
      {statusContent[status.state]}
    </>
  );
}

function SpreadsheetPreviewOverlay({
  status,
}: {
  status: Exclude<SpreadsheetPreviewStatus, { state: "loaded" }>;
}) {
  const isError = status.state === "error";
  return (
    <div className="absolute inset-0 flex items-center justify-center bg-[var(--surface-panel-subtle-background)] p-8 text-center">
      <div className="max-w-xs">
        <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl border border-(--surface-panel-subtle-border) bg-(--card-default-background)">
          {isError
            ? <FileWarning className="h-7 w-7 text-(--icon-muted)" />
            : <LoaderCircle className="h-7 w-7 animate-spin text-primary" />}
        </div>
        <p className="text-sm font-medium text-(--text-strong)">
          {isError ? "xlsx 预览失败" : "正在准备表格预览"}
        </p>
        <p className="mt-2 text-xs leading-5 text-(--text-soft)">
          {status.message}
        </p>
      </div>
    </div>
  );
}
