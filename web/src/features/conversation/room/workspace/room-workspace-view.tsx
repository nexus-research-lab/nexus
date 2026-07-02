"use client";

import { ReactNode, useEffect, useRef, useState } from "react";
import { FilePlus, FolderOpen, FolderPlus, FolderTree, LoaderCircle, Upload, } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import { Agent } from "@/types/agent/agent";
import { download_workspace_file_api } from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { ConfirmDialog, PromptDialog } from "@/shared/ui/dialog/confirm-dialog";
import { EditorPanel } from "@/features/conversation/shared/editor/editor-panel";
import { ConversationResizeHandle } from "@/features/conversation/shared/editor/conversation-resize-handle";
import { useRoomWorkspaceController, } from "./use-room-workspace-controller";
import { RoomAgentSwitcher } from "@/features/conversation/room/surface/room-agent-switcher";
import { WorkspaceContextMenu } from "./workspace-context-menu";
import { WorkspaceFileTree } from "./workspace-file-tree";
import { useMediaQuery } from "@/hooks/ui/use-media-query";

interface RoomWorkspaceViewProps {
  active_workspace_path: string | null;
  agent_id: string;
  header_action?: ReactNode;
  is_dm: boolean;
  is_editor_open: boolean;
  room_members: Agent[];
  on_open_workspace_file: (path: string | null) => void;
}

const WORKSPACE_FILE_LIST_DEFAULT_WIDTH = 280;
const WORKSPACE_FILE_LIST_MIN_WIDTH = 200;
const WORKSPACE_FILE_LIST_MAX_WIDTH = 360;
const COMPACT_WORKSPACE_FILE_LIST_DEFAULT_WIDTH = 220;
const COMPACT_WORKSPACE_FILE_LIST_MIN_WIDTH = 160;
const COMPACT_WORKSPACE_FILE_LIST_MAX_WIDTH = 280;

// ── main view ──────────────────────────────────────────────────────────────

