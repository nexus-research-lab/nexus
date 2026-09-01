import { LoaderCircle } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiResourceState } from "@/shared/ui/display/resource-state";
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

interface OfficePreviewDescriptor {
  failureTitleKey: TranslationKey;
}

const OFFICE_PREVIEW_DESCRIPTORS: Record<
  OfficePreviewKind,
  OfficePreviewDescriptor
> = {
  document: {
    failureTitleKey: "workspace_file.document_preview_failed",
  },
  presentation: {
    failureTitleKey: "workspace_file.presentation_preview_failed",
  },
  spreadsheet: {
    failureTitleKey: "workspace_file.spreadsheet_preview_failed",
  },
};

export function OfficePreviewFallback({
  agentId,
  fileName,
  isPreviewFocused,
  kind,
  onTogglePreviewFocus,
  path,
}: WorkspaceFilePreviewProps & { kind: OfficePreviewKind }) {
  const { t } = useI18n();
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
        meta={(
          <span className="flex items-center gap-1">
            <LoaderCircle className="h-3 w-3 animate-spin" />
            {t("workspace_file.preview_loading")}
          </span>
        )}
        title={fileName}
      />
      <div
        className="flex min-h-0 flex-1 items-center justify-center bg-[var(--surface-panel-subtle-background)] p-8 text-center"
        data-office-preview-kind={kind}
      >
        <div className="max-w-xs">
          <LoaderCircle className="mx-auto h-8 w-8 animate-spin text-primary" />
          <p className="mt-3 text-sm font-medium text-(--text-strong)">
            {t("workspace_file.preview_loading")}
          </p>
        </div>
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
      className="m-auto min-h-0 w-full max-w-lg py-5"
      impact={t("workspace_file.office_preview_failed_impact")}
      primaryAction={{
        label: t("workspace_file.retry_preview"),
        onClick: onRetry,
      }}
      size="sm"
      state="error"
      title={t(OFFICE_PREVIEW_DESCRIPTORS[kind].failureTitleKey)}
      urgency="polite"
      variant="card"
    />
  );
}
