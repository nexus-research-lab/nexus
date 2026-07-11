import { useCallback, useRef } from "react";

import { updateWorkspaceFileContentApi } from "@/lib/api/agent-manage-api";

import type {
  MemoryDocumentCommit,
  MemoryDocumentScopeRef,
  MemoryDocumentState,
} from "./use-memory-document-state";

interface SaveToken {
  draft: string;
  id: number;
  scopeKey: string;
}

interface UseMemoryDocumentSaveOptions {
  commit: MemoryDocumentCommit;
  fallbackSaveError: string;
  onSaved: () => void;
  runtimeWriting: boolean;
  scopeKey: string;
  scopeRef: MemoryDocumentScopeRef;
  state: MemoryDocumentState;
}

export function useMemoryDocumentSave({
  commit,
  fallbackSaveError,
  onSaved,
  runtimeWriting,
  scopeKey,
  scopeRef,
  state,
}: UseMemoryDocumentSaveOptions) {
  const saveSequenceRef = useRef(0);
  const saveTokenRef = useRef<SaveToken | null>(null);

  const save = useCallback(async () => {
    const scope = scopeRef.current;
    const draft = state.draft;
    if (
      !scope.document
      || scope.key !== scopeKey
      || draft === state.content
      || runtimeWriting
      || saveTokenRef.current?.scopeKey === scope.key
    ) {
      return;
    }
    const token = {
      draft,
      id: saveSequenceRef.current + 1,
      scopeKey: scope.key,
    };
    saveSequenceRef.current = token.id;
    saveTokenRef.current = token;
    commit(scope.key, (current) => ({
      ...current,
      command: "save",
      commandError: null,
    }));
    try {
      const response = await updateWorkspaceFileContentApi(
        scope.agentId,
        scope.document.path,
        token.draft,
      );
      if (scopeRef.current.key !== token.scopeKey) {
        return;
      }
      commit(scope.key, (current) => {
        const draftWasUnchanged = current.draft === token.draft;
        return {
          ...current,
          commandError: null,
          content: response.content,
          draft: draftWasUnchanged ? response.content : current.draft,
          editing: !draftWasUnchanged,
        };
      });
      onSaved();
    } catch (error) {
      if (scopeRef.current.key === token.scopeKey) {
        commit(scope.key, (current) => ({
          ...current,
          commandError: error instanceof Error ? error.message : fallbackSaveError,
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
    scopeKey,
    scopeRef,
    state.content,
    state.draft,
  ]);

  return { save };
}
