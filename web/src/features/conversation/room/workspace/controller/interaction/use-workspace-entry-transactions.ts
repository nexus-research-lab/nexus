import { useCallback, type ChangeEvent } from "react";

import type {
  WorkspaceEntryMutationResponse,
  WorkspaceEntryRenameResponse,
  WorkspaceFileEntry,
} from "@/types/agent/agent";

import type {
  WorkspaceCreateMode,
  WorkspacePromptState,
} from "./workspace-interaction-model";

type WorkspaceRenamePrompt = Extract<
  Exclude<WorkspacePromptState, null>,
  { mode: "rename" }
>;

interface WorkspaceEntryCommands {
  createEntry: (
    entryType: "file" | "directory",
    parentPath: string | null,
    name: string,
  ) => Promise<WorkspaceEntryMutationResponse | null>;
  deleteEntry: (
    entry: WorkspaceFileEntry,
  ) => Promise<WorkspaceEntryMutationResponse | null>;
  downloadEntry: (entry: WorkspaceFileEntry) => Promise<true | null>;
  renameEntry: (
    entry: WorkspaceFileEntry,
    name: string,
  ) => Promise<WorkspaceEntryRenameResponse | null>;
  uploadFiles: (
    files: File[],
    targetDirectory: string | null,
  ) => Promise<true | null>;
}

interface WorkspaceEntryInteraction {
  clearDeleteTarget: () => void;
  clearUploadTarget: () => void;
  closePrompt: () => void;
  contextEntry: WorkspaceFileEntry | null;
  deleteTarget: WorkspaceFileEntry | null;
  promptState: WorkspacePromptState;
  uploadTargetDirectory: string | null;
}

interface WorkspaceEntryNavigation {
  applyCreate: (entryType: "file" | "directory", path: string) => void;
  applyDelete: (deletedPath: string) => void;
  applyRename: (previousPath: string, nextPath: string) => void;
}

interface UseWorkspaceEntryTransactionsOptions {
  commands: WorkspaceEntryCommands;
  interaction: WorkspaceEntryInteraction;
  navigation: WorkspaceEntryNavigation;
}

const ENTRY_TYPE_BY_CREATE_MODE: Record<
  WorkspaceCreateMode,
  "file" | "directory"
> = {
  "create-directory": "directory",
  "create-file": "file",
};

export function useWorkspaceEntryTransactions({
  commands,
  interaction,
  navigation,
}: UseWorkspaceEntryTransactionsOptions) {
  const {
    createEntry,
    deleteEntry,
    downloadEntry,
    renameEntry,
    uploadFiles,
  } = commands;
  const {
    clearDeleteTarget,
    clearUploadTarget,
    closePrompt,
    contextEntry,
    deleteTarget,
    promptState,
    uploadTargetDirectory,
  } = interaction;
  const { applyCreate, applyDelete, applyRename } = navigation;
  const confirmRename = useCallback(async (
    prompt: WorkspaceRenamePrompt,
    name: string,
  ): Promise<void> => {
    if (name === prompt.entry.name) {
      closePrompt();
      return;
    }
    const result = await renameEntry(prompt.entry, name);
    if (!result) {
      return;
    }
    applyRename(prompt.entry.path, result.new_path);
    closePrompt();
  }, [applyRename, closePrompt, renameEntry]);
  const handlePromptConfirm = useCallback(async (value: string): Promise<void> => {
    const prompt = promptState;
    const name = value.trim();
    if (!prompt || !name) {
      return;
    }
    if (prompt.mode === "rename") {
      await confirmRename(prompt, name);
      return;
    }
    const entryType = ENTRY_TYPE_BY_CREATE_MODE[prompt.mode];
    const result = await createEntry(entryType, prompt.parentPath, name);
    if (!result) {
      return;
    }
    applyCreate(entryType, result.path);
    closePrompt();
  }, [applyCreate, closePrompt, confirmRename, createEntry, promptState]);
  const handleConfirmDelete = useCallback(async (): Promise<void> => {
    const target = deleteTarget;
    if (!target) {
      return;
    }
    const result = await deleteEntry(target);
    if (!result) {
      return;
    }
    applyDelete(target.path);
    clearDeleteTarget();
  }, [applyDelete, clearDeleteTarget, deleteEntry, deleteTarget]);
  const handleDownloadContextEntry = useCallback(async (): Promise<void> => {
    if (contextEntry && !contextEntry.is_dir) {
      await downloadEntry(contextEntry);
    }
  }, [contextEntry, downloadEntry]);
  const handleFileSelect = useCallback(async (
    event: ChangeEvent<HTMLInputElement>,
  ): Promise<void> => {
    const files = Array.from(event.currentTarget.files ?? []);
    const targetDirectory = uploadTargetDirectory;
    event.currentTarget.value = "";
    clearUploadTarget();
    if (files.length > 0) {
      await uploadFiles(files, targetDirectory);
    }
  }, [clearUploadTarget, uploadFiles, uploadTargetDirectory]);

  return {
    handleConfirmDelete,
    handleDownloadContextEntry,
    handleFileSelect,
    handlePromptConfirm,
  };
}
