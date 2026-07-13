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
    commit,
    editing: state.editing,
    fallbackLoadError,
    liveState,
    scopeKey,
    scopeRef,
  });
  const { save } = useMemoryDocumentSave({
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
    }));
  }, [commit, scopeKey]);

  return {
    ...state,
    cancelEditing,
    dirty: state.draft !== state.content,
    isSaving: state.command === "save",
    reload,
    save,
    setDraft,
    startEditing,
  };
}
