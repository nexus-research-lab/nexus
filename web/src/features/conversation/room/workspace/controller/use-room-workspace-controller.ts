import { useCallback, type MouseEvent, type RefObject } from "react";

import { isDesktopRuntime } from "@/config/desktop-runtime";
import type { Agent, WorkspaceFileEntry } from "@/types/agent/agent";

import { useWorkspaceAgentScope } from "./use-workspace-agent-scope";
import { useWorkspaceCommands } from "./use-workspace-commands";
import { useWorkspaceFilesResource } from "./use-workspace-files-resource";
import { useWorkspaceEntryTransactions } from "./interaction/use-workspace-entry-transactions";
import { useWorkspaceInteractionState } from "./interaction/use-workspace-interaction-state";
import { useWorkspaceNavigation } from "./interaction/use-workspace-navigation";

interface UseRoomWorkspaceControllerOptions {
  activeWorkspacePath: string | null;
  agentId: string;
  composerDraftScopeKey: string | null;
  fileInputRef: RefObject<HTMLInputElement | null>;
  isDm: boolean;
  onOpenWorkspaceFile: (path: string | null) => void;
  roomMembers: Agent[];
}

export function useRoomWorkspaceController({
  activeWorkspacePath,
  agentId,
  composerDraftScopeKey,
  fileInputRef,
  isDm,
  onOpenWorkspaceFile,
  roomMembers,
}: UseRoomWorkspaceControllerOptions) {
  const agent = useWorkspaceAgentScope({ agentId, isDm, onOpenWorkspaceFile });
  const workspaceRoot = roomMembers.find(
    (member) => member.agent_id === agent.viewAgentId,
  )?.workspace_path ?? "";
  const resource = useWorkspaceFilesResource(agent.viewAgentId);
  const commands = useWorkspaceCommands({
    agentId: agent.viewAgentId,
    composerDraftScopeKey,
    refreshFiles: resource.reload,
    workspaceRoot,
  });
  const navigation = useWorkspaceNavigation({
    activeWorkspacePath,
    onOpenWorkspaceFile,
    scopeKey: agent.viewAgentId,
  });
  const interaction = useWorkspaceInteractionState({
    fileInputRef,
    focusedDirectoryPath: navigation.focusedDirectoryPath,
    scopeKey: agent.viewAgentId,
  });
  const transactions = useWorkspaceEntryTransactions({
    commands,
    interaction: {
      clearDeleteTarget: interaction.clearDeleteTarget,
      clearUploadTarget: interaction.clearUploadTarget,
      closePrompt: interaction.closePrompt,
      contextEntry: interaction.contextMenu.entry,
      deleteTarget: interaction.deleteTarget,
      promptState: interaction.promptState,
      uploadTargetDirectory: interaction.uploadTargetDirectory,
    },
    navigation: {
      applyCreate: navigation.applyCreate,
      applyDelete: navigation.applyDelete,
      applyRename: navigation.applyRename,
    },
  });
  const clearCommandError = commands.clearError;
  const clearResourceError = resource.clearError;
  const clearErrorMessage = useCallback(() => {
    clearResourceError();
    clearCommandError();
  }, [clearCommandError, clearResourceError]);
  const loadOpenApplications = commands.loadOpenApplications;
  const openContextMenu = interaction.openContextMenu;
  const handleContextMenu = useCallback((
    event: MouseEvent,
    entry: WorkspaceFileEntry,
  ) => {
    openContextMenu(event, entry);
    if (isDesktopRuntime() && !entry.is_dir) {
      void loadOpenApplications(entry);
    }
  }, [loadOpenApplications, openContextMenu]);

  return {
    agent: {
      onSelect: agent.selectAgent,
      selectedId: agent.selectedAgentId,
      viewAgentId: agent.viewAgentId,
    },
    browser: {
      clearErrorMessage,
      errorMessage: commands.errorMessage ?? resource.errorMessage,
      files: resource.files,
      focusedDirectoryPath: navigation.focusedDirectoryPath,
      handleClickDirectory: navigation.focusDirectory,
      handleClickFile: navigation.openFile,
      handleContextMenu,
      handleRootContextMenu: interaction.openRootContextMenu,
      handleUploadClick: interaction.openUpload,
      isLoadingFiles: resource.isLoading,
      isUploading: commands.activeCommand === "upload",
      openCreatePrompt: interaction.openCreatePrompt,
      openDeletePrompt: interaction.openDeletePrompt,
      openRenamePrompt: interaction.openRenamePrompt,
    },
    dialogs: {
      closeContextMenu: interaction.closeContextMenu,
      closeDeletePrompt: interaction.clearDeleteTarget,
      closePrompt: interaction.closePrompt,
      contextMenu: interaction.contextMenu,
      deleteTarget: interaction.deleteTarget,
      handleConfirmDelete: transactions.handleConfirmDelete,
      handleAddContextEntryToChat: transactions.handleAddContextEntryToChat,
      handleCopyContextEntryPath: transactions.handleCopyContextEntryPath,
      handleDownloadContextEntry: transactions.handleDownloadContextEntry,
      handleOpenContextEntry: transactions.handleOpenContextEntry,
      openApplications: commands.openApplications,
      handlePromptConfirm: transactions.handlePromptConfirm,
      handleUploadClick: interaction.openUpload,
      openCreatePrompt: interaction.openCreatePrompt,
      openDeletePrompt: interaction.openDeletePrompt,
      openRenamePrompt: interaction.openRenamePrompt,
      promptState: interaction.promptState,
    },
    fileInput: {
      onChange: transactions.handleFileSelect,
    },
  };
}
