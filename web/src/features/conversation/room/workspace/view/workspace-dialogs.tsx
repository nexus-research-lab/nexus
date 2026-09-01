import {
  ConfirmDialog,
  PromptDialog,
} from "@/shared/ui/dialog/decision/decision-dialog";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  DesktopFileApplicationsResult,
  DesktopWorkspaceFileOpenTarget,
} from "@/lib/desktop-bridge/desktop-bridge";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

import type {
  WorkspaceContextMenuState,
  WorkspacePromptState,
} from "../controller/interaction/workspace-interaction-model";
import { WorkspaceContextMenu } from "./workspace-context-menu";

interface WorkspaceDialogsController {
  closeContextMenu: () => void;
  closeDeletePrompt: () => void;
  closePrompt: () => void;
  contextMenu: WorkspaceContextMenuState;
  deleteTarget: WorkspaceFileEntry | null;
  isMutating: boolean;
  promptState: WorkspacePromptState;
  handleUploadClick: (directoryPath?: string | null) => void;
  openCreatePrompt: (entryType: "file" | "directory", parentPath?: string | null) => void;
  openRenamePrompt: (entry: WorkspaceFileEntry) => void;
  handlePromptConfirm: (value: string) => Promise<void>;
  handleConfirmDelete: () => Promise<void>;
  handleAddContextEntryToChat: () => Promise<void>;
  handleCopyContextEntryPath: () => Promise<void>;
  handleDownloadContextEntry: () => Promise<void>;
  handleOpenContextEntry: (
    target: DesktopWorkspaceFileOpenTarget,
    applicationPath?: string,
  ) => Promise<void>;
  openApplications: {
    isLoading: boolean;
    path: string;
    result: DesktopFileApplicationsResult | null;
  } | null;
  openDeletePrompt: (entry: WorkspaceFileEntry) => void;
}

const PROMPT_COPY_BY_MODE = {
  "create-file": {
    title: "room.workspace_create_file_title",
    placeholder: "room.workspace_create_file_placeholder",
  },
  "create-directory": {
    title: "room.workspace_create_folder_title",
    placeholder: "room.workspace_create_folder_placeholder",
  },
  rename: {
    title: "room.workspace_rename_title",
    placeholder: "room.workspace_rename_placeholder",
  },
} as const;

export function WorkspaceDialogs({controller}: {controller: WorkspaceDialogsController}) {
  const {t} = useI18n();
  const contextEntry = controller.contextMenu.entry;
  const contextDirectory = contextEntry?.is_dir ? contextEntry.path : null;
  const openApplications = controller.openApplications?.path === contextEntry?.path
    ? controller.openApplications
    : null;
  const promptCopy = PROMPT_COPY_BY_MODE[controller.promptState?.mode ?? "rename"];

  return (
    <>
      <WorkspaceContextMenu
        canCreateChildren={contextEntry === null || contextEntry.is_dir}
        entry={contextEntry}
        isLoadingOpenApplications={openApplications?.isLoading ?? false}
        onAddToChat={() => void controller.handleAddContextEntryToChat()}
        onClose={controller.closeContextMenu}
        onCopyPath={() => void controller.handleCopyContextEntryPath()}
        onCreateFile={() => controller.openCreatePrompt("file", contextDirectory)}
        onCreateFolder={() => controller.openCreatePrompt("directory", contextDirectory)}
        onDelete={() => contextEntry && controller.openDeletePrompt(contextEntry)}
        onDownload={() => void controller.handleDownloadContextEntry()}
        onOpen={(target, applicationPath) => (
          void controller.handleOpenContextEntry(target, applicationPath)
        )}
        onRename={() => contextEntry && controller.openRenamePrompt(contextEntry)}
        onUpload={() => controller.handleUploadClick(contextDirectory)}
        openApplications={openApplications?.result ?? null}
        position={controller.contextMenu.position}
      />

      <PromptDialog
        defaultValue={controller.promptState?.defaultValue ?? ""}
        isOpen={controller.promptState !== null}
        onCancel={controller.closePrompt}
        onConfirm={controller.handlePromptConfirm}
        placeholder={t(promptCopy.placeholder)}
        title={t(promptCopy.title)}
      />

      <ConfirmDialog
        busy={controller.isMutating}
        cancelText={t("common.cancel")}
        confirmText={t("common.delete")}
        isOpen={controller.deleteTarget !== null}
        message={t("room.workspace_delete_message", {name: controller.deleteTarget?.name ?? ""})}
        onCancel={controller.closeDeletePrompt}
        onConfirm={controller.handleConfirmDelete}
        title={t("room.workspace_delete_title")}
        variant="danger"
      />
    </>
  );
}
