import type { MemoryDocument } from "@/types/memory/memory";

import {
  type ScopedMemoryCommit,
  type ScopedMemoryScope,
  type ScopedMemoryScopeRef,
  useScopedMemoryState,
} from "../use-scoped-memory-state";

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

export interface MemoryDocumentScope extends ScopedMemoryScope {
  agentId: string;
  document: MemoryDocument | null;
}

export type MemoryDocumentScopeRef = ScopedMemoryScopeRef<MemoryDocumentScope>;

export type MemoryDocumentCommit = ScopedMemoryCommit<MemoryDocumentState>;

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
  const { commit, scopeRef, state } = useScopedMemoryState(
    { agentId, document, key: scopeKey },
    createMemoryDocumentState,
  );
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
