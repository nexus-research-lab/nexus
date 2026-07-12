import { useCallback, useRef } from "react";

import { updateWorkspaceFileContentApi } from "@/lib/api/agent/agent-api";

import {
  mergeSavedMemoryDocument,
  type MemoryDocumentCommit,
  type MemoryDocumentScope,
  type MemoryDocumentScopeRef,
  type MemoryDocumentState,
} from "./use-memory-document-state";

interface SaveToken {
  agentId: string;
  draft: string;
  id: number;
  path: string;
  scopeKey: string;
}

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
  nextId,
  runtimeWriting,
  scope,
  state,
}: {
  activeToken: SaveToken | null;
  nextId: number;
  runtimeWriting: boolean;
  scope: MemoryDocumentScope;
  state: MemoryDocumentState;
}): SaveToken | null {
  if (
    !scope.document
    || scope.key !== state.scopeKey
    || state.draft === state.content
    || runtimeWriting
    || activeToken?.scopeKey === scope.key
  ) {
    return null;
  }
  return {
    agentId: scope.agentId,
    draft: state.draft,
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

  const save = useCallback(async () => {
    const scope = scopeRef.current;
    const token = createSaveToken({
      activeToken: saveTokenRef.current,
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
    }));
    try {
      const response = await updateWorkspaceFileContentApi(
        token.agentId,
        token.path,
        token.draft,
      );
      if (scopeRef.current.key !== token.scopeKey) {
        return;
      }
      commit(token.scopeKey, (current) => (
        mergeSavedMemoryDocument(current, token.draft, response.content)
      ));
      onSaved();
    } catch (error) {
      if (scopeRef.current.key === token.scopeKey) {
        commit(token.scopeKey, (current) => ({
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
    scopeRef,
    state,
  ]);

  return { save };
}
