// INPUT: exact owner generation + Agent + path、文件 revision API 与同 scope 实时状态。
// OUTPUT: 不丢草稿的加载、超限分段信号、条件保存、冲突选择和迟到响应栅栏。
// POS: 通用文本编辑可靠性边界；只读核对不会自动重放写入。
import {
  useCallback,
  useEffect,
  useRef,
  useSyncExternalStore,
  type Dispatch,
  type SetStateAction,
} from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import {
  getWorkspaceFileContentApi,
  updateWorkspaceFileContentApi,
} from "@/lib/api/agent/agent-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import {
  getResourceFailure,
  projectMutationFailure,
  type ResourceFailure,
} from "@/lib/error-message";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import { useWorkspaceLiveStore } from "@/store/workspace-live";
import type { WorkspaceFileContent } from "@/types/agent/agent";

import {
  classifyTextFileSaveReconciliation,
  completedTextFileLiveVersion,
  mergeSavedTextFile,
  resolveTextFileLiveUpdateIntent,
  type ConsumedTextFileLiveVersion,
  type TextFileSaveIssue,
} from "./text-file-editor-recovery";

interface UseTextFileEditorParams {
  agentId: string;
  fallbackLoadError: string;
  fallbackSaveError: string;
  path: string;
}

interface TextFileEditorScope {
  agentId: string;
  key: string;
  ownerGeneration: number;
  path: string;
}

interface TextFileEditorState {
  command: "reconcile" | "save" | null;
  draftContent: string;
  hasLoadedContent: boolean;
  isEditing: boolean;
  isLoading: boolean;
  requiresChunkedPreview: boolean;
  resourceFailure: ResourceFailure | null;
  revision: string | null;
  savedContent: string;
  saveIssue: TextFileSaveIssue | null;
  scopeKey: string;
}

interface SaveToken extends TextFileEditorScope {
  attemptedDraft: string;
  expectedRevision: string;
  id: number;
}

type SaveMode = "normal" | "overwrite_conflict";

