// INPUT: Memory 文档 scope、live 状态与读写回调。
// OUTPUT: 读取、编辑、条件保存、对账和冲突决策控制面。
// POS: Memory Document 组合层；不拥有底层文件或业务身份。
import { useCallback } from "react";

import type { WorkspaceLiveFileState } from "@/types/app/workspace-live";
import type { MemoryDocument } from "@/types/memory/memory";

import { useMemoryDocumentResource } from "./use-memory-document-resource";
import { useMemoryDocumentSave } from "./use-memory-document-save";
import { useMemoryDocumentState } from "./use-memory-document-state";

interface UseMemoryDocumentOptions {
  agentId: string;
  document: MemoryDocument | null;
  fallbackLoadError: string;
  fallbackSaveError: string;
  liveState?: WorkspaceLiveFileState;
  onSaved: () => void;
  runtimeWriting: boolean;
}

export function useMemoryDocument({
  agentId,
  document,
  fallbackLoadError,
  fallbackSaveError,
  liveState,
  onSaved,
  runtimeWriting,
}: UseMemoryDocumentOptions) {
  const { commit, scopeKey, scopeRef, state } = useMemoryDocumentState(agentId, document);
  const { reload } = useMemoryDocumentResource({
    accessBlocked: Boolean(state.resourceError?.access),
    commit,
    contentRevision: state.revision,
    editing: state.editing,
    fallbackLoadError,
    liveState,
    saving: state.command !== null,
    scopeKey,
    scopeRef,
  });
  const { overwriteConflict, reconcileSave, save } = useMemoryDocumentSave({
    commit,
    fallbackSaveError,
    onSaved,
    runtimeWriting,
    scopeRef,
    state,
  });
  const cancelEditing = useCallback(() => {
    commit(scopeKey, (current) => ({
      ...current,
      commandError: null,
      draft: current.content,
      editing: false,
      saveIssue: null,
    }));
    void reload();
  }, [commit, reload, scopeKey]);
  const adoptLatest = useCallback(() => {
    commit(scopeKey, (current) => ({
      ...current,
      commandError: null,
      draft: current.content,
      editing: false,
      saveIssue: null,
    }));
  }, [commit, scopeKey]);
  const setDraft = useCallback((draft: string) => {
    commit(scopeKey, (current) => ({ ...current, draft }));
  }, [commit, scopeKey]);
  const startEditing = useCallback(() => {
    commit(scopeKey, (current) => ({
      ...current,
      commandError: null,
      editing: true,
      saveIssue: current.saveIssue?.kind === "not_applied"
        ? null
        : current.saveIssue,
    }));
  }, [commit, scopeKey]);

  return {
    ...state,
    adoptLatest,
    cancelEditing,
    dirty: state.draft !== state.content,
    isReconciling: state.command === "reconcile",
    isSaving: state.command === "save",
    overwriteConflict,
    reconcileSave,
    reload,
    save,
    saveBlocked: Boolean(
      state.saveIssue && state.saveIssue.kind !== "not_applied",
    ),
    setDraft,
    startEditing,
  };
}
