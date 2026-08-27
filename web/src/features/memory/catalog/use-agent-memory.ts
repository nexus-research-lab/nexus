import { useCallback, useEffect, useMemo, useRef } from "react";

import {
  deleteAgentMemoryDocumentApi,
  getAgentMemorySnapshotApi,
} from "@/lib/api/agent/memory-api";
import { getResourceFailure, type ResourceFailure } from "@/lib/error-message";
import type { MemorySnapshot } from "@/types/memory/memory";

import {
  type ScopedMemoryScope,
  type ScopedMemoryScopeRef,
  useScopedMemoryState,
} from "../use-scoped-memory-state";
import {
  type MemoryFilter,
  projectMemoryCatalog,
  resolveSelectedMemoryPath,
} from "./memory-catalog-model";

interface AgentMemoryState {
  compactDocumentOpen: boolean;
  deleteError: string | null;
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
}

export function useAgentMemory(
  agentId: string,
  fallbackError: string,
  fallbackDeleteError: string,
) {
  const deleteRequestSequenceRef = useRef(0);
  const requestSequenceRef = useRef(0);
  const { commit, scopeRef, state } = useScopedMemoryState(
    { agentId, key: agentId },
    createAgentMemoryState,
  );

  const refresh = useCallback(async () => {
    const expectedAgentId = agentId;
    const requestSequence = requestSequenceRef.current + 1;
    requestSequenceRef.current = requestSequence;
    commit(expectedAgentId, (current) => ({
      ...current,
      error: current.error?.access ? current.error : null,
      isLoading: true,
    }));
    try {
      const snapshot = await getAgentMemorySnapshotApi(expectedAgentId);
      if (!isCurrentRequest(scopeRef, expectedAgentId, requestSequenceRef, requestSequence)) {
        return;
      }
      commit(expectedAgentId, (current) => ({
        ...current,
        error: null,
        isLoading: false,
        selectedPath: resolveSelectedMemoryPath(snapshot, current.selectedPath),
        snapshot,
      }));
    } catch (error) {
      if (!isCurrentRequest(scopeRef, expectedAgentId, requestSequenceRef, requestSequence)) {
        return;
      }
      commit(expectedAgentId, (current) => ({
        ...current,
        error: getResourceFailure(error, fallbackError),
        isLoading: false,
      }));
    }
  }, [agentId, commit, fallbackError, scopeRef]);

  useEffect(() => {
    deleteRequestSequenceRef.current += 1;
    requestSequenceRef.current += 1;
    void refresh();
    return () => {
      deleteRequestSequenceRef.current += 1;
      requestSequenceRef.current += 1;
    };
  }, [agentId, refresh]);

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
    commit(agentId, (current) => ({
      ...current,
      compactDocumentOpen: true,
      deleteError: null,
      deleteTargetPath: "",
      selectedPath: path,
    }));
  }, [agentId, commit, projection.allDocuments]);

  const requestDeleteDocument = useCallback((path: string) => {
    if (state.deletingPath) {
      return;
    }
    const target = projection.allDocuments.find((document) => document.path === path);
    if (!target || target.kind === "index") {
      return;
    }
    commit(agentId, (current) => ({
      ...current,
      deleteError: null,
      deleteTargetPath: path,
    }));
  }, [agentId, commit, projection.allDocuments, state.deletingPath]);
  const cancelDeleteDocument = useCallback(() => {
    commit(agentId, (current) => ({ ...current, deleteTargetPath: "" }));
  }, [agentId, commit]);
  const confirmDeleteDocument = useCallback(async () => {
    if (state.deletingPath) {
      cancelDeleteDocument();
      return;
    }
    const target = projection.allDocuments.find((document) => (
      document.path === state.deleteTargetPath
    ));
    if (!target || target.kind === "index") {
      cancelDeleteDocument();
      return;
    }

    const expectedAgentId = agentId;
    const requestSequence = deleteRequestSequenceRef.current + 1;
    deleteRequestSequenceRef.current = requestSequence;
    commit(expectedAgentId, (current) => ({
      ...current,
      deleteError: null,
      deleteTargetPath: "",
      deletingPath: target.path,
    }));
    try {
      await deleteAgentMemoryDocumentApi(expectedAgentId, target.path);
      if (!isCurrentRequest(
        scopeRef,
        expectedAgentId,
        deleteRequestSequenceRef,
        requestSequence,
      )) {
        return;
      }
      await refresh();
      if (!isCurrentRequest(
        scopeRef,
        expectedAgentId,
        deleteRequestSequenceRef,
        requestSequence,
      )) {
        return;
      }
      commit(expectedAgentId, (current) => ({ ...current, deletingPath: "" }));
    } catch (error) {
      if (!isCurrentRequest(
        scopeRef,
        expectedAgentId,
        deleteRequestSequenceRef,
        requestSequence,
      )) {
        return;
      }
      commit(expectedAgentId, (current) => ({
        ...current,
        deleteError: error instanceof Error ? error.message : fallbackDeleteError,
        deletingPath: "",
      }));
    }
  }, [
    agentId,
    cancelDeleteDocument,
    commit,
    fallbackDeleteError,
    projection.allDocuments,
    refresh,
    scopeRef,
    state.deleteTargetPath,
    state.deletingPath,
  ]);
  const closeCompactDocument = useCallback(() => {
    commit(agentId, (current) => ({ ...current, compactDocumentOpen: false }));
  }, [agentId, commit]);
  const setFilter = useCallback((filter: MemoryFilter) => {
    commit(agentId, (current) => ({ ...current, filter }));
  }, [agentId, commit]);
  const setQuery = useCallback((query: string) => {
    commit(agentId, (current) => ({ ...current, query }));
  }, [agentId, commit]);

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
      cancelDeleteDocument,
      closeCompactDocument,
      compactDocumentOpen: state.compactDocumentOpen,
      confirmDeleteDocument,
      deleteError: state.deleteError,
      deleteTarget,
      deletingPath: state.deletingPath,
      requestDeleteDocument,
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

function createAgentMemoryState(agentId: string): AgentMemoryState {
  return {
    compactDocumentOpen: false,
    deleteError: null,
    deleteTargetPath: "",
    deletingPath: "",
    error: null,
    filter: "all",
    isLoading: true,
    query: "",
    selectedPath: "",
    snapshot: null,
    scopeKey: agentId,
  };
}

function isCurrentRequest(
  currentScope: ScopedMemoryScopeRef<AgentMemoryScope>,
  expectedAgentId: string,
  currentSequence: { current: number },
  expectedSequence: number,
): boolean {
  return currentScope.current.agentId === expectedAgentId
    && currentSequence.current === expectedSequence;
}