export function useTextFileEditor({
  agentId,
  fallbackLoadError,
  fallbackSaveError,
  path,
}: UseTextFileEditorParams) {
  const ownerGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  const scopeKey = `${ownerGeneration}\u0000${agentId}\u0000${path}`;
  const scopeRef = useRef<TextFileEditorScope>({
    agentId,
    key: scopeKey,
    ownerGeneration,
    path,
  });
  scopeRef.current = {
    agentId,
    key: scopeKey,
    ownerGeneration,
    path,
  };
  const fallbackCopyRef = useRef({ fallbackLoadError, fallbackSaveError });
  fallbackCopyRef.current = { fallbackLoadError, fallbackSaveError };
  const [state, setState] = useResettableState(
    createTextFileEditorState(scopeKey),
    scopeKey,
  );
  const commit = useCallback((
    expectedScopeKey: string,
    update: (current: TextFileEditorState) => TextFileEditorState,
  ) => {
    setState((current) => current.scopeKey === expectedScopeKey
      ? update(current)
      : current);
  }, [setState]);
  const loadSequenceRef = useRef(0);
  const saveSequenceRef = useRef(0);
  const reconcileSequenceRef = useRef(0);
  const saveTokenRef = useRef<SaveToken | null>(null);
  const reconcileTokenRef = useRef<SaveToken | null>(null);
  const liveState = useWorkspaceLiveStore((store) => {
    const candidate = store.file_states[`${agentId}:${path}`];
    return candidate?.agent_id === agentId && candidate.path === path
      ? candidate
      : undefined;
  });
  const liveVersionRef = useRef(completedTextFileLiveVersion(liveState));
  liveVersionRef.current = completedTextFileLiveVersion(liveState);
  const consumedLiveVersionRef = useRef<ConsumedTextFileLiveVersion>({
    scopeKey,
    version: completedTextFileLiveVersion(liveState),
  });
  const isExternalWriting = Boolean(
    liveState
    && liveState.source !== "api"
    && liveState.status === "writing",
  );
  const externalWritingRef = useRef(isExternalWriting);
  externalWritingRef.current = isExternalWriting;
  const isDirty = state.draftContent !== state.savedContent;
  const isSaving = state.command === "save";
  const isReconciling = state.command === "reconcile";

  const loadContent = useCallback(async (): Promise<void> => {
    const requestScope = scopeRef.current;
    if (
      saveTokenRef.current?.key === requestScope.key
      || reconcileTokenRef.current?.key === requestScope.key
    ) {
      return;
    }
    const requestId = loadSequenceRef.current + 1;
    loadSequenceRef.current = requestId;
    commit(requestScope.key, (current) => ({ ...current, isLoading: true }));
    try {
      const response = await getWorkspaceFileContentApi(
        requestScope.agentId,
        requestScope.path,
      );
      if (!isCurrentEditorRequest(scopeRef, requestScope, loadSequenceRef, requestId)) {
        return;
      }
      if (!isExactWorkspaceFileResponse(response, requestScope.path)) {
        throw new Error(fallbackCopyRef.current.fallbackLoadError);
      }
      commit(requestScope.key, (current) => mergeLoadedTextFile(current, response));
    } catch (error) {
      if (!isCurrentEditorRequest(scopeRef, requestScope, loadSequenceRef, requestId)) {
        return;
      }
      const failure = getResourceFailure(
        error,
        fallbackCopyRef.current.fallbackLoadError,
      );
      if (isWorkspaceWholeFileTooLarge(error)) {
        commit(requestScope.key, (current) => ({
          ...current,
          isLoading: false,
          requiresChunkedPreview: true,
          resourceFailure: failure,
        }));
        return;
      }
      commit(requestScope.key, (current) => failure.access
        ? clearTextFileForAccess(current, failure)
        : {
            ...current,
            isLoading: false,
            requiresChunkedPreview: false,
            resourceFailure: failure,
          });
    } finally {
      if (isCurrentEditorRequest(scopeRef, requestScope, loadSequenceRef, requestId)) {
        commit(requestScope.key, (current) => ({ ...current, isLoading: false }));
      }
    }
  }, [commit]);

  useEffect(() => {
    loadSequenceRef.current += 1;
    reconcileSequenceRef.current += 1;
    consumedLiveVersionRef.current = {
      scopeKey,
      version: liveVersionRef.current,
    };
    void loadContent();
    return () => {
      loadSequenceRef.current += 1;
      reconcileSequenceRef.current += 1;
    };
  }, [loadContent, scopeKey]);

  useEffect(() => {
    if (state.requiresChunkedPreview) {
      return;
    }
    const intent = resolveTextFileLiveUpdateIntent({
      agentId,
      consumed: consumedLiveVersionRef.current,
      hasLoadedContent: state.hasLoadedContent,
      isDirty,
      isEditing: state.isEditing,
      isSaving,
      liveState,
      path,
      revision: state.revision,
      saveIssue: state.saveIssue,
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
            resourceFailure: null,
            saveIssue: { kind: "conflict", phase: "reload_required" },
          });
      return;
    }
    if (intent.kind === "apply") {
      loadSequenceRef.current += 1;
      commit(scopeKey, (current) => ({
        ...current,
        draftContent: intent.content,
        hasLoadedContent: true,
        isLoading: false,
        resourceFailure: null,
        revision: intent.revision,
        savedContent: intent.content,
        saveIssue: null,
      }));
      return;
    }
    void loadContent();
  }, [
    agentId,
    commit,
    isDirty,
    isSaving,
    liveState,
    loadContent,
    path,
    scopeKey,
    state.hasLoadedContent,
    state.isEditing,
    state.revision,
    state.requiresChunkedPreview,
    state.saveIssue,
  ]);

  const executeSave = useCallback(async (mode: SaveMode): Promise<boolean> => {
    const requestScope = scopeRef.current;
    const token = createSaveToken({
      activeReconcile: reconcileTokenRef.current,
      activeSave: saveTokenRef.current,
      isDirty,
      isExternalWriting: externalWritingRef.current,
      mode,
      nextId: saveSequenceRef.current + 1,
      scope: requestScope,
      state,
    });
    if (!token) {
      return false;
    }
    saveSequenceRef.current = token.id;
    saveTokenRef.current = token;
    commit(token.key, (current) => ({
      ...current,
      command: "save",
      resourceFailure: current.resourceFailure?.access
        ? current.resourceFailure
        : null,
      saveIssue: null,
    }));
    try {
      const response = await updateWorkspaceFileContentApi(
        token.agentId,
        token.path,
        token.attemptedDraft,
        token.expectedRevision,
      );
      if (!isCurrentSaveToken(scopeRef, token, saveTokenRef)) {
        return false;
      }
      if (!isExactWorkspaceFileResponse(response, token.path)) {
        throw new Error(fallbackCopyRef.current.fallbackSaveError);
      }
      commit(token.key, (current) => ({
        ...current,
        ...mergeSavedTextFile(current, token.attemptedDraft, response),
        command: null,
        hasLoadedContent: true,
        resourceFailure: null,
        saveIssue: null,
      }));
      return true;
    } catch (error) {
      if (!isCurrentSaveToken(scopeRef, token, saveTokenRef)) {
        return false;
      }
      const resourceFailure = getResourceFailure(
        error,
        fallbackCopyRef.current.fallbackSaveError,
      );
      if (resourceFailure.access) {
        commit(token.key, (current) => clearTextFileForAccess(
          current,
          resourceFailure,
        ));
        return false;
      }
      const failure = projectMutationFailure(
        error,
        fallbackCopyRef.current.fallbackSaveError,
      );
      commit(token.key, (current) => ({
        ...current,
        command: null,
        saveIssue: failure.code === "workspace.file_revision_conflict"
          && failure.effect === "not_applied"
          ? { kind: "conflict", phase: "reload_required" }
          : failure.effect === "not_applied"
            ? { detail: failure.message, kind: "not_applied" }
            : {
                attemptedDraft: token.attemptedDraft,
                expectedRevision: token.expectedRevision,
                kind: "outcome_unknown",
                reconciliationFailed: false,
              },
      }));
      return false;
    } finally {
      if (saveTokenRef.current?.id === token.id) {
        saveTokenRef.current = null;
        commit(token.key, (current) => current.command === "save"
          ? { ...current, command: null }
          : current);
      }
    }
  }, [commit, isDirty, state]);

  const save = useCallback(
    () => executeSave("normal"),
    [executeSave],
  );
  const overwriteConflict = useCallback(
    () => executeSave("overwrite_conflict"),
    [executeSave],
  );

  const reconcileSave = useCallback(async (): Promise<void> => {
    const requestScope = scopeRef.current;
    const issue = state.saveIssue;
    if (
      issue?.kind !== "outcome_unknown"
      || state.command
      || saveTokenRef.current?.key === requestScope.key
      || reconcileTokenRef.current?.key === requestScope.key
    ) {
      return;
    }
    const token: SaveToken = {
      ...requestScope,
      attemptedDraft: issue.attemptedDraft,
      expectedRevision: issue.expectedRevision,
      id: reconcileSequenceRef.current + 1,
    };
    reconcileSequenceRef.current = token.id;
    reconcileTokenRef.current = token;
    commit(token.key, (current) => ({ ...current, command: "reconcile" }));
    try {
      const response = await getWorkspaceFileContentApi(token.agentId, token.path);
      if (!isCurrentReconcileToken(scopeRef, token, reconcileTokenRef)) {
        return;
      }
      if (!isExactWorkspaceFileResponse(response, token.path)) {
        throw new Error(fallbackCopyRef.current.fallbackLoadError);
      }
      const reconciliation = classifyTextFileSaveReconciliation(issue, response);
      commit(token.key, (current) => {
        if (!matchesUnknownSaveIssue(current.saveIssue, token)) {
          return current;
        }
        if (reconciliation === "intent_present") {
          return {
            ...current,
            ...mergeSavedTextFile(current, token.attemptedDraft, response),
            command: null,
            hasLoadedContent: true,
            resourceFailure: null,
            saveIssue: null,
          };
        }
        return {
          ...current,
          command: null,
          hasLoadedContent: true,
          resourceFailure: null,
          revision: response.revision,
          savedContent: response.content,
          saveIssue: reconciliation === "retry_ready"
            ? { kind: "retry_ready" }
            : { kind: "conflict", phase: "review" },
        };
      });
    } catch (error) {
      if (!isCurrentReconcileToken(scopeRef, token, reconcileTokenRef)) {
        return;
      }
      const resourceFailure = getResourceFailure(
        error,
        fallbackCopyRef.current.fallbackLoadError,
      );
      commit(token.key, (current) => {
        if (resourceFailure.access) {
          return clearTextFileForAccess(current, resourceFailure);
        }
        if (!matchesUnknownSaveIssue(current.saveIssue, token)) {
          return current;
        }
        return {
          ...current,
          command: null,
          saveIssue: {
            ...current.saveIssue,
            reconciliationFailed: true,
          },
        };
      });
    } finally {
      if (reconcileTokenRef.current?.id === token.id) {
        reconcileTokenRef.current = null;
        commit(token.key, (current) => current.command === "reconcile"
          ? { ...current, command: null }
          : current);
      }
    }
  }, [commit, state.command, state.saveIssue]);

  const adoptLatest = useCallback((): void => {
    commit(scopeKey, (current) => current.saveIssue?.kind === "conflict"
      && current.saveIssue.phase === "review"
      ? {
          ...current,
          draftContent: current.savedContent,
          isEditing: false,
          resourceFailure: null,
          saveIssue: null,
        }
      : current);
  }, [commit, scopeKey]);

  const setDraftContent: Dispatch<SetStateAction<string>> = useCallback((value) => {
    commit(scopeKey, (current) => ({
      ...current,
      draftContent: typeof value === "function"
        ? value(current.draftContent)
        : value,
    }));
  }, [commit, scopeKey]);

  const setIsEditing = useCallback((value: boolean): void => {
    commit(scopeKey, (current) => {
      if (!value) {
        return { ...current, isEditing: false };
      }
      return current.hasLoadedContent
        && Boolean(current.revision)
        && !current.resourceFailure?.access
        && !externalWritingRef.current
        ? { ...current, isEditing: true }
        : current;
    });
  }, [commit, scopeKey]);

  const toggleEditing = useCallback((): void => {
    setIsEditing(!state.isEditing);
  }, [setIsEditing, state.isEditing]);

  const displayContent = isExternalWriting
    && state.hasLoadedContent
    && !isDirty
    && !state.isEditing
    && !state.saveIssue
    && typeof liveState?.live_content === "string"
    ? liveState.live_content
    : state.draftContent;

  return {
    adoptLatest,
    displayContent,
    draftContent: state.draftContent,
    hasLoadedContent: state.hasLoadedContent,
    isDirty,
    isEditing: state.isEditing,
    isExternalWriting,
    isLoading: state.isLoading,
    isReconciling,
    isSaving,
    liveState,
    loadContent,
    overwriteConflict,
    reconcileSave,
    requiresChunkedPreview: state.requiresChunkedPreview,
    resourceFailure: state.resourceFailure,
    revision: state.revision,
    save,
    saveIssue: state.saveIssue,
    setDraftContent,
    setIsEditing,
    toggleEditing,
  };
}

