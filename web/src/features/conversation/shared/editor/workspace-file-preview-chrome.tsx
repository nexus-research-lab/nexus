"use client";

import { type ReactNode } from "react";
import { Download, FolderOpen, Maximize2, Minimize2 } from "lucide-react";

import { download_workspace_file_api } from "@/lib/api/agent-manage-api";
import { get_workspace_file_external_action_copy } from "@/lib/workspace-file-action";
import { cn } from "@/lib/utils";

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
  agent_id,
  path,
  file_name,
  label,
}: {
  agent_id: string;
  path: string;
  file_name: string;
  label?: string;
}) {
  const file_action_copy = get_workspace_file_external_action_copy(file_name);
  const visible_label = label ?? file_action_copy.label;
  const handle_external_action = () => {
    void download_workspace_file_api(agent_id, path, file_name).catch((error) => {
      console.error(`[WorkspaceFileDownloadButton] ${file_action_copy.label} workspace 文件失败:`, error);
    });
  };

  return (
    <button
      aria-label={file_action_copy.aria_label}
      className={WORKSPACE_FILE_TOOLBAR_BUTTON_CLASS_NAME}
      onClick={handle_external_action}
      title={file_action_copy.title}
      type="button"
    >
      {file_action_copy.mode === "reveal" ? (
        <FolderOpen className="h-3.5 w-3.5" />
      ) : (
        <Download className="h-3.5 w-3.5" />
      )}
      <span className="max-xl:hidden">{visible_label}</span>
    </button>
  );
}

export function WorkspaceFileToolbarButton({
  children,
  disabled = false,
  on_click,
  title,
}: {
  children: ReactNode;
  disabled?: boolean;
  on_click: () => void;
  title?: string;
}) {
  return (
    <button
      className={WORKSPACE_FILE_TOOLBAR_BUTTON_CLASS_NAME}
      disabled={disabled}
      onMouseDown={(event) => event.preventDefault()}
      onClick={on_click}
      title={title}
      type="button"
    >
      {children}
    </button>
  );
}

export function WorkspaceFilePreviewFocusButton({
  is_preview_focused = false,
  on_toggle_preview_focus,
}: {
  is_preview_focused?: boolean;
  on_toggle_preview_focus?: () => void;
}) {
  if (!on_toggle_preview_focus) {
    return null;
  }

  return (
    <WorkspaceFileToolbarButton
      on_click={on_toggle_preview_focus}
      title={is_preview_focused ? "还原文件树" : "聚焦预览"}
    >
      {is_preview_focused ? <Minimize2 className="h-3.5 w-3.5" /> : <Maximize2 className="h-3.5 w-3.5" />}
      <span className="max-xl:hidden">{is_preview_focused ? "还原" : "放大"}</span>
    </WorkspaceFileToolbarButton>
  );
}
