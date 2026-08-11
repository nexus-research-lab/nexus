import { useCallback, useRef, useState } from "react";

import {
  createWorkspaceEntryApi,
  deleteWorkspaceEntryApi,
  downloadWorkspaceFileApi,
  loadWorkspaceFileApi,
  renameWorkspaceEntryApi,
  uploadWorkspaceFileApi,
} from "@/lib/api/agent/agent-api";
import {
  getDesktopWorkspaceFileApplications,
  openDesktopWorkspaceFile,
  type DesktopFileApplicationsResult,
  type DesktopWorkspaceFileOpenTarget,
} from "@/lib/desktop-bridge/desktop-bridge";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import type {
  WorkspaceEntryMutationResponse,
  WorkspaceEntryRenameResponse,
  WorkspaceFileEntry,
} from "@/types/agent/agent";
import {
  appendLocalAttachments,
  buildLocalAttachmentBatch,
} from "@/features/conversation/shared/composer/attachments/composer-local-attachment-model";
import { useComposerDraftStore } from "@/features/conversation/shared/composer/composer-draft-store";

import {
  getParentWorkspacePath,
  joinLocalWorkspacePath,
  joinWorkspacePath,
} from "./workspace-path-model";

type WorkspaceCommand =
  | "upload"
  | "create"
  | "rename"
  | "delete"
  | "download"
  | "open"
  | "copy-path"
  | "add-to-chat";

interface WorkspaceCommandState {
  scopeKey: string;
  activeCommand: WorkspaceCommand | null;
  errorMessage: string | null;
}

interface WorkspaceCommandToken {
  scopeKey: string;
  commandId: number;
}

interface UseWorkspaceCommandsOptions {
  agentId: string;
  composerDraftScopeKey: string | null;
  refreshFiles: () => Promise<WorkspaceFileEntry[] | null>;
  workspaceRoot: string;
}

interface WorkspaceOpenApplicationsState {
  isLoading: boolean;
  path: string;
  requestId: number;
  result: DesktopFileApplicationsResult | null;
  scopeKey: string;
}

const COMMAND_ERROR_MESSAGES: Record<WorkspaceCommand, string> = {
  upload: "上传文件失败",
  create: "创建工作区条目失败",
  rename: "重命名失败",
  delete: "删除失败",
  download: "处理文件失败",
  open: "打开文件失败",
  "copy-path": "复制路径失败",
  "add-to-chat": "添加文件到聊天失败",
};

const COMMAND_ERROR_KEYS: Partial<Record<WorkspaceCommand, TranslationKey>> = {
  open: "room.workspace_open_failed",
  "copy-path": "room.workspace_copy_path_failed",
  "add-to-chat": "room.workspace_add_to_chat_failed",
};

const COMMAND_REFRESH_POLICY: Record<WorkspaceCommand, boolean> = {
  upload: true,
  create: true,
  rename: true,
  delete: true,
  download: false,
  open: false,
  "copy-path": false,
  "add-to-chat": false,
};

function getCommandErrorMessage(
  error: unknown,
  command: WorkspaceCommand,
  translate: ReturnType<typeof useI18n>["t"],
): string {
  if (error instanceof Error) {
    return error.message;
  }
  const messageKey = COMMAND_ERROR_KEYS[command];
  return messageKey ? translate(messageKey) : COMMAND_ERROR_MESSAGES[command];
}

