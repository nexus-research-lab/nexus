/**
 * INPUT: 当前会话的 Agent、workspace 选择与文件面板交互。
 * OUTPUT: 文件资源、命令、弹窗与全局三段式反馈的统一控制器。
 * POS: Room/DM workspace 组合边界；资源刷新失败不覆盖已确认的修改结果。
 */

import { useCallback, useMemo, type MouseEvent, type RefObject } from "react";

import { isDesktopRuntime } from "@/config/desktop-runtime";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";
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
  const {t} = useI18n();
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
  const clearCommandFeedback = commands.clearFeedback;
  const clearResourceError = resource.clearError;
  const clearFeedback = useCallback(() => {
    clearResourceError();
    clearCommandFeedback();
  }, [clearCommandFeedback, clearResourceError]);
  const reloadFiles = resource.reload;
  const feedback = useMemo<FeedbackBannerProps | null>(() => {
    if (commands.feedback) {
      return {...commands.feedback, onDismiss: clearFeedback};
    }
    if (!resource.errorMessage) {
      return null;
    }
    return {
      action: {
        label: t("room.workspace_refresh_action"),
        onClick: () => void reloadFiles(),
      },
      impact: t("room.workspace_list_failed_impact"),
      message: resource.errorMessage,
      nextStep: t("room.workspace_list_failed_next"),
      onDismiss: clearFeedback,
      title: t("room.workspace_list_failed_title"),
      tone: "error",
    };
  }, [
    clearFeedback,
    commands.feedback,
    reloadFiles,
    resource.errorMessage,
    t,
  ]);
  const loadOpenApplications = commands.loadOpenApplications;
  const isMutating = commands.activeCommand === "upload"
    || commands.activeCommand === "create"
    || commands.activeCommand === "rename"
    || commands.activeCommand === "delete";
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
      files: resource.files,
      focusedDirectoryPath: navigation.focusedDirectoryPath,
      handleClickDirectory: navigation.focusDirectory,
      handleClickFile: navigation.openFile,
      handleContextMenu,
      handleRootContextMenu: interaction.openRootContextMenu,
      handleUploadClick: interaction.openUpload,
      isLoadingFiles: resource.isLoading,
      isMutating,
      isUploading: commands.activeCommand === "upload",
      openCreatePrompt: interaction.openCreatePrompt,
      openDeletePrompt: interaction.openDeletePrompt,
      openRenamePrompt: interaction.openRenamePrompt,
    },
    dialogs: {
      closeContextMenu: interaction.closeContextMenu,
      closeDeletePrompt: interaction.clearDeleteTarget,
      closePrompt: interaction.closePrompt,
      isMutating,
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
    feedback,
  };
}
