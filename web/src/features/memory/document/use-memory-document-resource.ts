// INPUT: exact Memory scope、HTTP 读取和带 revision 的 workspace live 状态。
// OUTPUT: 旧请求隔离的读取/刷新，编辑期间的并发变更转为冲突。
// POS: Memory 读资源边界；实时内容不覆盖编辑草稿。
import { useCallback, useEffect, useRef } from "react";

import { getWorkspaceFileContentApi } from "@/lib/api/agent/agent-api";
import { getResourceFailure } from "@/lib/error-message";
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
  accessBlocked: boolean;
  commit: MemoryDocumentCommit;
  contentRevision: string | null;
  editing: boolean;
  fallbackLoadError: string;
  liveState?: WorkspaceLiveFileState;
  saving: boolean;
  scopeKey: string;
  scopeRef: MemoryDocumentScopeRef;
}

export function useMemoryDocumentResource({
  accessBlocked,
  commit,
  contentRevision,
  editing,
  fallbackLoadError,
  liveState,
  saving,
  scopeKey,
  scopeRef,
}: UseMemoryDocumentResourceOptions) {
  const requestSequenceRef = useRef(0);
  const liveVersionRef = useRef(completedLiveVersion(liveState));
  liveVersionRef.current = completedLiveVersion(liveState);
  const consumedLiveVersionRef = useRef<ConsumedMemoryLiveVersion>({
    scopeKey,
    version: completedLiveVersion(liveState),
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
      resourceError: current.resourceError?.access
        ? current.resourceError
        : null,
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
        revision: response.revision || null,
        saveIssue: current.saveIssue?.kind === "conflict"
          ? { kind: "conflict", phase: "review" }
          : current.saveIssue,
      }));
    } catch (error) {
      if (!isCurrentRequest(scopeRef, scope.key, requestSequenceRef, requestSequence)) {
        return;
      }
      commit(scope.key, (current) => ({
        ...current,
        isLoading: false,
        resourceError: getResourceFailure(error, fallbackLoadError),
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
    if (accessBlocked) {
      consumedLiveVersionRef.current = {
        scopeKey,
        version: liveState?.version ?? liveVersionRef.current,
      };
      return;
    }
    const intent = resolveMemoryLiveUpdateIntent({
      consumed: consumedLiveVersionRef.current,
      contentRevision,
      editing,
      liveState,
      saving,
      scopeKey,
    });
    if (intent.kind === "ignore") {
      return;
    }
    consumedLiveVersionRef.current = { scopeKey, version: intent.version };
    if (intent.kind === "consume") {
      return;
    }
    if (intent.kind === "conflict") {
      commit(scopeKey, (current) => current.saveIssue?.kind === "outcome_unknown"
        ? current
        : {
            ...current,
            commandError: null,
            saveIssue: { kind: "conflict", phase: "reload_required" },
          });
      return;
    }
    if (intent.kind === "apply") {
      requestSequenceRef.current += 1;
      commit(scopeKey, (current) => current.resourceError?.access
        ? current
        : {
            ...current,
            content: intent.content,
            draft: intent.content,
            isLoading: false,
            resourceError: null,
            revision: intent.revision,
            saveIssue: null,
          });
      return;
    }
    void reload();
  }, [
    accessBlocked,
    commit,
    contentRevision,
    editing,
    liveState,
    reload,
    saving,
    scopeKey,
  ]);

  return { reload };
}

function completedLiveVersion(liveState?: WorkspaceLiveFileState): number {
  if (!liveState) {
    return 0;
  }
  return liveState.status === "updated"
    ? liveState.version
    : Math.max(0, liveState.version - 1);
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