function createTextFileEditorState(scopeKey: string): TextFileEditorState {
  return {
    command: null,
    draftContent: "",
    hasLoadedContent: false,
    isEditing: false,
    isLoading: true,
    requiresChunkedPreview: false,
    resourceFailure: null,
    revision: null,
    savedContent: "",
    saveIssue: null,
    scopeKey,
  };
}

function createSaveToken({
  activeReconcile,
  activeSave,
  isDirty,
  isExternalWriting,
  mode,
  nextId,
  scope,
  state,
}: {
  activeReconcile: SaveToken | null;
  activeSave: SaveToken | null;
  isDirty: boolean;
  isExternalWriting: boolean;
  mode: SaveMode;
  nextId: number;
  scope: TextFileEditorScope;
  state: TextFileEditorState;
}): SaveToken | null {
  if (
    state.scopeKey !== scope.key
    || !state.hasLoadedContent
    || !state.revision
    || !isDirty
    || state.isLoading
    || state.command
    || state.resourceFailure?.access
    || isExternalWriting
    || activeSave?.key === scope.key
    || activeReconcile?.key === scope.key
    || (mode === "normal" && Boolean(
      state.saveIssue
      && state.saveIssue.kind !== "not_applied"
      && state.saveIssue.kind !== "retry_ready",
    ))
    || (mode === "overwrite_conflict" && !(
      state.saveIssue?.kind === "conflict"
      && state.saveIssue.phase === "review"
    ))
  ) {
    return null;
  }
  return {
    ...scope,
    attemptedDraft: state.draftContent,
    expectedRevision: state.revision,
    id: nextId,
  };
}

