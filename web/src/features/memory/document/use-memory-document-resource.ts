import { useCallback, useEffect, useRef } from "react";

import { getWorkspaceFileContentApi } from "@/lib/api/agent/agent-api";
import type { WorkspaceLiveFileState } from "@/types/app/workspace-live";

import type {
  MemoryDocumentCommit,
  MemoryDocumentScopeRef,
} from "./use-memory-document-state";
import {
  type ConsumedMemoryLiveVersion,
  resolveMemoryLiveUpdateIntent,
} from "./memory-document-model";

interface UseMemoryDocumentResourceOptions {
  commit: MemoryDocumentCommit;
  editing: boolean;
  fallbackLoadError: string;
  liveState?: WorkspaceLiveFileState;
  scopeKey: string;
  scopeRef: MemoryDocumentScopeRef;
}

export function useMemoryDocumentResource({
  commit,
  editing,
  fallbackLoadError,
  liveState,
  scopeKey,
  scopeRef,
}: UseMemoryDocumentResourceOptions) {
  const requestSequenceRef = useRef(0);
  const liveVersionRef = useRef(liveState?.version ?? 0);
  liveVersionRef.current = liveState?.version ?? 0;
  const consumedLiveVersionRef = useRef<ConsumedMemoryLiveVersion>({
    scopeKey,
    version: liveState?.version ?? 0,
  });

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
  }, [commit, fallbackLoadError, scopeKey, scopeRef]);

  useEffect(() => {
    requestSequenceRef.current += 1;
    consumedLiveVersionRef.current = {
      scopeKey,
      version: liveVersionRef.current,
    };
    if (scopeKey) {
      void reload();
    }
    return () => {
      requestSequenceRef.current += 1;
    };
  }, [reload, scopeKey]);

  useEffect(() => {
    const intent = resolveMemoryLiveUpdateIntent({
      consumed: consumedLiveVersionRef.current,
      editing,
      liveState,
      scopeKey,
    });
    if (intent.kind === "ignore") {
      return;
    }
    consumedLiveVersionRef.current = { scopeKey, version: intent.version };
    if (intent.kind === "apply") {
      requestSequenceRef.current += 1;
      commit(scopeKey, (current) => ({
        ...current,
        content: intent.content,
        draft: intent.content,
        isLoading: false,
        resourceError: null,
      }));
      return;
    }
    void reload();
  }, [commit, editing, liveState, reload, scopeKey]);

  return { reload };
}

function isCurrentRequest(
  currentScope: MemoryDocumentScopeRef,
  expectedScopeKey: string,
  currentSequence: { current: number },
  expectedSequence: number,
): boolean {
  return currentScope.current.key === expectedScopeKey
    && currentSequence.current === expectedSequence;
}
