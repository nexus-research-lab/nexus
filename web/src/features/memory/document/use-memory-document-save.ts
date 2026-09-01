// INPUT: exact Memory scope、用户草稿、读取 revision 与修改结果事实。
// OUTPUT: 条件保存、未知结果对账和用户明确选择后的冲突覆盖。
// POS: Memory 保存可靠性边界；不自动合并、重放或改写 Agent/Session 身份。
import { useCallback, useRef } from "react";

import {
  getWorkspaceFileContentApi,
  updateWorkspaceFileContentApi,
} from "@/lib/api/agent/agent-api";
import { projectMutationFailure } from "@/lib/error-message";

import {
  classifyMemorySaveReconciliation,
  mergeSavedMemoryDocument,
  type MemoryDocumentCommit,
  type MemoryDocumentScope,
  type MemoryDocumentScopeRef,
  type MemoryDocumentState,
} from "./use-memory-document-state";

interface SaveToken {
  agentId: string;
  draft: string;
  expectedRevision: string;
  id: number;
  path: string;
  scopeKey: string;
}

type SaveMode = "normal" | "overwrite_conflict";

interface UseMemoryDocumentSaveOptions {
  commit: MemoryDocumentCommit;
  fallbackSaveError: string;
  onSaved: () => void;
  runtimeWriting: boolean;
  scopeRef: MemoryDocumentScopeRef;
  state: MemoryDocumentState;
}

function createSaveToken({
  activeToken,
  mode,
  nextId,
  runtimeWriting,
  scope,
  state,
}: {
  activeToken: SaveToken | null;
  mode: SaveMode;
  nextId: number;
  runtimeWriting: boolean;
  scope: MemoryDocumentScope;
  state: MemoryDocumentState;
}): SaveToken | null {
  if (
    !scope.document
    || scope.key !== state.scopeKey
    || state.draft === state.content
    || !state.revision
    || runtimeWriting
    || activeToken?.scopeKey === scope.key
    || (mode === "normal" && Boolean(
      state.saveIssue && state.saveIssue.kind !== "not_applied",
    ))
    || (mode === "overwrite_conflict" && !(
      state.saveIssue?.kind === "conflict"
      && state.saveIssue.phase === "review"
    ))
  ) {
    return null;
  }
  return {
    agentId: scope.agentId,
    draft: state.draft,
    expectedRevision: state.revision,
    id: nextId,
    path: scope.document.path,
    scopeKey: scope.key,
  };
}

export function useMemoryDocumentSave({
  commit,
  fallbackSaveError,
  onSaved,
  runtimeWriting,
  scopeRef,
  state,
}: UseMemoryDocumentSaveOptions) {
  const saveSequenceRef = useRef(0);
  const saveTokenRef = useRef<SaveToken | null>(null);
  const reconcileSequenceRef = useRef(0);
  const reconcileRunningRef = useRef(false);

  const executeSave = useCallback(async (mode: SaveMode) => {
    const scope = scopeRef.current;
    const token = createSaveToken({
      activeToken: saveTokenRef.current,
      mode,
      nextId: saveSequenceRef.current + 1,
      runtimeWriting,
      scope,
      state,
    });
    if (!token) {
      return;
    }
    saveSequenceRef.current = token.id;
    saveTokenRef.current = token;
    commit(token.scopeKey, (current) => ({
      ...current,
      command: "save",
      commandError: null,
      saveIssue: current.saveIssue?.kind === "not_applied"
        ? null
        : current.saveIssue,
    }));
    try {
      const response = await updateWorkspaceFileContentApi(
        token.agentId,
        token.path,
        token.draft,
        token.expectedRevision,
      );
      if (scopeRef.current.key !== token.scopeKey) {
        return;
      }
      commit(token.scopeKey, (current) => (
        mergeSavedMemoryDocument(
          current,
          token.draft,
          response.content,
          response.revision,
        )
      ));
      onSaved();
    } catch (error) {
      if (scopeRef.current.key === token.scopeKey) {
        commit(token.scopeKey, (current) => ({
          ...current,
          commandError: null,
          saveIssue: projectMemorySaveIssue(error, fallbackSaveError, token),
        }));
      }
    } finally {
      if (saveTokenRef.current?.id === token.id) {
        saveTokenRef.current = null;
        commit(token.scopeKey, (current) => ({ ...current, command: null }));
      }
    }
  }, [
    commit,
    fallbackSaveError,
    onSaved,
    runtimeWriting,
    scopeRef,
    state,
  ]);

  const save = useCallback(
    () => executeSave("normal"),
    [executeSave],
  );
  const overwriteConflict = useCallback(
    () => executeSave("overwrite_conflict"),
    [executeSave],
  );

  const reconcileSave = useCallback(async () => {
    const scope = scopeRef.current;
    const issue = state.saveIssue;
    if (
      reconcileRunningRef.current
      || saveTokenRef.current !== null
      || state.command
      || !scope.document
      || scope.key !== state.scopeKey
      || issue?.kind !== "outcome_unknown"
    ) {
      return;
    }
    reconcileRunningRef.current = true;
    const requestId = reconcileSequenceRef.current + 1;
    reconcileSequenceRef.current = requestId;
    commit(scope.key, (current) => ({
      ...current,
      command: "reconcile",
      commandError: null,
    }));
    try {
      const response = await getWorkspaceFileContentApi(
        scope.agentId,
        scope.document.path,
      );
      if (
        scopeRef.current.key !== scope.key
        || reconcileSequenceRef.current !== requestId
      ) {
        return;
      }
      const reconciliation = classifyMemorySaveReconciliation(issue, response);
      const saved = reconciliation === "saved";
      commit(scope.key, (current) => {
        if (saved) {
          return mergeSavedMemoryDocument(
            current,
            issue.attemptedDraft,
            response.content,
            response.revision,
          );
        }
        if (reconciliation === "not_applied") {
          return {
            ...current,
            content: response.content,
            resourceError: null,
            revision: response.revision,
            saveIssue: {
              detail: fallbackSaveError,
              kind: "not_applied",
            },
          };
        }
        return {
          ...current,
          content: response.content,
          resourceError: null,
          revision: response.revision,
          saveIssue: { kind: "conflict", phase: "review" },
        };
      });
      if (saved) {
        onSaved();
      }
    } catch {
      if (
        scopeRef.current.key === scope.key
        && reconcileSequenceRef.current === requestId
      ) {
        commit(scope.key, (current) => current.saveIssue?.kind === "outcome_unknown"
          ? {
              ...current,
              saveIssue: {
                ...current.saveIssue,
                reconciliationFailed: true,
              },
            }
          : current);
      }
    } finally {
      reconcileRunningRef.current = false;
      if (
        scopeRef.current.key === scope.key
        && reconcileSequenceRef.current === requestId
      ) {
        commit(scope.key, (current) => ({ ...current, command: null }));
      }
    }
  }, [commit, fallbackSaveError, onSaved, scopeRef, state]);

  return { overwriteConflict, reconcileSave, save };
}

function projectMemorySaveIssue(
  error: unknown,
  fallback: string,
  token: SaveToken,
): MemoryDocumentState["saveIssue"] {
  const failure = projectMutationFailure(error, fallback);
  if (
    failure.code === "workspace.file_revision_conflict"
    && failure.effect === "not_applied"
  ) {
    return { kind: "conflict", phase: "reload_required" };
  }
  if (failure.effect === "not_applied") {
    return { detail: failure.message, kind: "not_applied" };
  }
  return {
    attemptedDraft: token.draft,
    expectedRevision: token.expectedRevision,
    kind: "outcome_unknown",
    reconciliationFailed: false,
  };
}
