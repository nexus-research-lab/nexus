import { useCallback, useRef, useState } from "react";

import type { MemoryDocument } from "@/types/memory/memory";

export interface MemoryDocumentState {
  command: "save" | null;
  commandError: string | null;
  content: string;
  draft: string;
  editing: boolean;
  isLoading: boolean;
  resourceError: string | null;
  scopeKey: string;
}

export interface MemoryDocumentScope {
  agentId: string;
  document: MemoryDocument | null;
  key: string;
}

export interface MemoryDocumentScopeRef {
  current: MemoryDocumentScope;
}

export type MemoryDocumentCommit = (
  expectedScopeKey: string,
  update: (current: MemoryDocumentState) => MemoryDocumentState,
) => void;

export function mergeSavedMemoryDocument(
  current: MemoryDocumentState,
  savedDraft: string,
  savedContent: string,
): MemoryDocumentState {
  const draftWasUnchanged = current.draft === savedDraft;
  return {
    ...current,
    commandError: null,
    content: savedContent,
    draft: draftWasUnchanged ? savedContent : current.draft,
    editing: !draftWasUnchanged,
  };
}

export function useMemoryDocumentState(
  agentId: string,
  document: MemoryDocument | null,
) {
  const scopeKey = document ? `${agentId}:${document.path}` : "";
  const scopeRef = useRef<MemoryDocumentScope>({ agentId, document, key: scopeKey });
  scopeRef.current = { agentId, document, key: scopeKey };
  const [storedState, setStoredState] = useState<MemoryDocumentState>(() =>
    createMemoryDocumentState(scopeKey),
  );
  const state = storedState.scopeKey === scopeKey
    ? storedState
    : createMemoryDocumentState(scopeKey);

  const commit = useCallback<MemoryDocumentCommit>((expectedScopeKey, update) => {
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
          : createMemoryDocumentState(expectedScopeKey),
      );
    });
  }, []);

  return { commit, scopeKey, scopeRef, state };
}

function createMemoryDocumentState(scopeKey: string): MemoryDocumentState {
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
