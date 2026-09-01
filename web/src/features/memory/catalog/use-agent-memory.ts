/**
 * INPUT: exact owner generation + Agent 的 Memory 目录、用户选择与已确认删除意图。
 * OUTPUT: stale-preserving 目录读取、path-scoped 删除结果锁、只读对账和显式新意图入口。
 * POS: Memory Catalog 控制器；删除提交与目录刷新分离，普通刷新不得解除未知写入。
 */
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useSyncExternalStore,
} from "react";

import {
  deleteAgentMemoryDocumentApi,
  getAgentMemorySnapshotApi,
} from "@/lib/api/agent/memory-api";
import { getWorkspaceFileContentApi } from "@/lib/api/agent/agent-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import {
  getResourceFailure,
  projectMutationFailure,
  type ResourceFailure,
} from "@/lib/error-message";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import type { MemorySnapshot } from "@/types/memory/memory";

import {
  type ScopedMemoryScope,
  type ScopedMemoryScopeRef,
  useScopedMemoryState,
} from "../use-scoped-memory-state";
import {
  canStartNewMemoryDeletionIntent,
  type MemoryDeletionIssue,
  projectCommittedMemoryDeletion,
  projectMemoryDeletionFailure,
  removeMemoryDeletionIssue,
  upsertMemoryDeletionIssue,
} from "./memory-deletion-recovery";
import {
  type MemoryFilter,
  projectMemoryCatalog,
  resolveSelectedMemoryPath,
} from "./memory-catalog-model";

type MemoryDeleteIntent = "new" | "normal" | "retry";
type MemoryDeletionAction = "delete" | "reconcile" | null;

interface AgentMemoryState {
  compactDocumentOpen: boolean;
  deleteAction: MemoryDeletionAction;
  deleteIntent: MemoryDeleteIntent;
  deleteIssues: MemoryDeletionIssue[];
  deleteTargetPath: string;
  deletingPath: string;
  error: ResourceFailure | null;
  filter: MemoryFilter;
  isLoading: boolean;
  query: string;
  selectedPath: string;
  snapshot: MemorySnapshot | null;
  scopeKey: string;
}

interface AgentMemoryScope extends ScopedMemoryScope {
  agentId: string;
  ownerGeneration: number;
}

interface MemoryDeletionCommandToken extends AgentMemoryScope {
  id: number;
  path: string;
  title: string;
}