function mergeLoadedTextFile(
  current: TextFileEditorState,
  response: WorkspaceFileContent,
): TextFileEditorState {
  if (current.saveIssue?.kind === "outcome_unknown") {
    return { ...current, isLoading: false, resourceFailure: null };
  }
  const wasLoaded = current.hasLoadedContent;
  const isDirty = current.draftContent !== current.savedContent;
  const revisionChanged = Boolean(
    current.revision && current.revision !== response.revision,
  );
  const needsReview = wasLoaded && isDirty && revisionChanged;
  const conflictWasWaiting = current.saveIssue?.kind === "conflict";
  const retryBaselineBecameStale = (
    current.saveIssue?.kind === "not_applied"
    || current.saveIssue?.kind === "retry_ready"
  )
    && revisionChanged;
  return {
    ...current,
    draftContent: isDirty ? current.draftContent : response.content,
    hasLoadedContent: true,
    isLoading: false,
    requiresChunkedPreview: false,
    resourceFailure: null,
    revision: response.revision,
    savedContent: response.content,
    saveIssue: needsReview || retryBaselineBecameStale
      ? { kind: "conflict", phase: "review" }
      : conflictWasWaiting && revisionChanged
        ? { kind: "conflict", phase: "review" }
        : conflictWasWaiting
          ? null
          : current.saveIssue,
  };
}

