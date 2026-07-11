import type { MouseEvent } from "react";
import { FilePlus, FolderOpen, FolderPlus, FolderTree, LoaderCircle, Upload } from "lucide-react";

import { ConversationResizeHandle } from "@/features/conversation/shared/editor/conversation-resize-handle";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-toolbar-action";
import { WorkspaceFileTree } from "@/shared/ui/workspace/tree/workspace-file-tree";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

interface WorkspaceFileBrowserController {
  files: WorkspaceFileEntry[];
  isLoadingFiles: boolean;
  isUploading: boolean;
  errorMessage: string | null;
  focusedDirectoryPath: string | null;
  currentDirectoryLabel: string;
  clearErrorMessage: () => void;
  handleClickFile: (path: string) => void;
  handleClickDirectory: (path: string) => void;
  handleUploadClick: (directoryPath?: string | null) => void;
  openCreatePrompt: (entryType: "file" | "directory", parentPath?: string | null) => void;
  openDeletePrompt: (entry: WorkspaceFileEntry) => void;
  openRenamePrompt: (entry: WorkspaceFileEntry) => void;
  handleContextMenu: (event: MouseEvent, entry: WorkspaceFileEntry) => void;
  handleRootContextMenu: (event: MouseEvent) => void;
}

interface WorkspaceFileBrowserProps {
  activePath: string | null;
  controller: WorkspaceFileBrowserController;
  onResizeStart: () => void;
  width: number;
}

function WorkspaceDirectoryToolbar({controller}: {controller: WorkspaceFileBrowserController}) {
  const {t} = useI18n();
  const uploadKey = controller.isUploading
    ? "room.workspace_uploading"
    : "room.workspace_action_upload";

  return (
    <div className="soft-scrollbar flex items-center gap-1 overflow-x-auto whitespace-nowrap pb-1 max-xl:gap-2">
      <WorkspaceSurfaceToolbarAction
        ariaLabel={t(uploadKey)}
        className="max-xl:h-7 max-xl:w-7 max-xl:justify-center max-xl:gap-0"
        disabled={controller.isUploading}
        onClick={() => controller.handleUploadClick()}
        title={t(uploadKey)}
        tone="primary"
      >
        {controller.isUploading ? (
          <LoaderCircle className="h-3 w-3 animate-spin" />
        ) : (
          <Upload className="h-3 w-3" />
        )}
        <span className="max-xl:hidden">{t(uploadKey)}</span>
      </WorkspaceSurfaceToolbarAction>

      <WorkspaceSurfaceToolbarAction
        ariaLabel={t("room.workspace_action_new_folder")}
        className="max-xl:h-7 max-xl:w-7 max-xl:justify-center max-xl:gap-0"
        onClick={() => controller.openCreatePrompt("directory")}
        title={t("room.workspace_action_new_folder")}
      >
        <FolderPlus className="h-3 w-3" />
        <span className="max-xl:hidden">{t("room.workspace_action_new_folder")}</span>
      </WorkspaceSurfaceToolbarAction>

      <WorkspaceSurfaceToolbarAction
        ariaLabel={t("room.workspace_action_new_file")}
        className="max-xl:h-7 max-xl:w-7 max-xl:justify-center max-xl:gap-0"
        onClick={() => controller.openCreatePrompt("file")}
        title={t("room.workspace_action_new_file")}
      >
        <FilePlus className="h-3 w-3" />
        <span className="max-xl:hidden">{t("room.workspace_action_new_file")}</span>
      </WorkspaceSurfaceToolbarAction>
    </div>
  );
}

function WorkspaceFileList({
  activePath,
  controller,
}: Pick<WorkspaceFileBrowserProps, "activePath" | "controller">) {
  const {t} = useI18n();
  if (controller.files.length > 0) {
    return (
      <div className="soft-scrollbar h-full overflow-auto py-1">
        <WorkspaceFileTree
          activePath={activePath}
          entries={controller.files}
          focusedDirectoryPath={controller.focusedDirectoryPath}
          onClickDirectory={controller.handleClickDirectory}
          onClickFile={controller.handleClickFile}
          onContextMenu={controller.handleContextMenu}
          onDeleteEntry={controller.openDeletePrompt}
          onRenameEntry={controller.openRenamePrompt}
        />
      </div>
    );
  }
  if (controller.isLoadingFiles) {
    return (
      <div className="flex h-full items-center justify-center text-(--text-soft)">
        <LoaderCircle className="h-4 w-4 animate-spin" />
      </div>
    );
  }
  return (
    <div className="rounded-[12px] border border-(--divider-subtle-color) px-6 py-10 text-center">
      <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-default) shadow-(--surface-avatar-shadow)">
        <FolderTree className="h-4 w-4" />
      </div>
      <p className="mt-4 text-[15px] font-semibold text-(--text-strong)">{t("room.no_files")}</p>
      <p className="mt-1 text-[12px] leading-6 text-(--text-soft)">
        {t("room.workspace_empty_description")}
      </p>
    </div>
  );
}

export function WorkspaceFileBrowser({
  activePath,
  controller,
  onResizeStart,
  width,
}: WorkspaceFileBrowserProps) {
  const {t} = useI18n();
  return (
    <div
      className="relative flex min-h-0 shrink-0 flex-col border-l divider-subtle pl-4"
      style={{width: `${width}px`}}
    >
      <ConversationResizeHandle ariaLabel="调整文件列表宽度" onMouseDown={onResizeStart} />

      <div className="mb-2 inline-flex min-w-0 items-center gap-1.5 rounded-[7px] border border-(--divider-subtle-color) px-2.5 py-1 text-[11px] text-(--text-default)">
        {controller.focusedDirectoryPath ? (
          <FolderOpen className="h-3 w-3 shrink-0 text-[var(--accent)]" />
        ) : (
          <FolderTree className="h-3 w-3 shrink-0 text-(--icon-muted)" />
        )}
        <span className="truncate font-medium text-(--text-strong)">
          {controller.currentDirectoryLabel}
        </span>
      </div>

      <WorkspaceDirectoryToolbar controller={controller} />

      {controller.errorMessage ? (
        <div className="mb-4 flex items-center justify-between rounded-2xl border border-destructive/20 bg-destructive/6 px-4 py-3 text-sm text-destructive">
          <span className="min-w-0 flex-1 truncate">{controller.errorMessage}</span>
          <button
            type="button"
            className="ml-3 shrink-0 rounded-md px-2 py-1 text-xs font-medium transition hover:bg-destructive/10"
            onClick={controller.clearErrorMessage}
          >
            {t("common.close")}
          </button>
        </div>
      ) : null}

      <div className="min-h-0 flex-1 overflow-hidden" onContextMenu={controller.handleRootContextMenu}>
        <WorkspaceFileList activePath={activePath} controller={controller} />
      </div>
    </div>
  );
}
