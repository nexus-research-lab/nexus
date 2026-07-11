"use client";

import { type ReactNode } from "react";
import { Download, FolderOpen, Maximize2, Minimize2 } from "lucide-react";

import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import { cn } from "@/shared/ui/class-name";

const WORKSPACE_FILE_TOOLBAR_BUTTON_CLASS_NAME = cn(
  "inline-flex h-8 items-center justify-center gap-1.5 rounded-[10px] border px-2.5 text-[11px] font-semibold leading-none transition-colors",
  "border-(--divider-subtle-color) bg-(--surface-panel-background) text-(--text-default)",
  "hover:border-primary/30 hover:bg-primary/8 hover:text-primary",
  "disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) disabled:hover:border-(--divider-subtle-color) disabled:hover:bg-(--surface-panel-background) disabled:hover:text-(--text-default)",
  "max-xl:w-8 max-xl:px-0 max-xl:gap-0",
);

export function WorkspaceFilePreviewHeader({
  actions,
  embedded,
  meta,
  title,
}: {
  actions: ReactNode;
  embedded?: boolean;
  meta?: ReactNode;
  title: string;
}) {
  if (embedded) {
    return (
      <div className="overflow-hidden border-b divider-subtle px-3 pt-0 pb-2">
        <div className="flex min-w-0 items-center justify-between gap-3">
          <p
            className="min-w-0 flex-1 truncate text-xs font-semibold uppercase leading-5 tracking-[0.16em] text-muted-foreground"
            title={title}
          >
            {title}
          </p>
          <div className="flex shrink-0 items-center gap-2 self-start">
            {actions}
          </div>
        </div>
        {meta ? (
          <div className="mt-1 flex min-w-0 items-center gap-2 text-[10px] text-muted-foreground">
            {meta}
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <div className="flex h-14 min-w-0 items-center justify-between overflow-hidden border-b divider-subtle px-4">
      <div className="min-w-0 flex-1 overflow-hidden pr-3">
        <p
          className="w-full truncate text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground"
          title={title}
        >
          {title}
        </p>
        {meta ? (
          <div className="mt-1 flex min-w-0 items-center gap-2 text-[10px] text-muted-foreground">
            {meta}
          </div>
        ) : null}
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {actions}
      </div>
    </div>
  );
}

export function WorkspaceFileDownloadButton({
  agentId: agentId,
  path,
  fileName: fileName,
  label,
}: {
  agentId: string;
  path: string;
  fileName: string;
  label?: string;
}) {
  const fileActionCopy = getWorkspaceFileExternalActionCopy(fileName);
  const visibleLabel = label ?? fileActionCopy.label;
  const handleExternalAction = () => {
    void downloadWorkspaceFileApi(agentId, path, fileName).catch((error) => {
      console.error(`[WorkspaceFileDownloadButton] ${fileActionCopy.label} workspace 文件失败:`, error);
    });
  };

  return (
    <button
      aria-label={fileActionCopy.ariaLabel}
      className={WORKSPACE_FILE_TOOLBAR_BUTTON_CLASS_NAME}
      onClick={handleExternalAction}
      title={fileActionCopy.title}
      type="button"
    >
      {fileActionCopy.mode === "reveal" ? (
        <FolderOpen className="h-3.5 w-3.5" />
      ) : (
        <Download className="h-3.5 w-3.5" />
      )}
      <span className="max-xl:hidden">{visibleLabel}</span>
    </button>
  );
}

export function WorkspaceFileToolbarButton({
  children,
  disabled = false,
  onClick: onClick,
  title,
}: {
  children: ReactNode;
  disabled?: boolean;
  onClick: () => void;
  title?: string;
}) {
  return (
    <button
      className={WORKSPACE_FILE_TOOLBAR_BUTTON_CLASS_NAME}
      disabled={disabled}
      onMouseDown={(event) => event.preventDefault()}
      onClick={onClick}
      title={title}
      type="button"
    >
      {children}
    </button>
  );
}

export function WorkspaceFilePreviewFocusButton({
  isPreviewFocused: isPreviewFocused = false,
  onTogglePreviewFocus: onTogglePreviewFocus,
}: {
  isPreviewFocused?: boolean;
  onTogglePreviewFocus?: () => void;
}) {
  if (!onTogglePreviewFocus) {
    return null;
  }

  return (
    <WorkspaceFileToolbarButton
      onClick={onTogglePreviewFocus}
      title={isPreviewFocused ? "还原文件树" : "聚焦预览"}
    >
      {isPreviewFocused ? <Minimize2 className="h-3.5 w-3.5" /> : <Maximize2 className="h-3.5 w-3.5" />}
      <span className="max-xl:hidden">{isPreviewFocused ? "还原" : "放大"}</span>
    </WorkspaceFileToolbarButton>
  );
}
