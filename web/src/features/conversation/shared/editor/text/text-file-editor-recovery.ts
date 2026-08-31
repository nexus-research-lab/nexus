// INPUT: exact Agent/path live 文件事实、已读取 revision 与一次保存意图。
// OUTPUT: 保存对账、成功合并和实时更新的无副作用决策。
// POS: 通用文本编辑器可靠性纯模型；不发请求、不自动重放写入。
import type { WorkspaceFileContent } from "@/types/agent/agent";
import type { WorkspaceLiveFileState } from "@/types/app/workspace-live";

export type TextFileSaveIssue =
  | {
      kind: "conflict";
      phase: "reload_required" | "review";
    }
  | {
      attemptedDraft: string;
      expectedRevision: string;
      kind: "outcome_unknown";
      reconciliationFailed: boolean;
    }
  | {
      detail: string;
      kind: "not_applied";
    }
  | {
      kind: "retry_ready";
    };

export type TextFileSaveReconciliation =
  | "conflict"
  | "intent_present"
  | "retry_ready";

interface TextFileBaselineState {
  draftContent: string;
  isEditing: boolean;
  revision: string | null;
  savedContent: string;
}

export function mergeSavedTextFile(
  current: TextFileBaselineState,
  submittedDraft: string,
  response: WorkspaceFileContent,
): TextFileBaselineState {
  return {
    draftContent: current.draftContent === submittedDraft
      ? response.content
      : current.draftContent,
    isEditing: current.isEditing,
    revision: response.revision,
    savedContent: response.content,
  };
}

export function classifyTextFileSaveReconciliation(
  issue: Extract<TextFileSaveIssue, { kind: "outcome_unknown" }>,
  response: WorkspaceFileContent,
): TextFileSaveReconciliation {
  if (response.content === issue.attemptedDraft) {
    return "intent_present";
  }
  if (response.revision === issue.expectedRevision) {
    return "retry_ready";
  }
  return "conflict";
}

export interface ConsumedTextFileLiveVersion {
  scopeKey: string;
  version: number;
}

export type TextFileLiveUpdateIntent =
  | { kind: "ignore" }
  | { kind: "consume"; version: number }
  | { kind: "conflict"; version: number }
  | { content: string; kind: "apply"; revision: string; version: number }
  | { kind: "reload"; version: number };

interface TextFileLiveUpdateState {
  agentId: string;
  consumed: ConsumedTextFileLiveVersion;
  hasLoadedContent: boolean;
  isDirty: boolean;
  isEditing: boolean;
  isSaving: boolean;
  liveState?: WorkspaceLiveFileState;
  path: string;
  revision: string | null;
  saveIssue: TextFileSaveIssue | null;
  scopeKey: string;
}

const IGNORE_LIVE_UPDATE: TextFileLiveUpdateIntent = { kind: "ignore" };

export function resolveTextFileLiveUpdateIntent({
  agentId,
  consumed,
  hasLoadedContent,
  isDirty,
  isEditing,
  isSaving,
  liveState,
  path,
  revision,
  saveIssue,
  scopeKey,
}: TextFileLiveUpdateState): TextFileLiveUpdateIntent {
  const activeLiveState = getActiveLiveState({
    agentId,
    consumed,
    liveState,
    path,
    scopeKey,
  });
  if (!activeLiveState || isSaving || saveIssue?.kind === "outcome_unknown") {
    return IGNORE_LIVE_UPDATE;
  }
  if (!hasLoadedContent) {
    return { kind: "reload", version: activeLiveState.version };
  }
  if (
    revision
    && activeLiveState.content_revision === revision
  ) {
    return { kind: "consume", version: activeLiveState.version };
  }
  if (isDirty || isEditing || saveIssue?.kind === "conflict") {
    return { kind: "conflict", version: activeLiveState.version };
  }
  if (
    typeof activeLiveState.live_content === "string"
    && typeof activeLiveState.content_revision === "string"
    && activeLiveState.content_revision
  ) {
    return {
      content: activeLiveState.live_content,
      kind: "apply",
      revision: activeLiveState.content_revision,
      version: activeLiveState.version,
    };
  }
  return { kind: "reload", version: activeLiveState.version };
}

export function completedTextFileLiveVersion(
  liveState?: WorkspaceLiveFileState,
): number {
  if (!liveState) {
    return 0;
  }
  return liveState.status === "updated"
    ? liveState.version
    : Math.max(0, liveState.version - 1);
}

function getActiveLiveState({
  agentId,
  consumed,
  liveState,
  path,
  scopeKey,
}: {
  agentId: string;
  consumed: ConsumedTextFileLiveVersion;
  liveState?: WorkspaceLiveFileState;
  path: string;
  scopeKey: string;
}): WorkspaceLiveFileState | null {
  if (
    !liveState
    || liveState.agent_id !== agentId
    || liveState.path !== path
    || liveState.status !== "updated"
    || (consumed.scopeKey === scopeKey && liveState.version <= consumed.version)
  ) {
    return null;
  }
  return liveState;
}
