// INPUT: Workbook preview controller and file actions.
// OUTPUT: Sheet count, workbook and one shared state surface; failed reads stay scrollable in short panels.
// POS: Spreadsheet presentation; workbook selection and parsing stay with their existing owners.
"use client";

import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "../workspace-file-preview-chrome";
import { OfficePreviewFailureState } from "../office-preview-fallbacks";
import { WorkspaceFilePreviewLoading } from "../workspace-file-preview-loading";
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
  const { t } = useI18n();
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
        meta={preview.status.state === "loaded" ? t("workspace_file.spreadsheet_loaded", { count: preview.status.sheetCount }) : undefined}
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

function SpreadsheetPreviewOverlay({
  onRetry,
  status,
}: {
  onRetry: () => void;
  status: Exclude<SpreadsheetPreviewStatus, { state: "loaded" }>;
}) {
  const isError = status.state === "error";
  if (isError) {
    return (
      <div className="soft-scrollbar absolute inset-0 overflow-auto overscroll-contain bg-[var(--surface-panel-subtle-background)] p-4">
        <OfficePreviewFailureState kind="spreadsheet" onRetry={onRetry} />
      </div>
    );
  }
  return (
    <WorkspaceFilePreviewLoading className="absolute inset-0" />
  );
}
