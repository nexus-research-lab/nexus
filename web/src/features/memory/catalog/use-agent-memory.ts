import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { getAgentMemorySnapshotApi } from "@/lib/api/memory-api";
import type { MemorySnapshot } from "@/types/memory/memory";

import {
  type MemoryFilter,
  projectMemoryCatalog,
  resolveSelectedMemoryPath,
} from "./memory-catalog-model";

interface AgentMemoryState {
  agentId: string;
  compactDocumentOpen: boolean;
  error: string | null;
  filter: MemoryFilter;
  isLoading: boolean;
  query: string;
  selectedPath: string;
  snapshot: MemorySnapshot | null;
}

export function useAgentMemory(agentId: string, fallbackError: string) {
  const agentIdRef = useRef(agentId);
  agentIdRef.current = agentId;
  const requestSequenceRef = useRef(0);
  const [storedState, setStoredState] = useState<AgentMemoryState>(() =>
    createAgentMemoryState(agentId),
  );
  const state = storedState.agentId === agentId
    ? storedState
    : createAgentMemoryState(agentId);

  const commit = useCallback((expectedAgentId: string, update: (
    current: AgentMemoryState,
  ) => AgentMemoryState) => {
    if (agentIdRef.current !== expectedAgentId) {
      return;
    }
    setStoredState((current) => {
      if (agentIdRef.current !== expectedAgentId) {
        return current;
      }
      return update(
        current.agentId === expectedAgentId
          ? current
          : createAgentMemoryState(expectedAgentId),
      );
    });
  }, []);

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
      if (!isCurrentRequest(agentIdRef, expectedAgentId, requestSequenceRef, requestSequence)) {
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
      if (!isCurrentRequest(agentIdRef, expectedAgentId, requestSequenceRef, requestSequence)) {
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
  }, [agentId, commit, fallbackError]);

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
      filter: state.filter,
      indexVisible: projection.indexVisible,
      query: state.query,
      selectedPath: state.selectedPath,
      setFilter,
      setQuery,
      snapshot: state.snapshot,
      visibleDocuments: projection.visibleDocuments,
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
    agentId,
    compactDocumentOpen: false,
    error: null,
    filter: "all",
    isLoading: true,
    query: "",
    selectedPath: "",
    snapshot: null,
  };
}

function isCurrentRequest(
  currentAgentId: { current: string },
  expectedAgentId: string,
  currentSequence: { current: number },
  expectedSequence: number,
): boolean {
  return currentAgentId.current === expectedAgentId
    && currentSequence.current === expectedSequence;
}