function isWorkspaceWholeFileTooLarge(error: unknown): boolean {
  return error instanceof ApiRequestError
    && error.failure?.version === 1
    && error.failure.code === "workspace.file_too_large";
}

function clearTextFileForAccess(
  current: TextFileEditorState,
  resourceFailure: ResourceFailure,
): TextFileEditorState {
  return {
    ...createTextFileEditorState(current.scopeKey),
    isLoading: false,
    resourceFailure,
  };
}

function isExactWorkspaceFileResponse(
  response: WorkspaceFileContent,
  expectedPath: string,
): boolean {
  return response.path === expectedPath && Boolean(response.revision.trim());
}

function isCurrentEditorRequest(
  scopeRef: { current: TextFileEditorScope },
  expectedScope: TextFileEditorScope,
  sequenceRef: { current: number },
  expectedSequence: number,
): boolean {
  return scopeRef.current.key === expectedScope.key
    && sequenceRef.current === expectedSequence
    && isAuthOwnerScopeGenerationCurrent(expectedScope.ownerGeneration);
}

function isCurrentSaveToken(
  scopeRef: { current: TextFileEditorScope },
  token: SaveToken,
  tokenRef: { current: SaveToken | null },
): boolean {
  return scopeRef.current.key === token.key
    && tokenRef.current?.id === token.id
    && isAuthOwnerScopeGenerationCurrent(token.ownerGeneration);
}

function isCurrentReconcileToken(
  scopeRef: { current: TextFileEditorScope },
  token: SaveToken,
  tokenRef: { current: SaveToken | null },
): boolean {
  return isCurrentSaveToken(scopeRef, token, tokenRef);
}

function matchesUnknownSaveIssue(
  issue: TextFileSaveIssue | null,
  token: SaveToken,
): issue is Extract<TextFileSaveIssue, { kind: "outcome_unknown" }> {
  return issue?.kind === "outcome_unknown"
    && issue.attemptedDraft === token.attemptedDraft
    && issue.expectedRevision === token.expectedRevision;
}
