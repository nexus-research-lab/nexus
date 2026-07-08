import { FileSpreadsheet, FileText, LoaderCircle } from "lucide-react";

import { ConversationResizeHandle } from "./conversation-resize-handle";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "./workspace-file-preview-chrome";

interface OfficePreviewFallbackProps {
  agentId: string;
  embedded?: boolean;
  fileName: string;
  isPreviewFocused?: boolean;
  onResizeStart: () => void;
  onTogglePreviewFocus?: () => void;
  path: string;
}

function OfficePreviewFallback({
  agentId: agentId,
  embedded,
  fileName: fileName,
  icon,
  label,
  loadingLabel: loadingLabel,
  onResizeStart: onResizeStart,
  onTogglePreviewFocus: onTogglePreviewFocus,
  isPreviewFocused: isPreviewFocused,
  path,
}: OfficePreviewFallbackProps & {
  icon: "spreadsheet" | "document";
  label: string;
  loadingLabel: string;
}) {
  const Icon = icon === "spreadsheet" ? FileSpreadsheet : FileText;

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
              <Icon className="h-3 w-3" />
              {label}
            </span>
            <span className="flex items-center gap-1">
              <LoaderCircle className="h-3 w-3 animate-spin" />
              加载预览组件中
            </span>
          </>
        )}
        title={fileName}
      />

      <div className="flex min-h-0 flex-1 items-center justify-center bg-[var(--surface-panel-subtle-background)] p-8 text-center">
        <div className="max-w-xs">
          <LoaderCircle className="mx-auto h-8 w-8 animate-spin text-primary" />
          <p className="mt-3 text-sm font-medium text-(--text-strong)">{loadingLabel}</p>
        </div>
      </div>
    </>
  );
}

export function SpreadsheetPreviewFallback(props: OfficePreviewFallbackProps) {
  return (
    <OfficePreviewFallback
      {...props}
      icon="spreadsheet"
      label="xlsx 预览"
      loadingLabel="正在加载 xlsx 预览组件"
    />
  );
}

export function DocumentPreviewFallback(props: OfficePreviewFallbackProps) {
  return (
    <OfficePreviewFallback
      {...props}
      icon="document"
      label="docx 预览"
      loadingLabel="正在加载 docx 预览组件"
    />
  );
}

export function PresentationPreviewFallback(props: OfficePreviewFallbackProps) {
  return (
    <OfficePreviewFallback
      {...props}
      icon="document"
      label="pptx 预览"
      loadingLabel="正在加载 pptx 预览组件"
    />
  );
}
