// INPUT: Office preview kind, file scope, focus action and an explicit retry callback.
// OUTPUT: Bounded shared loading/failure surfaces with one file header and domain-specific failure copy.
// POS: Lazy Office module fallback; no binary fetch or parsing.

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { WorkspaceFilePreviewLoading } from "./workspace-file-preview-loading";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "./workspace-file-preview-chrome";
import type { WorkspaceFilePreviewProps } from "./workspace-file-preview-types";

export type OfficePreviewKind =
  | "document"
  | "presentation"
  | "spreadsheet";

const OFFICE_PREVIEW_DESCRIPTORS: Record<
  OfficePreviewKind,
  TranslationKey
> = {
  document: "workspace_file.document_preview_failed",
  presentation: "workspace_file.presentation_preview_failed",
  spreadsheet: "workspace_file.spreadsheet_preview_failed",
};

export function OfficePreviewFallback({
  agentId,
  fileName,
  isPreviewFocused,
  kind,
  onTogglePreviewFocus,
  path,
}: WorkspaceFilePreviewProps & { kind: OfficePreviewKind }) {
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
        title={fileName}
      />
      <div
        className="soft-scrollbar min-h-0 min-w-0 flex-1 overflow-auto overscroll-contain bg-[var(--surface-panel-subtle-background)]"
        data-office-preview-kind={kind}
      >
        <WorkspaceFilePreviewLoading className="min-h-full" />
      </div>
    </>
  );
}

export function OfficePreviewFailureState({
  kind,
  onRetry,
}: {
  kind: OfficePreviewKind;
  onRetry: () => void;
}) {
  const { t } = useI18n();
  return (
    <UiResourceState
      className="mx-auto min-h-0 w-full max-w-lg py-5"
      impact={t("workspace_file.office_preview_failed_impact")}
      primaryAction={{
        label: t("workspace_file.retry_preview"),
        onClick: onRetry,
      }}
      size="sm"
      state="error"
      title={t(OFFICE_PREVIEW_DESCRIPTORS[kind])}
      urgency="polite"
      variant="card"
    />
  );
}
