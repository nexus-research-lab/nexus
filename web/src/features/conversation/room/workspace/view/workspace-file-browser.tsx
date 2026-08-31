import type { MouseEvent } from "react";
import { FilePlus, FolderPlus, FolderTree, LoaderCircle, Upload } from "lucide-react";

import { WorkspaceFileToolbarButton } from "@/features/conversation/shared/editor/workspace-file-preview-chrome";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { PanelResizeHandle } from "@/shared/ui/layout/panel-resize-handle";
import { WORKSPACE_PANEL_HEADER_ICON_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";
import { WorkspaceFileTree } from "@/shared/ui/workspace/tree/workspace-file-tree";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

interface WorkspaceFileBrowserController {
  files: WorkspaceFileEntry[];
  isLoadingFiles: boolean;
  isMutating: boolean;
  isUploading: boolean;
  focusedDirectoryPath: string | null;
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
  stacked?: boolean;
  width: number;
}

type WorkspaceDirectoryToolbarController = Pick<
  WorkspaceFileBrowserController,
  "handleUploadClick" | "isMutating" | "isUploading" | "openCreatePrompt"
>;

export function WorkspaceDirectoryToolbar({
  controller,
}: {
  controller: WorkspaceDirectoryToolbarController;
}) {
  const {t} = useI18n();
  const uploadKey = controller.isUploading
    ? "room.workspace_uploading"
    : "room.workspace_action_upload";

  return (
    <div className="flex shrink-0 items-center gap-0.5">
      <WorkspaceFileToolbarButton
        disabled={controller.isMutating}
        onClick={() => controller.handleUploadClick()}
        title={t(uploadKey)}
      >
        {controller.isUploading ? (
          <LoaderCircle className={cn(WORKSPACE_PANEL_HEADER_ICON_CLASS, "animate-spin")} />
        ) : (
          <Upload className={WORKSPACE_PANEL_HEADER_ICON_CLASS} />
        )}
      </WorkspaceFileToolbarButton>

      <WorkspaceFileToolbarButton
        disabled={controller.isMutating}
        onClick={() => controller.openCreatePrompt("directory")}
        title={t("room.workspace_action_new_folder")}
      >
        <FolderPlus className={WORKSPACE_PANEL_HEADER_ICON_CLASS} />
      </WorkspaceFileToolbarButton>

      <WorkspaceFileToolbarButton
        disabled={controller.isMutating}
        onClick={() => controller.openCreatePrompt("file")}
        title={t("room.workspace_action_new_file")}
      >
        <FilePlus className={WORKSPACE_PANEL_HEADER_ICON_CLASS} />
      </WorkspaceFileToolbarButton>
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
      <p className="mt-4 text-base font-semibold text-(--text-strong)">{t("room.no_files")}</p>
      <p className="mt-1 text-compact leading-6 text-(--text-soft)">
        {t("room.workspace_empty_description")}
      </p>
    </div>
  );
}

export function WorkspaceFileBrowser({
  activePath,
  controller,
  onResizeStart,
  stacked = false,
  width,
}: WorkspaceFileBrowserProps) {
  const {t} = useI18n();
  return (
    <div
      className={cn(
        "relative flex min-h-0 shrink-0 flex-col border-l divider-subtle pl-4",
        stacked &&
          "h-[42%] min-h-[220px] max-h-[320px] w-full border-l-0 border-b pb-3 pl-0",
      )}
      style={{width: stacked ? "100%" : `${width}px`}}
    >
      {!stacked ? (
        <PanelResizeHandle
          ariaLabel={t("room.resize_workspace_file_list")}
          onResizeStart={onResizeStart}
        />
      ) : null}

      <div
        className="min-h-0 flex-1 overflow-hidden"
        onContextMenu={controller.handleRootContextMenu}
      >
        <WorkspaceFileList activePath={activePath} controller={controller} />
      </div>
    </div>
  );
}
