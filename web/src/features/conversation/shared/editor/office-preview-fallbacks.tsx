import type { ComponentType } from "react";
import { FileSpreadsheet, FileText, LoaderCircle } from "lucide-react";

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
  icon: ComponentType<{ className?: string }>;
  label: string;
  loadingLabel: string;
}

const OFFICE_PREVIEW_DESCRIPTORS: Record<
  OfficePreviewKind,
  OfficePreviewDescriptor
> = {
  document: {
    icon: FileText,
    label: "docx 预览",
    loadingLabel: "正在加载 docx 预览组件",
  },
  presentation: {
    icon: FileText,
    label: "pptx 预览",
    loadingLabel: "正在加载 pptx 预览组件",
  },
  spreadsheet: {
    icon: FileSpreadsheet,
    label: "xlsx 预览",
    loadingLabel: "正在加载 xlsx 预览组件",
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
  const descriptor = OFFICE_PREVIEW_DESCRIPTORS[kind];
  const Icon = descriptor.icon;
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
          <>
            <span className="flex items-center gap-1">
              <Icon className="h-3 w-3" />
              {descriptor.label}
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
          <p className="mt-3 text-sm font-medium text-(--text-strong)">
            {descriptor.loadingLabel}
          </p>
        </div>
      </div>
    </>
  );
}