export function useAgentMemory(
  agentId: string,
  fallbackError: string,
  fallbackDeleteError: string,
) {
  const ownerGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  const scopeKey = `${ownerGeneration}\u0000${agentId}`;
  const directorySequenceRef = useRef(0);
  const commandSequenceRef = useRef(0);
  const activeCommandRef = useRef<MemoryDeletionCommandToken | null>(null);
  const { commit, scopeRef, state } = useScopedMemoryState(
    { agentId, key: scopeKey, ownerGeneration },
    createAgentMemoryState,
  );

  const refresh = useCallback(async () => {
    const expectedScope = scopeRef.current;
    const requestSequence = directorySequenceRef.current + 1;
    directorySequenceRef.current = requestSequence;
    commit(expectedScope.key, (current) => ({
      ...current,
      error: current.error?.access ? current.error : null,
      isLoading: true,
    }));
    try {
      const snapshot = await getAgentMemorySnapshotApi(expectedScope.agentId);
      if (!isCurrentDirectoryRequest(
        scopeRef,
        expectedScope,
        directorySequenceRef,
        requestSequence,
      )) {
        return;
      }
      commit(expectedScope.key, (current) => ({
        ...current,
        error: null,
        isLoading: false,
        selectedPath: resolveSelectedMemoryPath(snapshot, current.selectedPath),
        snapshot,
        // 普通/后台刷新只更新目录；它没有用户发起的 exact 对账意图，
        // 因此不能解除任何 path-scoped outcome_unknown。
      }));
    } catch (error) {
      if (!isCurrentDirectoryRequest(
        scopeRef,
        expectedScope,
        directorySequenceRef,
        requestSequence,
      )) {
        return;
      }
      commit(expectedScope.key, (current) => ({
        ...current,
        error: getResourceFailure(error, fallbackError),
        isLoading: false,
      }));
    }
  }, [commit, fallbackError, scopeRef]);

  useEffect(() => {
    activeCommandRef.current = null;
    commandSequenceRef.current += 1;
    directorySequenceRef.current += 1;
    void refresh();
    return () => {
      activeCommandRef.current = null;
      commandSequenceRef.current += 1;
      directorySequenceRef.current += 1;
    };
  }, [refresh, scopeKey]);

  const projection = useMemo(
    () => projectMemoryCatalog(
      state.snapshot,
      state.selectedPath,
      state.filter,
      state.query,
    ),
    [state.filter, state.query, state.selectedPath, state.snapshot],
  );

  const deleteTarget = useMemo(() => (
    projection.allDocuments.find((document) => (
      document.path === state.deleteTargetPath && document.kind !== "index"
    )) ?? null
  ), [projection.allDocuments, state.deleteTargetPath]);

  const selectDocument = useCallback((path: string) => {
    if (!projection.allDocuments.some((document) => document.path === path)) {
      return;
    }
    commit(scopeKey, (current) => ({
      ...current,
      compactDocumentOpen: true,
      deleteTargetPath: "",
      selectedPath: path,
    }));
  }, [commit, projection.allDocuments, scopeKey]);

  const openDeleteConfirmation = useCallback((
    path: string,
    intent: MemoryDeleteIntent,
  ) => {
    if (activeCommandRef.current || state.deletingPath) {
      return;
    }
    const target = projection.allDocuments.find((document) => (
      document.path === path && document.kind !== "index"
    ));
    if (!target) {
      return;
    }
    const issue = findExactDeletionIssue(
      state.deleteIssues,
      ownerGeneration,
      agentId,
      path,
    );
    const intentAllowed = intent === "normal"
      ? !issue
      : intent === "retry"
        ? issue?.kind === "not_applied"
        : Boolean(issue && canStartNewMemoryDeletionIntent(issue));
    if (!intentAllowed) {
      return;
    }
    commit(scopeKey, (current) => ({
      ...current,
      deleteIntent: intent,
      deleteTargetPath: path,
    }));
  }, [
    agentId,
    commit,
    ownerGeneration,
    projection.allDocuments,
    scopeKey,
    state.deleteIssues,
    state.deletingPath,
  ]);

  const requestDeleteDocument = useCallback((path: string) => {
    openDeleteConfirmation(path, "normal");
  }, [openDeleteConfirmation]);
  const retryDeleteDocument = useCallback((path: string) => {
    openDeleteConfirmation(path, "retry");
  }, [openDeleteConfirmation]);
  const beginNewDeleteIntent = useCallback((path: string) => {
    openDeleteConfirmation(path, "new");
  }, [openDeleteConfirmation]);
  const cancelDeleteDocument = useCallback(() => {
    commit(scopeKey, (current) => ({
      ...current,
      deleteIntent: "normal",
      deleteTargetPath: "",
    }));
  }, [commit, scopeKey]);

  const reconcileDeletionIssue = useCallback(async (
    token: MemoryDeletionCommandToken,
    issue: MemoryDeletionIssue,
  ) => {
    const requestSequence = directorySequenceRef.current + 1;
    directorySequenceRef.current = requestSequence;
    commit(token.key, (current) => ({
      ...current,
      deleteAction: "reconcile",
      deletingPath: token.path,
      error: current.error?.access ? current.error : null,
      isLoading: true,
    }));
    try {
      const snapshot = await getAgentMemorySnapshotApi(token.agentId);
      if (!isCurrentDeletionCommand(
        scopeRef,
        token,
        activeCommandRef,
      ) || directorySequenceRef.current !== requestSequence) {
        return;
      }
      const targetPresent = await memoryDocumentStillExists(
        snapshot,
        token.agentId,
        token.path,
      );
      if (!isCurrentDeletionCommand(
        scopeRef,
        token,
        activeCommandRef,
      ) || directorySequenceRef.current !== requestSequence) {
        return;
      }
      commit(token.key, (current) => ({
        ...current,
        deleteIssues: targetPresent
          ? upsertMemoryDeletionIssue(current.deleteIssues, {
              ...issue,
              directoryCheck: "target_present",
            })
          : removeMemoryDeletionIssue(current.deleteIssues, token.path),
        error: null,
        isLoading: false,
        selectedPath: resolveSelectedMemoryPath(snapshot, current.selectedPath),
        snapshot,
      }));
    } catch (error) {
      if (!isCurrentDeletionCommand(
        scopeRef,
        token,
        activeCommandRef,
      ) || directorySequenceRef.current !== requestSequence) {
        return;
      }
      commit(token.key, (current) => ({
        ...current,
        deleteIssues: upsertMemoryDeletionIssue(current.deleteIssues, {
          ...issue,
          directoryCheck: "failed",
        }),
        error: getResourceFailure(error, fallbackError),
        isLoading: false,
      }));
    }
  }, [commit, fallbackError, scopeRef]);

  const confirmDeleteDocument = useCallback(async () => {
    if (activeCommandRef.current || state.deletingPath) {
      return;
    }
    const target = projection.allDocuments.find((document) => (
      document.path === state.deleteTargetPath && document.kind !== "index"
    ));
    if (!target) {
      cancelDeleteDocument();
      return;
    }
    const expectedScope = scopeRef.current;
    const existingIssue = findExactDeletionIssue(
      state.deleteIssues,
      expectedScope.ownerGeneration,
      expectedScope.agentId,
      target.path,
    );
    const intentAllowed = state.deleteIntent === "normal"
      ? !existingIssue
      : state.deleteIntent === "retry"
        ? existingIssue?.kind === "not_applied"
        : Boolean(
            existingIssue && canStartNewMemoryDeletionIntent(existingIssue),
          );
    if (!intentAllowed) {
      cancelDeleteDocument();
      return;
    }

    const token: MemoryDeletionCommandToken = {
      ...expectedScope,
      id: commandSequenceRef.current + 1,
      path: target.path,
      title: target.title,
    };
    commandSequenceRef.current = token.id;
    activeCommandRef.current = token;
    commit(token.key, (current) => ({
      ...current,
      deleteAction: "delete",
      deleteIntent: "normal",
      deleteTargetPath: "",
      deletingPath: token.path,
    }));
    try {
      try {
        await deleteAgentMemoryDocumentApi(token.agentId, token.path);
      } catch (error) {
        if (!isCurrentDeletionCommand(scopeRef, token, activeCommandRef)) {
          return;
        }
        const issue = projectMemoryDeletionFailure(
          projectMutationFailure(error, fallbackDeleteError),
          token,
        );
        commit(token.key, (current) => ({
          ...current,
          deleteIssues: upsertMemoryDeletionIssue(current.deleteIssues, issue),
        }));
        if (issue.kind !== "not_applied") {
          await reconcileDeletionIssue(token, issue);
        }
        return;
      }

      if (!isCurrentDeletionCommand(scopeRef, token, activeCommandRef)) {
        return;
      }
      // DELETE 的完整成功响应是提交证据；后续目录读取失败只能重刷目录，
      // 不能把已提交删除降级成普通失败或再次 DELETE。
      const committedIssue = projectCommittedMemoryDeletion(token);
      commit(token.key, (current) => ({
        ...current,
        deleteIssues: upsertMemoryDeletionIssue(
          current.deleteIssues,
          committedIssue,
        ),
      }));
      await reconcileDeletionIssue(token, committedIssue);
    } finally {
      if (activeCommandRef.current?.id === token.id) {
        activeCommandRef.current = null;
        commit(token.key, (current) => ({
          ...current,
          deleteAction: null,
          deletingPath: "",
        }));
      }
    }
  }, [
    cancelDeleteDocument,
    commit,
    fallbackDeleteError,
    projection.allDocuments,
    reconcileDeletionIssue,
    scopeRef,
    state.deleteIntent,
    state.deleteIssues,
    state.deleteTargetPath,
    state.deletingPath,
  ]);

  const reconcileDeleteDocument = useCallback(async (path: string) => {
    if (activeCommandRef.current || state.deletingPath) {
      return;
    }
    const expectedScope = scopeRef.current;
    const issue = findExactDeletionIssue(
      state.deleteIssues,
      expectedScope.ownerGeneration,
      expectedScope.agentId,
      path,
    );
    if (!issue || issue.kind === "not_applied") {
      return;
    }
    const token: MemoryDeletionCommandToken = {
      ...expectedScope,
      id: commandSequenceRef.current + 1,
      path: issue.path,
      title: issue.title,
    };
    commandSequenceRef.current = token.id;
    activeCommandRef.current = token;
    try {
      await reconcileDeletionIssue(token, issue);
    } finally {
      if (activeCommandRef.current?.id === token.id) {
        activeCommandRef.current = null;
        commit(token.key, (current) => ({
          ...current,
          deleteAction: null,
          deletingPath: "",
        }));
      }
    }
  }, [
    commit,
    reconcileDeletionIssue,
    scopeRef,
    state.deleteIssues,
    state.deletingPath,
  ]);

  const closeCompactDocument = useCallback(() => {
    commit(scopeKey, (current) => ({ ...current, compactDocumentOpen: false }));
  }, [commit, scopeKey]);
  const setFilter = useCallback((filter: MemoryFilter) => {
    commit(scopeKey, (current) => ({ ...current, filter }));
  }, [commit, scopeKey]);
  const setQuery = useCallback((query: string) => {
    commit(scopeKey, (current) => ({ ...current, query }));
  }, [commit, scopeKey]);

  return {
    catalog: {
      emptyFilterVisible: projection.emptyFilterVisible,
      emptyMemoryVisible: projection.emptyMemoryVisible,
      filter: state.filter,
      query: state.query,
      sections: projection.sections,
      setFilter,
      setQuery,
      truncated: projection.truncated,
    },
    document: {
      beginNewDeleteIntent,
      cancelDeleteDocument,
      closeCompactDocument,
      compactDocumentOpen: state.compactDocumentOpen,
      confirmDeleteDocument,
      deleteAction: state.deleteAction,
      deleteIntent: state.deleteIntent,
      deleteIssues: state.deleteIssues,
      deleteTarget,
      deletingPath: state.deletingPath,
      reconcileDeleteDocument,
      requestDeleteDocument,
      retryDeleteDocument,
      selectDocument,
      selectedDocument: projection.selectedDocument,
    },
    resource: {
      error: state.error,
      isLoading: state.isLoading,
      refresh,
      snapshot: state.snapshot,
    },
  };
}

