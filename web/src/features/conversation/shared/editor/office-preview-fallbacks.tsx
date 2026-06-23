import { FileSpreadsheet, FileText, LoaderCircle } from "lucide-react";

import { ConversationResizeHandle } from "./conversation-resize-handle";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "./workspace-file-preview-chrome";

interface OfficePreviewFallbackProps {
  agent_id: string;
  embedded?: boolean;
  file_name: string;
  is_preview_focused?: boolean;
  on_resize_start: () => void;
  on_toggle_preview_focus?: () => void;
  path: string;
}

function OfficePreviewFallback({
  agent_id,
  embedded,
  file_name,
  icon,
  label,
  loading_label,
  on_resize_start,
  on_toggle_preview_focus,
  is_preview_focused,
  path,
}: OfficePreviewFallbackProps & {
  icon: "spreadsheet" | "document";
  label: string;
  loading_label: string;
}) {
  const Icon = icon === "spreadsheet" ? FileSpreadsheet : FileText;

  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          aria_label="调整编辑器宽度"
          class_name="flex"
          on_mouse_down={on_resize_start}
        />
      ) : null}

      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agent_id={agent_id} file_name={file_name} path={path} />
            <WorkspaceFilePreviewFocusButton
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
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
        title={file_name}
      />

      <div className="flex min-h-0 flex-1 items-center justify-center bg-[var(--surface-panel-subtle-background)] p-8 text-center">
        <div className="max-w-xs">
          <LoaderCircle className="mx-auto h-8 w-8 animate-spin text-primary" />
          <p className="mt-3 text-sm font-medium text-(--text-strong)">{loading_label}</p>
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
      loading_label="正在加载 xlsx 预览组件"
    />
  );
}

export function DocumentPreviewFallback(props: OfficePreviewFallbackProps) {
  return (
    <OfficePreviewFallback
      {...props}
      icon="document"
      label="docx 预览"
      loading_label="正在加载 docx 预览组件"
    />
  );
}

export function PresentationPreviewFallback(props: OfficePreviewFallbackProps) {
  return (
    <OfficePreviewFallback
      {...props}
      icon="document"
      label="pptx 预览"
      loading_label="正在加载 pptx 预览组件"
    />
  );
}
