"use client";

import { useEffect, useRef, useState } from "react";
import { FilePlus, FolderOpen, FolderPlus, FolderTree, LoaderCircle, Upload, } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import { Agent } from "@/types/agent/agent";
import { downloadWorkspaceFileApi } from "@/lib/api/agent-manage-api";
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
  activeWorkspacePath: string | null;
  agentId: string;
  isDm: boolean;
  isEditorOpen: boolean;
  roomMembers: Agent[];
  onOpenWorkspaceFile: (path: string | null) => void;
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
    activeWorkspacePath: activeWorkspacePath,
    agentId: agentId,
    isDm: isDm,
    isEditorOpen: isEditorOpen,
    roomMembers: roomMembers,
    onOpenWorkspaceFile: onOpenWorkspaceFile,
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
    viewAgentId,
    files,
    selectedAgentId,
    setSelectedAgentId,
    isUploading,
    isLoadingFiles,
    errorMessage,
    clearErrorMessage,
    contextMenu,
    promptState,
    deleteTarget,
    focusedDirectoryPath,
    currentDirectoryLabel,
    handleClickFile,
    handleClickDirectory,
    handleUploadClick,
    handleFileSelect,
    openCreatePrompt,
    openRenamePrompt,
    handlePromptConfirm,
    handleConfirmDelete,
    handleContextMenu,
    handleRootContextMenu,
    closeContextMenu,
    setDeleteTarget,
    setPromptState,
  } = useRoomWorkspaceController({
    activeWorkspacePath,
    agentId,
    isDm,
    onOpenWorkspaceFile,
    fileInputRef,
  });

  const titleTrailing = !isDm && roomMembers.length > 1 ? (
    <RoomAgentSwitcher
      members={roomMembers}
      selectedId={selectedAgentId}
      onSelect={setSelectedAgentId}
    />
  ) : null;

  const handleExternalContextEntry = () => {
    if (!contextMenu.entry || contextMenu.entry.is_dir) {
      return;
    }
    void downloadWorkspaceFileApi(
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
        bodyClassName="px-2 pt-1 pb-0 sm:px-2 xl:px-4"
        bodyScrollable={false}
        contentClassName="flex h-full min-h-0 min-w-0 gap-4"
        eyebrow={t("room.workspace")}
        maxWidthClassName="max-w-none"
        showEyebrow={false}
        showTitle={false}
        title={t("room.workspace_title")}
        titleTrailing={titleTrailing}
      >
        <div
          ref={workspacePanelRef}
          className={cn("flex h-full min-h-0 min-w-0 flex-1", isResizingFileList && "cursor-col-resize select-none")}
        >
          <div className="min-h-0 min-w-0 flex-1 overflow-hidden">
            <EditorPanel
              agentId={viewAgentId}
              className="h-full w-full"
              embedded
              isOpen={isEditorOpen}
              isPreviewFocused={isPreviewFocused}
              onResizeStart={() => {
              }}
              onTogglePreviewFocus={activeWorkspacePath ? handleTogglePreviewFocus : undefined}
              path={activeWorkspacePath}
              widthPercent={100}
            />
          </div>

          {!isPreviewFocused ? (
            <div
              className="relative flex min-h-0 shrink-0 flex-col border-l divider-subtle pl-4"
              style={{width: `${fileListWidth}px`}}
            >
              <ConversationResizeHandle
                ariaLabel="调整文件列表宽度"
                onMouseDown={() => setIsResizingFileList(true)}
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
                                                 ariaLabel={t(isUploading ? "room.workspace_uploading" : "room.workspace_action_upload")}
                                                 className="max-xl:h-7 max-xl:w-7 max-xl:justify-center max-xl:gap-0"
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
                                                 ariaLabel={t("room.workspace_action_new_folder")}
                                                 className="max-xl:h-7 max-xl:w-7 max-xl:justify-center max-xl:gap-0"
                                                 title={t("room.workspace_action_new_folder")}>
                    <FolderPlus className="h-3 w-3"/>
                    <span className="max-xl:hidden">{t("room.workspace_action_new_folder")}</span>
                  </WorkspaceSurfaceToolbarAction>
                </div>

                <div className="shrink-0">
                  <WorkspaceSurfaceToolbarAction onClick={() => openCreatePrompt("file")}
                                                 ariaLabel={t("room.workspace_action_new_file")}
                                                 className="max-xl:h-7 max-xl:w-7 max-xl:justify-center max-xl:gap-0"
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
                      activePath={activeWorkspacePath}
                      focusedDirectoryPath={focusedDirectoryPath}
                      onClickFile={handleClickFile}
                      onClickDirectory={handleClickDirectory}
                      onRenameEntry={openRenamePrompt}
                      onDeleteEntry={setDeleteTarget}
                      onContextMenu={handleContextMenu}
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
        canCreateChildren={contextMenu.entry === null || contextMenu.entry.is_dir}
        onUpload={() => handleUploadClick(contextMenu.entry?.is_dir ? contextMenu.entry.path : null)}
        onCreateFile={() => openCreatePrompt("file", contextMenu.entry?.is_dir ? contextMenu.entry.path : null)}
        onCreateFolder={() => openCreatePrompt("directory", contextMenu.entry?.is_dir ? contextMenu.entry.path : null)}
        onDownload={handleExternalContextEntry}
        onRename={() => {
          if (contextMenu.entry) openRenamePrompt(contextMenu.entry);
        }}
        onDelete={() => {
          if (contextMenu.entry) setDeleteTarget(contextMenu.entry);
        }}
        onClose={closeContextMenu}
      />

      <PromptDialog
        isOpen={promptState !== null}
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
        defaultValue={promptState?.defaultValue ?? ""}
        onConfirm={handlePromptConfirm}
        onCancel={() => setPromptState(null)}
      />

      <ConfirmDialog
        isOpen={deleteTarget !== null}
        title={t("room.workspace_delete_title")}
        message={t("room.workspace_delete_message", {name: deleteTarget?.name ?? ""})}
        confirmText={t("common.delete")}
        cancelText={t("common.cancel")}
        onConfirm={handleConfirmDelete}
        onCancel={() => setDeleteTarget(null)}
        variant="danger"
      />
    </>
  );
}
