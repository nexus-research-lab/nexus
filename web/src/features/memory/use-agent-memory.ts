import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { getAgentMemorySnapshotApi } from "@/lib/api/memory-api";
import type { MemoryDocument, MemorySnapshot } from "@/types/memory/memory";

import {
  memoryDocumentMatches,
  type MemoryFilter,
} from "./memory-utils";

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
      commit(expectedAgentId, (current) => {
        const documents = getAllMemoryDocuments(snapshot);
        const selectedPath = documents.some((document) =>
          document.path === current.selectedPath)
          ? current.selectedPath
          : documents[0]?.path ?? "";
        return {
          ...current,
          error: null,
          isLoading: false,
          selectedPath,
          snapshot,
        };
      });
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

  const allDocuments = useMemo(
    () => getAllMemoryDocuments(state.snapshot),
    [state.snapshot],
  );
  const visibleDocuments = useMemo(
    () => (state.snapshot?.documents ?? []).filter((document) =>
      memoryDocumentMatches(document, state.filter, state.query)),
    [state.filter, state.query, state.snapshot?.documents],
  );
  const selectedDocument = useMemo(
    () => allDocuments.find((document) => document.path === state.selectedPath) ?? null,
    [allDocuments, state.selectedPath],
  );

  const selectDocument = useCallback((path: string) => {
    if (!allDocuments.some((document) => document.path === path)) {
      return;
    }
    commit(agentId, (current) => ({
      ...current,
      compactDocumentOpen: true,
      selectedPath: path,
    }));
  }, [agentId, allDocuments, commit]);

  return {
    ...state,
    allDocuments,
    counts: countMemoryDocuments(state.snapshot),
    indexVisible: Boolean(
      state.snapshot?.index
      && memoryDocumentMatches(state.snapshot.index, "index", state.query),
    ),
    latestDocument: state.snapshot?.documents[0] ?? state.snapshot?.index ?? null,
    refresh,
    selectDocument,
    selectedDocument,
    setCompactDocumentOpen: (open: boolean) => commit(agentId, (current) => ({
      ...current,
      compactDocumentOpen: open,
    })),
    setFilter: (filter: MemoryFilter) => commit(agentId, (current) => ({
      ...current,
      filter,
    })),
    setQuery: (query: string) => commit(agentId, (current) => ({
      ...current,
      query,
    })),
    visibleDocuments,
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

function getAllMemoryDocuments(snapshot: MemorySnapshot | null): MemoryDocument[] {
  return snapshot
    ? [snapshot.index, ...snapshot.documents].filter(Boolean) as MemoryDocument[]
    : [];
}

function countMemoryDocuments(
  snapshot: MemorySnapshot | null,
): { logs: number; topics: number } {
  return (snapshot?.documents ?? []).reduce(
    (counts, document) => ({
      logs: counts.logs + Number(document.kind === "daily_log"),
      topics: counts.topics + Number(document.kind !== "daily_log"),
    }),
    { logs: 0, topics: 0 },
  );
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
