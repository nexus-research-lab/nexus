import { useCallback, useEffect, useRef, useState } from "react";

import {
  getWorkspaceFileContentApi,
  updateWorkspaceFileContentApi,
} from "@/lib/api/agent-manage-api";
import type { WorkspaceLiveFileState } from "@/types/app/workspace-live";
import type { MemoryDocument } from "@/types/memory/memory";

interface UseMemoryDocumentOptions {
  agentId: string;
  document: MemoryDocument | null;
  fallbackLoadError: string;
  fallbackSaveError: string;
  liveState?: WorkspaceLiveFileState;
  onSaved: () => void;
  runtimeWriting: boolean;
}

interface MemoryDocumentState {
  command: "save" | null;
  commandError: string | null;
  content: string;
  draft: string;
  editing: boolean;
  isLoading: boolean;
  resourceError: string | null;
  scopeKey: string;
}

interface DocumentScope {
  agentId: string;
  document: MemoryDocument | null;
  key: string;
}

interface SaveToken {
  draft: string;
  id: number;
  scopeKey: string;
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
  const scopeKey = document ? `${agentId}:${document.path}` : "";
  const scopeRef = useRef<DocumentScope>({ agentId, document, key: scopeKey });
  scopeRef.current = { agentId, document, key: scopeKey };
  const requestSequenceRef = useRef(0);
  const saveSequenceRef = useRef(0);
  const saveTokenRef = useRef<SaveToken | null>(null);
  const liveVersionRef = useRef(liveState?.version ?? 0);
  liveVersionRef.current = liveState?.version ?? 0;
  const consumedLiveVersionRef = useRef({ scopeKey, version: liveState?.version ?? 0 });
  const [storedState, setStoredState] = useState<MemoryDocumentState>(() =>
    createDocumentState(scopeKey),
  );
  const state = storedState.scopeKey === scopeKey
    ? storedState
    : createDocumentState(scopeKey);

  const commit = useCallback((expectedScopeKey: string, update: (
    current: MemoryDocumentState,
  ) => MemoryDocumentState) => {
    if (scopeRef.current.key !== expectedScopeKey) {
      return;
    }
    setStoredState((current) => {
      if (scopeRef.current.key !== expectedScopeKey) {
        return current;
      }
      return update(
        current.scopeKey === expectedScopeKey
          ? current
          : createDocumentState(expectedScopeKey),
      );
    });
  }, []);

  const reload = useCallback(async () => {
    const scope = scopeRef.current;
    if (!scope.document || scope.key !== scopeKey) {
      return;
    }
    const requestSequence = requestSequenceRef.current + 1;
    requestSequenceRef.current = requestSequence;
    commit(scope.key, (current) => ({
      ...current,
      isLoading: true,
      resourceError: null,
    }));
    try {
      const response = await getWorkspaceFileContentApi(scope.agentId, scope.document.path);
      if (!isCurrentRequest(scopeRef, scope.key, requestSequenceRef, requestSequence)) {
        return;
      }
      commit(scope.key, (current) => ({
        ...current,
        content: response.content,
        draft: current.editing ? current.draft : response.content,
        isLoading: false,
        resourceError: null,
      }));
    } catch (error) {
      if (!isCurrentRequest(scopeRef, scope.key, requestSequenceRef, requestSequence)) {
        return;
      }
      commit(scope.key, (current) => ({
        ...current,
        isLoading: false,
        resourceError: error instanceof Error ? error.message : fallbackLoadError,
      }));
    }
  }, [commit, fallbackLoadError, scopeKey]);

  const hasDocument = Boolean(document);
  useEffect(() => {
    requestSequenceRef.current += 1;
    consumedLiveVersionRef.current = {
      scopeKey,
      version: liveVersionRef.current,
    };
    if (hasDocument) {
      void reload();
    }
    return () => {
      requestSequenceRef.current += 1;
    };
  }, [hasDocument, reload, scopeKey]);

  useEffect(() => {
    if (state.editing || !liveState || !scopeKey) {
      return;
    }
    if (typeof liveState.live_content === "string") {
      requestSequenceRef.current += 1;
      consumedLiveVersionRef.current = { scopeKey, version: liveState.version };
      commit(scopeKey, (current) => ({
        ...current,
        content: liveState.live_content as string,
        draft: liveState.live_content as string,
        isLoading: false,
        resourceError: null,
      }));
      return;
    }
    const consumed = consumedLiveVersionRef.current;
    if (
      liveState.status === "updated"
      && (consumed.scopeKey !== scopeKey || liveState.version > consumed.version)
    ) {
      consumedLiveVersionRef.current = { scopeKey, version: liveState.version };
      void reload();
    }
  }, [commit, liveState, reload, scopeKey, state.editing]);

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
  }, [commit, fallbackSaveError, onSaved, runtimeWriting, scopeKey, state.content, state.draft]);

  return {
    ...state,
    cancelEditing: () => commit(scopeKey, (current) => ({
      ...current,
      commandError: null,
      draft: current.content,
      editing: false,
    })),
    dirty: state.draft !== state.content,
    isSaving: state.command === "save",
    reload,
    save,
    setDraft: (draft: string) => commit(scopeKey, (current) => ({ ...current, draft })),
    startEditing: () => commit(scopeKey, (current) => ({
      ...current,
      commandError: null,
      editing: true,
    })),
  };
}

function createDocumentState(scopeKey: string): MemoryDocumentState {
  return {
    command: null,
    commandError: null,
    content: "",
    draft: "",
    editing: false,
    isLoading: Boolean(scopeKey),
    resourceError: null,
    scopeKey,
  };
}

function isCurrentRequest(
  currentScope: { current: DocumentScope },
  expectedScopeKey: string,
  currentSequence: { current: number },
  expectedSequence: number,
): boolean {
  return currentScope.current.key === expectedScopeKey
    && currentSequence.current === expectedSequence;
}
