import { useCallback, useEffect, useMemo, useRef } from "react";

import { getAgentMemorySnapshotApi } from "@/lib/api/agent/memory-api";
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
  error: string | null;
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

export function useAgentMemory(agentId: string, fallbackError: string) {
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
      error: null,
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
        error: error instanceof Error ? error.message : fallbackError,
        isLoading: false,
        selectedPath: "",
        snapshot: null,
      }));
    }
  }, [agentId, commit, fallbackError, scopeRef]);

  useEffect(() => {
    requestSequenceRef.current += 1;
    void refresh();
    return () => {
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

  const selectDocument = useCallback((path: string) => {
    if (!projection.allDocuments.some((document) => document.path === path)) {
      return;
    }
    commit(agentId, (current) => ({
      ...current,
      compactDocumentOpen: true,
      selectedPath: path,
    }));
  }, [agentId, commit, projection.allDocuments]);
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
      closeCompactDocument,
      compactDocumentOpen: state.compactDocumentOpen,
      selectDocument,
      selectedDocument: projection.selectedDocument,
    },
    resource: {
      error: state.error,
      isLoading: state.isLoading,
      refresh,
      snapshot: state.snapshot,
    },
    summary: {
      counts: projection.counts,
      latestDocument: projection.latestDocument,
    },
  };
}

function createAgentMemoryState(agentId: string): AgentMemoryState {
  return {
    compactDocumentOpen: false,
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
