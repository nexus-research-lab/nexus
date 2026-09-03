"use client";

import type { ReactNode } from "react";
import { Eye, FileWarning, LoaderCircle } from "lucide-react";

import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "../workspace-file-preview-chrome";
import { OfficePreviewFailureState } from "../office-preview-fallbacks";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { WorkspaceFilePreviewProps } from "../workspace-file-preview-types";
import { SpreadsheetReadonlyWorkbook } from "./spreadsheet-readonly-workbook";
import {
  useSpreadsheetPreview,
  type SpreadsheetPreviewStatus,
} from "./use-spreadsheet-preview";

export function SpreadsheetFilePreview({
  agentId,
  fileName,
  isPreviewFocused,
  onTogglePreviewFocus,
  path,
}: WorkspaceFilePreviewProps) {
  const preview = useSpreadsheetPreview(agentId, path);
  return (
    <>
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
          <SpreadsheetPreviewOverlay
            onRetry={preview.retryPreview}
            status={preview.status}
          />
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
  const { t } = useI18n();
  const statusContent = {
    error: (
      <span className="flex min-w-0 items-center gap-1 text-destructive">
        <FileWarning className="h-3 w-3 shrink-0" />
        <span className="truncate">{t("workspace_file.preview_failed_status")}</span>
      </span>
    ),
    loaded: (
      <span className="flex items-center gap-1 text-(--success)">
        <Eye className="h-3 w-3" />
        {t("workspace_file.spreadsheet_loaded", {
          count: status.state === "loaded" ? status.sheetCount : 0,
        })}
      </span>
    ),
    loading: (
      <span className="flex min-w-0 items-center gap-1">
        <LoaderCircle className={getUiSpinnerClassName({ size: "xs" })} />
        <span className="truncate">{t("workspace_file.preview_loading")}</span>
      </span>
    ),
  } satisfies Record<SpreadsheetPreviewStatus["state"], ReactNode>;
  return statusContent[status.state];
}

function SpreadsheetPreviewOverlay({
  onRetry,
  status,
}: {
  onRetry: () => void;
  status: Exclude<SpreadsheetPreviewStatus, { state: "loaded" }>;
}) {
  const { t } = useI18n();
  const isError = status.state === "error";
  if (isError) {
    return (
      <div className="absolute inset-0 flex items-center justify-center bg-[var(--surface-panel-subtle-background)] p-8">
        <OfficePreviewFailureState kind="spreadsheet" onRetry={onRetry} />
      </div>
    );
  }
  return (
    <div className="absolute inset-0 flex items-center justify-center bg-[var(--surface-panel-subtle-background)] p-8 text-center">
      <div className="max-w-xs">
        <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center surface-radius-md border border-(--surface-panel-subtle-border) bg-(--card-default-background)">
          <LoaderCircle
            className={getUiSpinnerClassName({ size: "2xl", tone: "primary" })}
          />
        </div>
        <p className="text-sm font-medium text-(--text-strong)">
          {t("workspace_file.preview_loading")}
        </p>
      </div>
    </div>
  );
}
