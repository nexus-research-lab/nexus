// INPUT: 已确定的文件目录、读取/变更状态、页面命令与外部分栏尺寸。
// OUTPUT: 共享文件树、具名状态和关联目录的分隔条；目录动作仍连接原控制器。
// POS: Workspace 目录面组合，导航/变更语义和宽度状态归调用方。

import { useId, type MouseEvent } from "react";
import { FilePlus, FolderPlus, FolderTree, LoaderCircle, Upload } from "lucide-react";

import { WorkspaceFileToolbarButton } from "@/features/conversation/shared/editor/workspace-file-preview-chrome";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { PanelResizeHandle, type PanelResizeControl } from "@/shared/ui/layout/panel-resize-handle";
import { WORKSPACE_PANEL_HEADER_ICON_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";
import { SidebarEmptyGuide } from "@/shared/ui/sidebar/sidebar-empty-guide";
import { WorkspaceLoadingState } from "@/shared/ui/workspace/frame/workspace-loading-state";
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
  resizeControl: PanelResizeControl | null;
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
          <LoaderCircle className={getUiSpinnerClassName({ size: "sm" })} />
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
      <div className="flex h-full min-h-0">
        <WorkspaceLoadingState label={t("common.loading")} />
      </div>
    );
  }
  return (
    <SidebarEmptyGuide
      description={t("room.workspace_empty_description")}
      icon={FolderTree}
      title={t("room.no_files")}
    />
  );
}

export function WorkspaceFileBrowser({
  activePath,
  controller,
  onResizeStart,
  resizeControl,
  stacked = false,
  width,
}: WorkspaceFileBrowserProps) {
  const {t} = useI18n();
  const panelId = useId();
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
          control={resizeControl}
          controls={panelId}
          onResizeStart={onResizeStart}
        />
      ) : null}

      <div
        id={panelId}
        className="min-h-0 flex-1 overflow-hidden"
        onContextMenu={controller.handleRootContextMenu}
      >
        <WorkspaceFileList activePath={activePath} controller={controller} />
      </div>
    </div>
  );
}