function createAgentMemoryState(scopeKey: string): AgentMemoryState {
  return {
    compactDocumentOpen: false,
    deleteAction: null,
    deleteIntent: "normal",
    deleteIssues: [],
    deleteTargetPath: "",
    deletingPath: "",
    error: null,
    filter: "all",
    isLoading: true,
    query: "",
    selectedPath: "",
    snapshot: null,
    scopeKey,
  };
}

function findExactDeletionIssue(
  issues: MemoryDeletionIssue[],
  ownerGeneration: number,
  agentId: string,
  path: string,
): MemoryDeletionIssue | null {
  return issues.find((issue) => (
    issue.ownerGeneration === ownerGeneration
    && issue.agentId === agentId
    && issue.path === path
  )) ?? null;
}

function isCurrentDirectoryRequest(
  currentScope: ScopedMemoryScopeRef<AgentMemoryScope>,
  expectedScope: AgentMemoryScope,
  currentSequence: { current: number },
  expectedSequence: number,
): boolean {
  return currentScope.current.key === expectedScope.key
    && currentSequence.current === expectedSequence
    && isAuthOwnerScopeGenerationCurrent(expectedScope.ownerGeneration);
}

function isCurrentDeletionCommand(
  currentScope: ScopedMemoryScopeRef<AgentMemoryScope>,
  token: MemoryDeletionCommandToken,
  activeCommand: { current: MemoryDeletionCommandToken | null },
): boolean {
  return currentScope.current.key === token.key
    && activeCommand.current?.id === token.id
    && activeCommand.current.path === token.path
    && isAuthOwnerScopeGenerationCurrent(token.ownerGeneration);
}

async function memoryDocumentStillExists(
  snapshot: MemorySnapshot,
  agentId: string,
  path: string,
): Promise<boolean> {
  if (snapshot.documents.some((document) => document.path === path)) {
    return true;
  }
  if (!snapshot.truncated) {
    return false;
  }
  // 截断目录中的“未出现”不是缺失证据；只在这一小概率分支读取 exact path。
  try {
    await getWorkspaceFileContentApi(agentId, path);
    return true;
  } catch (error) {
    if (error instanceof ApiRequestError && error.status === 404) {
      return false;
    }
    throw error;
  }
}
