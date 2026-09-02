/**
 * INPUT: 当前 Agent、精确 workspace 命令、文件选择与权威列表刷新函数。
 * OUTPUT: 串行命令、精确未知结果锁，以及保留逐文件结果证据的精简恢复反馈。
 * POS: workspace 文件命令控制器；已提交结果与列表刷新分阶段，未知副作用不自动重放。
 */

import { useCallback, useMemo, useRef, useState } from "react";

import {
  createWorkspaceEntryApi,
  deleteWorkspaceEntryApi,
  downloadWorkspaceFileApi,
  loadWorkspaceFileApi,
  renameWorkspaceEntryApi,
  uploadWorkspaceFileApi,
  WorkspaceFileSizeLimitError,
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
import { MAX_COMPOSER_ATTACHMENT_SIZE_BYTES } from "@/features/conversation/shared/composer/attachments/composer-attachments";
import { useComposerDraftStore } from "@/features/conversation/shared/composer/composer-draft-store";
import { projectMutationFailure } from "@/lib/error-message";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";

import {
  getParentWorkspacePath,
  joinLocalWorkspacePath,
  joinWorkspacePath,
} from "./workspace-path-model";
import {
  getWorkspaceMutationIntentKey,
  groupWorkspaceUploadOutcomes,
  reconcileWorkspaceMutation,
  type WorkspaceMutationIntent,
  type WorkspaceReconciledMutation,
  type WorkspaceUploadOutcome,
} from "./workspace-command-recovery";

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
  feedback: WorkspaceCommandFeedback | null;
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

type WorkspaceFeedbackAction = "allow-new-intent" | "refresh";

interface WorkspaceCommandFeedback {
  action: WorkspaceFeedbackAction | null;
  impact: string;
  nextStep: string;
  recoveryKey: string | null;
  title: string;
  tone: FeedbackBannerProps["tone"];
}

interface PendingWorkspaceRecovery {
  canStartNewIntent: boolean;
  intent: WorkspaceMutationIntent;
  listChecked: boolean;
  uploadOutcomes: WorkspaceUploadOutcome[] | null;
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

function asUploadFileIdentity(file: File) {
  return {
    lastModified: file.lastModified,
    name: file.name,
    size: file.size,
    type: file.type,
  };
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
  const recoveryLocksRef = useRef(new Map<string, PendingWorkspaceRecovery>());
  const recoveryRequestSequenceRef = useRef(0);
  const [state, setState] = useState<WorkspaceCommandState>({
    scopeKey: agentId,
    activeCommand: null,
    feedback: null,
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

  const mutationActionLabel = useCallback((
    intent: WorkspaceMutationIntent,
  ): string => {
    switch (intent.command) {
      case "create":
        return t(intent.entryType === "directory"
          ? "room.workspace_create_folder_title"
          : "room.workspace_create_file_title");
      case "rename":
        return t("room.workspace_rename_title");
      case "delete":
        return t("room.workspace_delete_title");
      case "upload":
        return t("room.workspace_action_upload");
    }
  }, [t]);

  const formatUploadNames = useCallback((names: string[]): string => {
    if (names.length === 0) {
      return t("room.workspace_upload_status_none");
    }
    const visibleNames = names.slice(0, 5).join(", ");
    return names.length > 5
      ? t("room.workspace_upload_status_more", {
          count: names.length - 5,
          names: visibleNames,
        })
      : visibleNames;
  }, [t]);

  const uploadOutcomeSummary = useCallback((
    outcomes: WorkspaceUploadOutcome[],
  ): string => {
    const grouped = groupWorkspaceUploadOutcomes(outcomes);
    return t("room.workspace_upload_status_summary", {
      completed: formatUploadNames(grouped.completed),
      notApplied: formatUploadNames(grouped.not_applied),
      notStarted: formatUploadNames(grouped.not_started),
      unconfirmed: formatUploadNames(grouped.unconfirmed),
    });
  }, [formatUploadNames, t]);

  const unknownFeedback = useCallback((
    recoveryKey: string,
    recovery: PendingWorkspaceRecovery,
  ): WorkspaceCommandFeedback => {
    const actionLabel = mutationActionLabel(recovery.intent);
    return {
      action: recovery.listChecked && recovery.canStartNewIntent
        ? "allow-new-intent"
        : "refresh",
      impact: recovery.uploadOutcomes
        ? t("room.workspace_upload_unknown_impact", {
            summary: uploadOutcomeSummary(recovery.uploadOutcomes),
          })
        : t("room.workspace_mutation_unknown_impact", {action: actionLabel}),
      nextStep: recovery.listChecked
        ? t(recovery.canStartNewIntent
            ? "room.workspace_mutation_checked_next"
            : "room.workspace_mutation_wait_next")
        : t("room.workspace_mutation_unknown_next"),
      recoveryKey,
      title: t("room.workspace_mutation_unknown_title"),
      tone: "warning",
    };
  }, [mutationActionLabel, t, uploadOutcomeSummary]);

  const safeRefreshFiles = useCallback(async (): Promise<WorkspaceFileEntry[] | null> => {
    try {
      return await refreshFiles();
    } catch {
      return null;
    }
  }, [refreshFiles]);

  const committedRefreshFeedback = useCallback((): WorkspaceCommandFeedback => ({
    action: "refresh",
    impact: t("room.workspace_committed_refresh_impact"),
    nextStep: t("room.workspace_committed_refresh_next"),
    recoveryKey: null,
    title: t("room.workspace_committed_refresh_title"),
    tone: "warning",
  }), [t]);

  const refreshAfterCommittedMutation = useCallback(async (
    token: WorkspaceCommandToken,
  ): Promise<boolean> => {
    const files = await safeRefreshFiles();
    if (files === null && isCurrentToken(token)) {
      setState((current) => ({
        ...current,
        feedback: committedRefreshFeedback(),
      }));
    }
    return files !== null;
  }, [committedRefreshFeedback, isCurrentToken, safeRefreshFiles]);

  const beginCommand = useCallback((
    command: WorkspaceCommand,
  ): WorkspaceCommandToken | null => {
    if (activeTokenRef.current?.scopeKey === agentId) {
      return null;
    }
    const token = {scopeKey: agentId, commandId: ++commandSequenceRef.current};
    activeTokenRef.current = token;
    setState({scopeKey: agentId, activeCommand: command, feedback: null});
    return token;
  }, [agentId]);

  const finishCommand = useCallback((token: WorkspaceCommandToken): void => {
    if (!isCurrentToken(token)) {
      return;
    }
    activeTokenRef.current = null;
    setState((current) => ({...current, activeCommand: null}));
  }, [isCurrentToken]);

  const runCommand = useCallback(async <Result,>(
    command: WorkspaceCommand,
    mutation: (scopeKey: string) => Promise<Result>,
  ): Promise<Result | null> => {
    const token = beginCommand(command);
    if (!token) {
      return null;
    }

    try {
      const result = await mutation(token.scopeKey);
      return isCurrentToken(token) ? result : null;
    } catch {
      if (isCurrentToken(token)) {
        setState({
          scopeKey: token.scopeKey,
          activeCommand: null,
          feedback: {
            action: null,
            impact: t("room.workspace_read_action_failed_impact"),
            nextStep: t("room.workspace_read_action_failed_next"),
            recoveryKey: null,
            title: COMMAND_ERROR_KEYS[command]
              ? t(COMMAND_ERROR_KEYS[command] as TranslationKey)
              : COMMAND_ERROR_MESSAGES[command],
            tone: "error",
          },
        });
      }
      return null;
    } finally {
      finishCommand(token);
    }
  }, [beginCommand, finishCommand, isCurrentToken, t]);

  const recoverMutation = useCallback(async (
    token: WorkspaceCommandToken,
    recoveryKey: string,
    recovery: PendingWorkspaceRecovery,
  ): Promise<WorkspaceReconciledMutation | null> => {
    recoveryLocksRef.current.set(recoveryKey, recovery);
    const files = await safeRefreshFiles();
    if (!isCurrentToken(token)) {
      return null;
    }
    if (files !== null && recovery.intent.command !== "upload") {
      const reconciled = reconcileWorkspaceMutation(recovery.intent, files);
      if (reconciled) {
        recoveryLocksRef.current.delete(recoveryKey);
        setState((current) => ({
          ...current,
          feedback: {
            action: null,
            impact: t("room.workspace_reconciled_impact"),
            nextStep: t("room.workspace_reconciled_next"),
            recoveryKey: null,
            title: t("room.workspace_reconciled_title"),
            tone: "success",
          },
        }));
        return reconciled;
      }
    }
    const checkedRecovery = {...recovery, listChecked: files !== null};
    recoveryLocksRef.current.set(recoveryKey, checkedRecovery);
    setState((current) => ({
      ...current,
      feedback: unknownFeedback(recoveryKey, checkedRecovery),
    }));
    return null;
  }, [
    isCurrentToken,
    safeRefreshFiles,
    t,
    unknownFeedback,
  ]);

  const runMutationCommand = useCallback(async <Result,>(
    command: Extract<WorkspaceCommand, "create" | "delete" | "rename">,
    intent: WorkspaceMutationIntent,
    mutation: (scopeKey: string) => Promise<Result>,
  ): Promise<Result | null> => {
    const recoveryKey = getWorkspaceMutationIntentKey(intent);
    const existingRecovery = recoveryLocksRef.current.get(recoveryKey);
    if (existingRecovery) {
      setState({
        scopeKey: agentId,
        activeCommand: null,
        feedback: unknownFeedback(recoveryKey, existingRecovery),
      });
      return null;
    }
    const token = beginCommand(command);
    if (!token) {
      return null;
    }
    try {
      const result = await mutation(token.scopeKey);
      await refreshAfterCommittedMutation(token);
      return isCurrentToken(token) ? result : null;
    } catch (error) {
      if (!isCurrentToken(token)) {
        return null;
      }
      const failure = projectMutationFailure(
        error,
        COMMAND_ERROR_MESSAGES[command],
      );
      if (failure.effect === "not_applied") {
        setState((current) => ({
          ...current,
          feedback: {
            action: null,
            impact: t("room.workspace_mutation_not_applied_impact"),
            nextStep: t("room.workspace_mutation_not_applied_next"),
            recoveryKey: null,
            title: t("room.workspace_mutation_not_applied_title", {
              action: mutationActionLabel(intent),
            }),
            tone: "error",
          },
        }));
        return null;
      }
      const reconciled = await recoverMutation(token, recoveryKey, {
        canStartNewIntent: failure.effect === "unknown",
        intent,
        listChecked: false,
        uploadOutcomes: null,
      });
      return reconciled ? reconciled.result as Result : null;
    } finally {
      finishCommand(token);
    }
  }, [
    agentId,
    beginCommand,
    finishCommand,
    isCurrentToken,
    mutationActionLabel,
    recoverMutation,
    refreshAfterCommittedMutation,
    t,
    unknownFeedback,
  ]);

  const uploadFiles = useCallback(async (
    files: File[],
    targetDirectory: string | null,
  ): Promise<true | null> => {
    const token = beginCommand("upload");
    if (!token) {
      return null;
    }
    const targetPath = targetDirectory ? `${targetDirectory}/` : undefined;
    const outcomes: WorkspaceUploadOutcome[] = files.map((file) => ({
      name: file.name,
      status: "not_started",
    }));
    let committedWithoutResponse = false;
    try {
      for (const [index, file] of files.entries()) {
        if (!isCurrentToken(token)) {
          return null;
        }
        const intent: WorkspaceMutationIntent = {
          agentId: token.scopeKey,
          command: "upload",
          file: asUploadFileIdentity(file),
          targetDirectory,
        };
        const recoveryKey = getWorkspaceMutationIntentKey(intent);
        const existingRecovery = recoveryLocksRef.current.get(recoveryKey);
        if (existingRecovery) {
          outcomes[index] = {name: file.name, status: "unconfirmed"};
          const pending = {...existingRecovery, uploadOutcomes: outcomes};
          recoveryLocksRef.current.set(recoveryKey, pending);
          setState((current) => ({
            ...current,
            feedback: unknownFeedback(recoveryKey, pending),
          }));
          return null;
        }
        try {
          await uploadWorkspaceFileApi(token.scopeKey, file, targetPath);
          outcomes[index] = {name: file.name, status: "completed"};
        } catch (error) {
          if (!isCurrentToken(token)) {
            return null;
          }
          const failure = projectMutationFailure(error, COMMAND_ERROR_MESSAGES.upload);
          if (failure.effect === "committed") {
            outcomes[index] = {name: file.name, status: "completed"};
            committedWithoutResponse = true;
            continue;
          }
          if (failure.effect === "not_applied") {
            outcomes[index] = {name: file.name, status: "not_applied"};
            if (outcomes.some((outcome) => outcome.status === "completed")) {
              await safeRefreshFiles();
            }
            if (isCurrentToken(token)) {
              setState((current) => ({
                ...current,
                feedback: {
                  action: null,
                  impact: t("room.workspace_upload_partial_impact", {
                    summary: uploadOutcomeSummary(outcomes),
                  }),
                  nextStep: t("room.workspace_upload_not_applied_next"),
                  recoveryKey: null,
                  title: t("room.workspace_upload_partial_title"),
                  tone: "error",
                },
              }));
            }
            return null;
          }
          outcomes[index] = {name: file.name, status: "unconfirmed"};
          const recovery: PendingWorkspaceRecovery = {
            canStartNewIntent: failure.effect === "unknown",
            intent,
            listChecked: false,
            uploadOutcomes: outcomes,
          };
          await recoverMutation(token, recoveryKey, recovery);
          return null;
        }
      }
      const refreshed = await refreshAfterCommittedMutation(token);
      if (committedWithoutResponse && refreshed && isCurrentToken(token)) {
        setState((current) => ({
          ...current,
          feedback: {
            action: null,
            impact: t("room.workspace_upload_committed_impact", {
              names: formatUploadNames(
                outcomes
                  .filter((outcome) => outcome.status === "completed")
                  .map((outcome) => outcome.name),
              ),
            }),
            nextStep: t("room.workspace_upload_committed_next"),
            recoveryKey: null,
            title: t("room.workspace_upload_committed_title"),
            tone: "success",
          },
        }));
      }
      return isCurrentToken(token) ? true : null;
    } finally {
      finishCommand(token);
    }
  }, [
    beginCommand,
    finishCommand,
    formatUploadNames,
    isCurrentToken,
    recoverMutation,
    refreshAfterCommittedMutation,
    safeRefreshFiles,
    t,
    unknownFeedback,
    uploadOutcomeSummary,
  ]);

  const createEntry = useCallback((
    entryType: "file" | "directory",
    parentPath: string | null,
    name: string,
  ): Promise<WorkspaceEntryMutationResponse | null> => {
    const path = joinWorkspacePath(parentPath, name);
    const intent: WorkspaceMutationIntent = {
      agentId,
      command: "create",
      entryType,
      path,
    };
    return runMutationCommand(
      "create",
      intent,
      (scopeKey) => createWorkspaceEntryApi(scopeKey, path, entryType),
    );
  }, [agentId, runMutationCommand]);

  const renameEntry = useCallback((
    entry: WorkspaceFileEntry,
    name: string,
  ): Promise<WorkspaceEntryRenameResponse | null> => {
    const newPath = joinWorkspacePath(getParentWorkspacePath(entry.path), name);
    const intent: WorkspaceMutationIntent = {
      agentId,
      command: "rename",
      isDirectory: entry.is_dir,
      newPath,
      path: entry.path,
    };
    return runMutationCommand(
      "rename",
      intent,
      (scopeKey) => renameWorkspaceEntryApi(scopeKey, entry.path, newPath),
    );
  }, [agentId, runMutationCommand]);

  const deleteEntry = useCallback((
    entry: WorkspaceFileEntry,
  ): Promise<WorkspaceEntryMutationResponse | null> => runMutationCommand(
    "delete",
    {agentId, command: "delete", path: entry.path},
    (scopeKey) => deleteWorkspaceEntryApi(scopeKey, entry.path),
  ), [agentId, runMutationCommand]);

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
    if ((entry.size ?? 0) > MAX_COMPOSER_ATTACHMENT_SIZE_BYTES) {
      throw new Error(t("composer.attachment_too_large", {name: entry.name}));
    }
    let file: File;
    try {
      file = await loadWorkspaceFileApi(
        scopeKey,
        entry.path,
        entry.name,
        MAX_COMPOSER_ATTACHMENT_SIZE_BYTES,
      );
    } catch (error) {
      if (error instanceof WorkspaceFileSizeLimitError) {
        throw new Error(t("composer.attachment_too_large", {name: entry.name}));
      }
      throw error;
    }
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

  const clearFeedback = useCallback(() => {
    setState((current) => (
      current.scopeKey === agentId ? {...current, feedback: null} : current
    ));
  }, [agentId]);

  const reconcilePendingRecovery = useCallback(async (
    recoveryKey: string,
  ): Promise<void> => {
    const recovery = recoveryLocksRef.current.get(recoveryKey);
    if (
      !recovery
      || recovery.intent.agentId !== agentId
      || !recovery.canStartNewIntent
    ) {
      return;
    }
    const requestId = ++recoveryRequestSequenceRef.current;
    const files = await safeRefreshFiles();
    if (
      scopeRef.current !== recovery.intent.agentId
      || recoveryRequestSequenceRef.current !== requestId
      || recoveryLocksRef.current.get(recoveryKey) !== recovery
    ) {
      return;
    }
    if (files !== null && recovery.intent.command !== "upload") {
      const reconciled = reconcileWorkspaceMutation(recovery.intent, files);
      if (reconciled) {
        recoveryLocksRef.current.delete(recoveryKey);
        setState((current) => ({
          ...current,
          feedback: {
            action: null,
            impact: t("room.workspace_reconciled_impact"),
            nextStep: t("room.workspace_reconciled_finish_next"),
            recoveryKey: null,
            title: t("room.workspace_reconciled_title"),
            tone: "success",
          },
        }));
        return;
      }
    }
    const checkedRecovery = {...recovery, listChecked: files !== null};
    recoveryLocksRef.current.set(recoveryKey, checkedRecovery);
    setState((current) => ({
      ...current,
      feedback: unknownFeedback(recoveryKey, checkedRecovery),
    }));
  }, [
    agentId,
    safeRefreshFiles,
    t,
    unknownFeedback,
  ]);

  const allowNewIntent = useCallback((recoveryKey: string): void => {
    const recovery = recoveryLocksRef.current.get(recoveryKey);
    if (!recovery || recovery.intent.agentId !== agentId) {
      return;
    }
    recoveryLocksRef.current.delete(recoveryKey);
    setState((current) => ({
      ...current,
      feedback: {
        action: null,
        impact: t("room.workspace_new_intent_impact"),
        nextStep: t("room.workspace_new_intent_next"),
        recoveryKey: null,
        title: t("room.workspace_new_intent_title"),
        tone: "info",
      },
    }));
  }, [agentId, t]);

  const refreshCommittedList = useCallback(async (): Promise<void> => {
    const requestId = ++recoveryRequestSequenceRef.current;
    const scopeKey = agentId;
    const files = await safeRefreshFiles();
    if (
      files !== null
      && scopeRef.current === scopeKey
      && recoveryRequestSequenceRef.current === requestId
    ) {
      setState((current) => (
        current.scopeKey === scopeKey ? {...current, feedback: null} : current
      ));
    }
  }, [agentId, safeRefreshFiles]);

  const currentState = state.scopeKey === agentId
    ? state
    : {scopeKey: agentId, activeCommand: null, feedback: null};
  const currentOpenApplications = openApplicationsState?.scopeKey === agentId
    ? openApplicationsState
    : null;
  const feedback = useMemo<FeedbackBannerProps | null>(() => {
    const item = currentState.feedback;
    if (!item) {
      return null;
    }
    const onClick = item.action === "allow-new-intent" && item.recoveryKey
      ? () => allowNewIntent(item.recoveryKey as string)
      : item.action === "refresh" && item.recoveryKey
        ? () => void reconcilePendingRecovery(item.recoveryKey as string)
        : item.action === "refresh"
          ? () => void refreshCommittedList()
          : null;
    if (onClick && (item.tone === "error" || item.tone === "warning")) {
      return {
        action: {
          label: item.action === "allow-new-intent"
            ? t("room.workspace_allow_new_intent_action")
            : t("room.workspace_refresh_action"),
          onClick,
        },
        impact: item.impact,
        onDismiss: clearFeedback,
        title: item.title,
        tone: item.tone,
      };
    }
    return {
      impact: item.impact,
      nextStep: item.nextStep,
      onDismiss: clearFeedback,
      title: item.title,
      tone: item.tone,
    };
  }, [
    allowNewIntent,
    clearFeedback,
    currentState.feedback,
    reconcilePendingRecovery,
    refreshCommittedList,
    t,
  ]);

  return {
    activeCommand: currentState.activeCommand,
    feedback,
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
    clearFeedback,
  };
}
