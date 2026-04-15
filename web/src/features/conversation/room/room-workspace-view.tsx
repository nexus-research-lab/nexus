"use client";

import { ReactNode, useEffect, useRef, useState } from "react";
import {
  FolderOpen,
  FolderTree,
  Upload,
  LoaderCircle,
  FilePlus,
  FolderPlus,
  GripVertical,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/workspace-surface-header";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/workspace-surface-view";
import { Agent } from "@/types/agent";
import { cn } from "@/lib/utils";
import { ConfirmDialog, PromptDialog } from "@/shared/ui/dialog/confirm-dialog";
import { EditorPanel } from "@/features/conversation/shared/context/editor-panel";
import {
  useRoomWorkspaceController,
} from "./use-room-workspace-controller";
import { RoomAgentSwitcher } from "./room-agent-switcher";
import { WorkspaceContextMenu } from "./workspace/workspace-context-menu";
import { WorkspaceFileTree } from "./workspace/workspace-file-tree";

interface RoomWorkspaceViewProps {
  active_workspace_path: string | null;
  agent_id: string;
  header_action?: ReactNode;
  is_dm: boolean;
  is_editor_open: boolean;
  room_members: Agent[];
  on_close_workspace_pane: () => void;
  on_open_workspace_file: (path: string | null) => void;
}

const WORKSPACE_FILE_LIST_DEFAULT_WIDTH = 248;
const WORKSPACE_FILE_LIST_MIN_WIDTH = 248;
const WORKSPACE_FILE_LIST_MAX_WIDTH = 300;

// ── main view ──────────────────────────────────────────────────────────────

export function RoomWorkspaceView({
  active_workspace_path,
  agent_id,
  header_action,
  is_dm,
  is_editor_open,
  room_members,
  on_close_workspace_pane,
  on_open_workspace_file,
}: RoomWorkspaceViewProps) {
  const { t } = useI18n();
  const file_input_ref = useRef<HTMLInputElement>(null);
  const workspace_panel_ref = useRef<HTMLDivElement>(null);
  const [file_list_width, set_file_list_width] = useState(WORKSPACE_FILE_LIST_DEFAULT_WIDTH);
  const [is_resizing_file_list, set_is_resizing_file_list] = useState(false);
  const {
    view_agent_id,
    files,
    selected_agent_id,
    set_selected_agent_id,
    is_uploading,
    is_loading_files,
    error_message,
    clear_error_message,
    context_menu,
    prompt_state,
    delete_target,
    focused_directory_path,
    current_directory_label,
    handle_click_file,
    handle_click_directory,
    handle_upload_click,
    handle_file_select,
    open_create_prompt,
    open_rename_prompt,
    handle_prompt_confirm,
    handle_confirm_delete,
    handle_context_menu,
    handle_root_context_menu,
    close_context_menu,
    set_delete_target,
    set_prompt_state,
  } = useRoomWorkspaceController({
    active_workspace_path,
    agent_id,
    is_dm,
    on_open_workspace_file,
    file_input_ref,
  });

  const title_trailing = !is_dm && room_members.length > 1 ? (
    <RoomAgentSwitcher
      members={room_members}
      selected_id={selected_agent_id}
      on_select={set_selected_agent_id}
    />
  ) : null;

  useEffect(() => {
    if (!is_resizing_file_list) {
      return;
    }

    const handle_mouse_move = (event: MouseEvent) => {
      const bounds = workspace_panel_ref.current?.getBoundingClientRect();
      if (!bounds) {
        return;
      }

      const next_width = event.clientX - bounds.left;
      set_file_list_width(
        Math.min(
          Math.max(next_width, WORKSPACE_FILE_LIST_MIN_WIDTH),
          WORKSPACE_FILE_LIST_MAX_WIDTH,
        ),
      );
    };

    const handle_mouse_up = () => {
      set_is_resizing_file_list(false);
    };

    window.addEventListener("mousemove", handle_mouse_move);
    window.addEventListener("mouseup", handle_mouse_up);

    return () => {
      window.removeEventListener("mousemove", handle_mouse_move);
      window.removeEventListener("mouseup", handle_mouse_up);
    };
  }, [is_resizing_file_list]);

  return (
    <>
      <input
        ref={file_input_ref}
        type="file"
        className="hidden"
        multiple
        onChange={handle_file_select}
      />

      <WorkspaceSurfaceView
        action={header_action}
        body_class_name="px-4 pt-1 pb-0 sm:px-5 xl:px-6"
        body_scrollable={false}
        content_class_name="flex h-full min-h-0 min-w-0 gap-4"
        eyebrow={t("room.workspace")}
        max_width_class_name="max-w-none"
        show_eyebrow={false}
        title={t("room.workspace_title")}
        title_trailing={title_trailing}
      >
        <div
          ref={workspace_panel_ref}
          className={cn("flex h-full min-h-0 min-w-0 flex-1", is_resizing_file_list && "cursor-col-resize select-none")}
        >
          <div
            className="flex min-h-0 shrink-0 flex-col"
            style={{ width: `${file_list_width}px` }}
          >
            <div className="mb-3 inline-flex min-w-0 items-center gap-2 rounded-full border border-(--divider-subtle-color) px-3 py-1.5 text-[12px] text-(--text-default)">
              {focused_directory_path ? (
                <FolderOpen className="h-3.5 w-3.5 shrink-0 text-[var(--accent)]" />
              ) : (
                <FolderTree className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
              )}
              <span className="truncate font-medium text-(--text-strong)">{current_directory_label}</span>
            </div>

            <div className="mb-4 flex flex-wrap items-center gap-3">
              <WorkspaceSurfaceToolbarAction onClick={() => handle_upload_click()} disabled={is_uploading} tone="primary">
                {is_uploading ? (
                  <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Upload className="h-3.5 w-3.5" />
                )}
                {t(is_uploading ? "room.workspace_uploading" : "room.workspace_action_upload")}
              </WorkspaceSurfaceToolbarAction>

              <WorkspaceSurfaceToolbarAction onClick={() => open_create_prompt("directory")}>
                <FolderPlus className="h-3.5 w-3.5" />
                {t("room.workspace_action_new_folder")}
              </WorkspaceSurfaceToolbarAction>

              <WorkspaceSurfaceToolbarAction onClick={() => open_create_prompt("file")}>
                <FilePlus className="h-3.5 w-3.5" />
                {t("room.workspace_action_new_file")}
              </WorkspaceSurfaceToolbarAction>
            </div>

            {error_message ? (
              <div className="mb-4 flex items-center justify-between rounded-2xl border border-destructive/20 bg-destructive/6 px-4 py-3 text-sm text-destructive">
                <span className="min-w-0 flex-1 truncate">{error_message}</span>
                <button
                  type="button"
                  className="ml-3 shrink-0 rounded-md px-2 py-1 text-xs font-medium transition hover:bg-destructive/10"
                  onClick={clear_error_message}
                >
                  {t("common.close")}
                </button>
              </div>
            ) : null}

            <div className="min-h-0 flex-1 overflow-hidden" onContextMenu={handle_root_context_menu}>
              {files.length > 0 ? (
                <div className="soft-scrollbar h-full overflow-y-auto py-1">
                  <WorkspaceFileTree
                    entries={files}
                    active_path={active_workspace_path}
                    focused_directory_path={focused_directory_path}
                    on_click_file={handle_click_file}
                    on_click_directory={handle_click_directory}
                    on_rename_entry={open_rename_prompt}
                    on_delete_entry={set_delete_target}
                    on_context_menu={handle_context_menu}
                  />
                </div>
              ) : is_loading_files ? (
                <div className="flex h-full items-center justify-center text-(--text-soft)">
                  <LoaderCircle className="h-4 w-4 animate-spin" />
                </div>
              ) : (
                <div className="rounded-[24px] border border-(--divider-subtle-color) px-6 py-10 text-center">
                  <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-default) shadow-(--surface-avatar-shadow)">
                    <FolderTree className="h-4 w-4" />
                  </div>
                  <p className="mt-4 text-[15px] font-semibold text-(--text-strong)">
                    {t("room.no_files")}
                  </p>
                  <p className="mt-1 text-[12px] leading-6 text-(--text-soft)">
                    {t("room.workspace_empty_description")}
                  </p>
                </div>
              )}
            </div>
          </div>

          <button
            aria-label="调整文件列表宽度"
            className="hidden h-full w-6 shrink-0 cursor-col-resize items-center justify-center text-muted-foreground/60 transition-colors hover:text-primary lg:flex"
            onMouseDown={() => set_is_resizing_file_list(true)}
            type="button"
          >
            <GripVertical className="h-4 w-4" />
          </button>

          <div className="min-h-0 min-w-0 flex-1 overflow-hidden">
            <EditorPanel
              agent_id={view_agent_id}
              class_name="h-full w-full"
              embedded
              is_open={is_editor_open}
              on_close={on_close_workspace_pane}
              on_resize_start={() => { }}
              path={active_workspace_path}
              width_percent={100}
            />
          </div>
        </div>
      </WorkspaceSurfaceView>

      {/* 上下文菜单 */}
      <WorkspaceContextMenu
        position={context_menu.position}
        entry={context_menu.entry}
        can_create_children={context_menu.entry === null || context_menu.entry.is_dir}
        on_upload={() => handle_upload_click(context_menu.entry?.is_dir ? context_menu.entry.path : null)}
        on_create_file={() => open_create_prompt("file", context_menu.entry?.is_dir ? context_menu.entry.path : null)}
        on_create_folder={() => open_create_prompt("directory", context_menu.entry?.is_dir ? context_menu.entry.path : null)}
        on_rename={() => { if (context_menu.entry) open_rename_prompt(context_menu.entry); }}
        on_delete={() => { if (context_menu.entry) set_delete_target(context_menu.entry); }}
        on_close={close_context_menu}
      />

      <PromptDialog
        is_open={prompt_state !== null}
        title={
          prompt_state?.mode === "create-file"
            ? t("room.workspace_create_file_title")
            : prompt_state?.mode === "create-directory"
              ? t("room.workspace_create_folder_title")
              : t("room.workspace_rename_title")
        }
        placeholder={
          prompt_state?.mode === "create-file"
            ? t("room.workspace_create_file_placeholder")
            : prompt_state?.mode === "create-directory"
              ? t("room.workspace_create_folder_placeholder")
              : t("room.workspace_rename_placeholder")
        }
        default_value={prompt_state?.default_value ?? ""}
        on_confirm={handle_prompt_confirm}
        on_cancel={() => set_prompt_state(null)}
      />

      <ConfirmDialog
        is_open={delete_target !== null}
        title={t("room.workspace_delete_title")}
        message={t("room.workspace_delete_message", { name: delete_target?.name ?? "" })}
        confirm_text={t("common.delete")}
        cancel_text={t("common.cancel")}
        on_confirm={handle_confirm_delete}
        on_cancel={() => set_delete_target(null)}
        variant="danger"
      />
    </>
  );
}
