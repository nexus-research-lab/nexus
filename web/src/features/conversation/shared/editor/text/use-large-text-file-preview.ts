// INPUT: Exact owner generation, Agent/path and explicit chunk navigation.
// OUTPUT: One bounded chunk and its pagination/loading facts; obsolete requests cannot commit.
// POS: Read-only large-text controller; never joins chunks, edits files or replays writes.
import { useCallback, useEffect, useRef, useSyncExternalStore } from "react";
import { getWorkspaceFileTextChunkApi } from "@/lib/api/agent/agent-api";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import type { WorkspaceFileTextChunk } from "@/types/agent/agent";

interface ChunkPreviewState {
  offsets: number[];
  pageIndex: number;
  requestRevision: number;
  chunk: WorkspaceFileTextChunk | null;
  loadState: "error" | "loaded" | "loading";
}
const INITIAL_STATE: ChunkPreviewState = {
  offsets: [0], pageIndex: 0, requestRevision: 0, chunk: null, loadState: "loading",
};

export function useLargeTextFilePreview(agentId: string, path: string) {
  const ownerGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  const scopeKey = JSON.stringify([ownerGeneration, agentId, path]);
  const scopeRef = useRef(scopeKey);
  scopeRef.current = scopeKey;
  const [state, setState] = useResettableState(INITIAL_STATE, scopeKey);
  const { offsets, pageIndex, requestRevision } = state;
  const offset = offsets[pageIndex] ?? 0;

  useEffect(() => {
    if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) return;
    const controller = new AbortController();
    const isCurrent = () => !controller.signal.aborted && scopeRef.current === scopeKey
      && isAuthOwnerScopeGenerationCurrent(ownerGeneration);
    void getWorkspaceFileTextChunkApi(agentId, path, offset, controller.signal)
      .then((chunk) => {
        if (!isCurrent()) return;
        setState((current) => isCurrent() && current.requestRevision === requestRevision
          ? { ...current, chunk, loadState: "loaded" }
          : current);
      })
      .catch(() => {
        if (!isCurrent()) return;
        setState((current) => isCurrent() && current.requestRevision === requestRevision
          ? { ...current, chunk: null, loadState: "error" }
          : current);
      });
    return () => controller.abort();
  }, [agentId, offset, ownerGeneration, path, requestRevision, scopeKey, setState]);

  const loadFromStart = useCallback(() => {
    setState((current) => ({ ...INITIAL_STATE, requestRevision: current.requestRevision + 1 }));
  }, [setState]);
  const loadPrevious = useCallback(() => {
    setState((current) => current.loadState !== "loaded" || current.pageIndex === 0
      ? current
      : {
          ...current, pageIndex: current.pageIndex - 1, chunk: null,
          loadState: "loading", requestRevision: current.requestRevision + 1,
        });
  }, [setState]);
  const loadNext = useCallback(() => {
    setState((current) => {
      const nextOffset = current.chunk?.nextOffset;
      if (current.loadState !== "loaded" || nextOffset == null) return current;
      return {
        offsets: [...current.offsets.slice(0, current.pageIndex + 1), nextOffset],
        pageIndex: current.pageIndex + 1, chunk: null, loadState: "loading",
        requestRevision: current.requestRevision + 1,
      };
    });
  }, [setState]);

  return {
    chunk: state.chunk,
    hasError: state.loadState === "error",
    isLoading: state.loadState === "loading",
    pageIndex,
    loadFromStart, loadPrevious, loadNext,
  };
}