export function RoomWorkspaceView(
  {
    active_workspace_path: activeWorkspacePath,
    agent_id: agentId,
    header_action: headerAction,
    is_dm: isDm,
    is_editor_open: isEditorOpen,
    room_members: roomMembers,
    on_open_workspace_file: onOpenWorkspaceFile,
  }: RoomWorkspaceViewProps) {
  const {t} = useI18n();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const workspacePanelRef = useRef<HTMLDivElement>(null);
  const isCompactFileTree = useMediaQuery("(max-width: 1280px)");
  const [fileListWidth, setFileListWidth] = useState(WORKSPACE_FILE_LIST_DEFAULT_WIDTH);
  const [isResizingFileList, setIsResizingFileList] = useState(false);
  const [isPreviewFocused, setIsPreviewFocused] = useResettableState(
    false,
    activeWorkspacePath ? "has-path" : "no-path",
  );
  const fileListMinWidth = isCompactFileTree
    ? COMPACT_WORKSPACE_FILE_LIST_MIN_WIDTH
    : WORKSPACE_FILE_LIST_MIN_WIDTH;
  const fileListMaxWidth = isCompactFileTree
    ? COMPACT_WORKSPACE_FILE_LIST_MAX_WIDTH
    : WORKSPACE_FILE_LIST_MAX_WIDTH;
  const {
    view_agent_id: viewAgentId,
    files,
    selected_agent_id: selectedAgentId,
    set_selected_agent_id: setSelectedAgentId,
    is_uploading: isUploading,
    is_loading_files: isLoadingFiles,
    error_message: errorMessage,
    clear_error_message: clearErrorMessage,
    context_menu: contextMenu,
    prompt_state: promptState,
    delete_target: deleteTarget,
    focused_directory_path: focusedDirectoryPath,
    current_directory_label: currentDirectoryLabel,
    handle_click_file: handleClickFile,
    handle_click_directory: handleClickDirectory,
    handle_upload_click: handleUploadClick,
    handle_file_select: handleFileSelect,
    open_create_prompt: openCreatePrompt,
    open_rename_prompt: openRenamePrompt,
    handle_prompt_confirm: handlePromptConfirm,
    handle_confirm_delete: handleConfirmDelete,
    handle_context_menu: handleContextMenu,
    handle_root_context_menu: handleRootContextMenu,
    close_context_menu: closeContextMenu,
    set_delete_target: setDeleteTarget,
    set_prompt_state: setPromptState,
  } = useRoomWorkspaceController({
    active_workspace_path: activeWorkspacePath,
    agent_id: agentId,
    is_dm: isDm,
    on_open_workspace_file: onOpenWorkspaceFile,
    file_input_ref: fileInputRef,
  });

  const titleTrailing = !isDm && roomMembers.length > 1 ? (
    <RoomAgentSwitcher
      members={roomMembers}
      selected_id={selectedAgentId}
      on_select={setSelectedAgentId}
    />
  ) : null;

  const handleExternalContextEntry = () => {
    if (!contextMenu.entry || contextMenu.entry.is_dir) {
      return;
    }
    void download_workspace_file_api(
      viewAgentId,
      contextMenu.entry.path,
      contextMenu.entry.name,
    ).catch((error) => {
      console.error("[RoomWorkspaceView] 处理 workspace 文件失败:", error);
    });
  };

  const handleTogglePreviewFocus = () => {
    setIsPreviewFocused((value) => !value);
    setIsResizingFileList(false);
  };

  useEffect(() => {
    if (isCompactFileTree) {
      setFileListWidth((current) => Math.min(current, COMPACT_WORKSPACE_FILE_LIST_DEFAULT_WIDTH));
      return;
    }
    setFileListWidth((current) => Math.max(current, WORKSPACE_FILE_LIST_DEFAULT_WIDTH));
  }, [isCompactFileTree]);

  useEffect(() => {
    if (!isResizingFileList) {
      return;
    }

    const handleMouseMove = (event: MouseEvent) => {
      const bounds = workspacePanelRef.current?.getBoundingClientRect();
      if (!bounds) {
        return;
      }

      const nextWidth = bounds.right - event.clientX;
      setFileListWidth(
        Math.min(
          Math.max(nextWidth, fileListMinWidth),
          fileListMaxWidth,
        ),
      );
    };

    const handleMouseUp = () => {
      setIsResizingFileList(false);
    };

    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);

    return () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };
  }, [fileListMaxWidth, fileListMinWidth, isResizingFileList]);

  return (
    <>
      <input
        aria-label="上传工作区文件"
        ref={fileInputRef}
        type="file"
        className="hidden"
        multiple
        onChange={handleFileSelect}
      />

      <WorkspaceSurfaceView
        action={headerAction}
        body_class_name="px-2 pt-1 pb-0 sm:px-2 xl:px-4"
        body_scrollable={false}
        content_class_name="flex h-full min-h-0 min-w-0 gap-4"
        eyebrow={t("room.workspace")}
        max_width_class_name="max-w-none"
        show_eyebrow={false}
        title={t("room.workspace_title")}
        title_trailing={titleTrailing}
      >
        <div
          ref={workspacePanelRef}
          className={cn("flex h-full min-h-0 min-w-0 flex-1", isResizingFileList && "cursor-col-resize select-none")}
        >
          <div className="min-h-0 min-w-0 flex-1 overflow-hidden">
            <EditorPanel
              agent_id={viewAgentId}
              class_name="h-full w-full"
              embedded
              is_open={isEditorOpen}
              is_preview_focused={isPreviewFocused}
              on_resize_start={() => {
              }}
              on_toggle_preview_focus={activeWorkspacePath ? handleTogglePreviewFocus : undefined}
              path={activeWorkspacePath}
              width_percent={100}
            />
          </div>

          {!isPreviewFocused ? (
            <div
              className="relative flex min-h-0 shrink-0 flex-col border-l divider-subtle pl-4"
              style={{width: `${fileListWidth}px`}}
            >
              <ConversationResizeHandle
                aria_label="调整文件列表宽度"
                on_mouse_down={() => setIsResizingFileList(true)}
              />

              <div
                className="mb-2 inline-flex min-w-0 items-center gap-1.5 rounded-[7px] border border-(--divider-subtle-color) px-2.5 py-1 text-[11px] text-(--text-default)">
                {focusedDirectoryPath ? (
                  <FolderOpen className="h-3 w-3 shrink-0 text-[var(--accent)]"/>
                ) : (
                  <FolderTree className="h-3 w-3 shrink-0 text-(--icon-muted)"/>
                )}
                <span className="truncate font-medium text-(--text-strong)">{currentDirectoryLabel}</span>
              </div>

              <div
                className="soft-scrollbar flex items-center gap-1 overflow-x-auto whitespace-nowrap pb-1 max-xl:gap-2">
                <div className="shrink-0">
                  <WorkspaceSurfaceToolbarAction onClick={() => handleUploadClick()}
                                                 disabled={isUploading}
                                                 tone="primary"
                                                 aria_label={t(isUploading ? "room.workspace_uploading" : "room.workspace_action_upload")}
                                                 class_name="max-xl:h-7 max-xl:w-7 max-xl:justify-center max-xl:gap-0"
                                                 title={t(isUploading ? "room.workspace_uploading" : "room.workspace_action_upload")}>
                    {isUploading ? (
                      <LoaderCircle className="h-3 w-3 animate-spin"/>
                    ) : (
                      <Upload className="h-3 w-3"/>
                    )}
                    <span className="max-xl:hidden">
                      {t(isUploading ? "room.workspace_uploading" : "room.workspace_action_upload")}
                    </span>
                  </WorkspaceSurfaceToolbarAction>
                </div>

                <div className="shrink-0">
                  <WorkspaceSurfaceToolbarAction onClick={() => openCreatePrompt("directory")}
                                                 aria_label={t("room.workspace_action_new_folder")}
                                                 class_name="max-xl:h-7 max-xl:w-7 max-xl:justify-center max-xl:gap-0"
                                                 title={t("room.workspace_action_new_folder")}>
                    <FolderPlus className="h-3 w-3"/>
                    <span className="max-xl:hidden">{t("room.workspace_action_new_folder")}</span>
                  </WorkspaceSurfaceToolbarAction>
                </div>

                <div className="shrink-0">
                  <WorkspaceSurfaceToolbarAction onClick={() => openCreatePrompt("file")}
                                                 aria_label={t("room.workspace_action_new_file")}
                                                 class_name="max-xl:h-7 max-xl:w-7 max-xl:justify-center max-xl:gap-0"
                                                 title={t("room.workspace_action_new_file")}>
                    <FilePlus className="h-3 w-3"/>
                    <span className="max-xl:hidden">{t("room.workspace_action_new_file")}</span>
                  </WorkspaceSurfaceToolbarAction>
                </div>
              </div>

              {errorMessage ? (
                <div
                  className="mb-4 flex items-center justify-between rounded-2xl border border-destructive/20 bg-destructive/6 px-4 py-3 text-sm text-destructive">
                  <span className="min-w-0 flex-1 truncate">{errorMessage}</span>
                  <button
                    type="button"
                    className="ml-3 shrink-0 rounded-md px-2 py-1 text-xs font-medium transition hover:bg-destructive/10"
                    onClick={clearErrorMessage}
                  >
                    {t("common.close")}
                  </button>
                </div>
              ) : null}

              <div className="min-h-0 flex-1 overflow-hidden" onContextMenu={handleRootContextMenu}>
                {files.length > 0 ? (
                  <div className="soft-scrollbar h-full overflow-auto py-1">
                    <WorkspaceFileTree
                      entries={files}
                      active_path={activeWorkspacePath}
                      focused_directory_path={focusedDirectoryPath}
                      on_click_file={handleClickFile}
                      on_click_directory={handleClickDirectory}
                      on_rename_entry={openRenamePrompt}
                      on_delete_entry={setDeleteTarget}
                      on_context_menu={handleContextMenu}
                    />
                  </div>
                ) : isLoadingFiles ? (
                  <div className="flex h-full items-center justify-center text-(--text-soft)">
                    <LoaderCircle className="h-4 w-4 animate-spin"/>
                  </div>
                ) : (
                  <div
                    className="rounded-[12px] border border-(--divider-subtle-color) px-6 py-10 text-center">
                    <div
                      className="mx-auto flex h-11 w-11 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-default) shadow-(--surface-avatar-shadow)">
                      <FolderTree className="h-4 w-4"/>
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
          ) : null}
        </div>
      </WorkspaceSurfaceView>

      {/* 上下文菜单 */}
      <WorkspaceContextMenu
        position={contextMenu.position}
        entry={contextMenu.entry}
        can_create_children={contextMenu.entry === null || contextMenu.entry.is_dir}
        on_upload={() => handleUploadClick(contextMenu.entry?.is_dir ? contextMenu.entry.path : null)}
        on_create_file={() => openCreatePrompt("file", contextMenu.entry?.is_dir ? contextMenu.entry.path : null)}
        on_create_folder={() => openCreatePrompt("directory", contextMenu.entry?.is_dir ? contextMenu.entry.path : null)}
        on_download={handleExternalContextEntry}
        on_rename={() => {
          if (contextMenu.entry) openRenamePrompt(contextMenu.entry);
        }}
        on_delete={() => {
          if (contextMenu.entry) setDeleteTarget(contextMenu.entry);
        }}
        on_close={closeContextMenu}
      />

      <PromptDialog
        is_open={promptState !== null}
        title={
          promptState?.mode === "create-file"
            ? t("room.workspace_create_file_title")
            : promptState?.mode === "create-directory"
              ? t("room.workspace_create_folder_title")
              : t("room.workspace_rename_title")
        }
        placeholder={
          promptState?.mode === "create-file"
            ? t("room.workspace_create_file_placeholder")
            : promptState?.mode === "create-directory"
              ? t("room.workspace_create_folder_placeholder")
              : t("room.workspace_rename_placeholder")
        }
        default_value={promptState?.default_value ?? ""}
        on_confirm={handlePromptConfirm}
        on_cancel={() => setPromptState(null)}
      />

      <ConfirmDialog
        is_open={deleteTarget !== null}
        title={t("room.workspace_delete_title")}
        message={t("room.workspace_delete_message", {name: deleteTarget?.name ?? ""})}
        confirm_text={t("common.delete")}
        cancel_text={t("common.cancel")}
        on_confirm={handleConfirmDelete}
        on_cancel={() => setDeleteTarget(null)}
        variant="danger"
      />
    </>
  );
}