export function useWorkspaceCommands({
  agentId,
  composerDraftScopeKey,
  refreshFiles,
  workspaceRoot,
}: UseWorkspaceCommandsOptions) {
  const {t} = useI18n();
  const scopeRef = useRef(agentId);
  const commandSequenceRef = useRef(0);
  const activeTokenRef = useRef<WorkspaceCommandToken | null>(null);
  const [state, setState] = useState<WorkspaceCommandState>({
    scopeKey: agentId,
    activeCommand: null,
    errorMessage: null,
  });
  const applicationRequestRef = useRef(0);
  const [openApplicationsState, setOpenApplicationsState] =
    useState<WorkspaceOpenApplicationsState | null>(null);
  scopeRef.current = agentId;

  const isCurrentToken = useCallback((token: WorkspaceCommandToken): boolean => (
    scopeRef.current === token.scopeKey
      && activeTokenRef.current?.scopeKey === token.scopeKey
      && activeTokenRef.current.commandId === token.commandId
  ), []);

  const runCommand = useCallback(async <Result,>(
    command: WorkspaceCommand,
    mutation: (scopeKey: string) => Promise<Result>,
  ): Promise<Result | null> => {
    if (activeTokenRef.current?.scopeKey === agentId) {
      return null;
    }

    const token = {scopeKey: agentId, commandId: ++commandSequenceRef.current};
    activeTokenRef.current = token;
    setState({scopeKey: agentId, activeCommand: command, errorMessage: null});

    try {
      const result = await mutation(token.scopeKey);
      if (COMMAND_REFRESH_POLICY[command]) {
        await refreshFiles();
      }
      return isCurrentToken(token) ? result : null;
    } catch (error) {
      if (isCurrentToken(token)) {
        setState({
          scopeKey: agentId,
          activeCommand: null,
          errorMessage: getCommandErrorMessage(error, command, t),
        });
      }
      return null;
    } finally {
      if (isCurrentToken(token)) {
        activeTokenRef.current = null;
        setState((current) => ({...current, activeCommand: null}));
      }
    }
  }, [agentId, isCurrentToken, refreshFiles, t]);

  const uploadFiles = useCallback((
    files: File[],
    targetDirectory: string | null,
  ): Promise<true | null> => runCommand("upload", async (scopeKey) => {
    const targetPath = targetDirectory ? `${targetDirectory}/` : undefined;
    for (const file of files) {
      await uploadWorkspaceFileApi(scopeKey, file, targetPath);
    }
    return true as const;
  }), [runCommand]);

  const createEntry = useCallback((
    entryType: "file" | "directory",
    parentPath: string | null,
    name: string,
  ): Promise<WorkspaceEntryMutationResponse | null> => runCommand(
    "create",
    (scopeKey) => createWorkspaceEntryApi(
      scopeKey,
      joinWorkspacePath(parentPath, name),
      entryType,
    ),
  ), [runCommand]);

  const renameEntry = useCallback((
    entry: WorkspaceFileEntry,
    name: string,
  ): Promise<WorkspaceEntryRenameResponse | null> => runCommand(
    "rename",
    (scopeKey) => renameWorkspaceEntryApi(
      scopeKey,
      entry.path,
      joinWorkspacePath(getParentWorkspacePath(entry.path), name),
    ),
  ), [runCommand]);

  const deleteEntry = useCallback((
    entry: WorkspaceFileEntry,
  ): Promise<WorkspaceEntryMutationResponse | null> => runCommand(
    "delete",
    (scopeKey) => deleteWorkspaceEntryApi(scopeKey, entry.path),
  ), [runCommand]);

  const downloadEntry = useCallback((
    entry: WorkspaceFileEntry,
  ): Promise<true | null> => runCommand("download", async (scopeKey) => {
    await downloadWorkspaceFileApi(scopeKey, entry.path, entry.name);
    return true as const;
  }), [runCommand]);

  const openEntry = useCallback((
    entry: WorkspaceFileEntry,
    target: DesktopWorkspaceFileOpenTarget,
    applicationPath?: string,
  ): Promise<true | null> => runCommand("open", async () => {
    if (!workspaceRoot.trim()) {
      throw new Error(t("room.workspace_open_failed"));
    }
    await openDesktopWorkspaceFile(
      joinLocalWorkspacePath(workspaceRoot, entry.path),
      target,
      applicationPath,
    );
    return true as const;
  }), [runCommand, t, workspaceRoot]);

  const copyEntryPath = useCallback((
    entry: WorkspaceFileEntry,
  ): Promise<true | null> => runCommand("copy-path", async () => {
    if (!workspaceRoot.trim()) {
      throw new Error(t("room.workspace_copy_path_failed"));
    }
    await navigator.clipboard.writeText(
      joinLocalWorkspacePath(workspaceRoot, entry.path),
    );
    return true as const;
  }), [runCommand, t, workspaceRoot]);

  const loadOpenApplications = useCallback(async (
    entry: WorkspaceFileEntry,
  ): Promise<void> => {
    if (!workspaceRoot.trim()) {
      return;
    }
    const requestId = ++applicationRequestRef.current;
    const localPath = joinLocalWorkspacePath(workspaceRoot, entry.path);
    setOpenApplicationsState({
      isLoading: true,
      path: entry.path,
      requestId,
      result: null,
      scopeKey: agentId,
    });
    try {
      const result = await getDesktopWorkspaceFileApplications(localPath);
      if (
        scopeRef.current === agentId
        && applicationRequestRef.current === requestId
      ) {
        setOpenApplicationsState({
          isLoading: false,
          path: entry.path,
          requestId,
          result,
          scopeKey: agentId,
        });
      }
    } catch {
      if (
        scopeRef.current === agentId
        && applicationRequestRef.current === requestId
      ) {
        setOpenApplicationsState({
          isLoading: false,
          path: entry.path,
          requestId,
          result: null,
          scopeKey: agentId,
        });
      }
    }
  }, [agentId, workspaceRoot]);

  const addEntryToChat = useCallback((
    entry: WorkspaceFileEntry,
  ): Promise<true | null> => runCommand("add-to-chat", async (scopeKey) => {
    const draftScopeKey = composerDraftScopeKey?.trim();
    if (!draftScopeKey) {
      throw new Error(t("room.workspace_chat_unavailable"));
    }
    const file = await loadWorkspaceFileApi(scopeKey, entry.path, entry.name);
    const batch = buildLocalAttachmentBatch([file]);
    const rejection = batch.rejections[0];
    if (rejection) {
      const messageKey = rejection.code === "too_large"
        ? "composer.attachment_too_large"
        : "composer.attachment_format_unsupported";
      throw new Error(t(messageKey, {name: rejection.fileName}));
    }
    const addition = batch.attachments[0];
    if (!addition) {
      throw new Error(t("room.workspace_add_to_chat_failed"));
    }

    const outcome: {value: "added" | "full" | "goal"} = {value: "full"};
    useComposerDraftStore.getState().update_composer_draft(
      draftScopeKey,
      (current) => {
        if (current.inputMode === "goal") {
          outcome.value = "goal";
          return current;
        }
        const attachments = appendLocalAttachments(
          current.attachments,
          [addition],
        );
        if (!attachments.some((item) => item.id === addition.id)) {
          return current;
        }
        outcome.value = "added";
        return {...current, attachments};
      },
    );
    if (outcome.value === "goal") {
      throw new Error(t("composer.goal_attachment_unsupported"));
    }
    if (outcome.value === "full") {
      throw new Error(t("room.workspace_attachment_limit_reached"));
    }
    return true as const;
  }), [composerDraftScopeKey, runCommand, t]);

  const clearError = useCallback(() => {
    setState((current) => (
      current.scopeKey === agentId ? {...current, errorMessage: null} : current
    ));
  }, [agentId]);

  const currentState = state.scopeKey === agentId
    ? state
    : {scopeKey: agentId, activeCommand: null, errorMessage: null};
  const currentOpenApplications = openApplicationsState?.scopeKey === agentId
    ? openApplicationsState
    : null;

  return {
    activeCommand: currentState.activeCommand,
    errorMessage: currentState.errorMessage,
    uploadFiles,
    createEntry,
    renameEntry,
    deleteEntry,
    downloadEntry,
    openEntry,
    copyEntryPath,
    loadOpenApplications,
    openApplications: currentOpenApplications,
    addEntryToChat,
    clearError,
  };
}
