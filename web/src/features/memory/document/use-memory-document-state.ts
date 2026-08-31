// INPUT: exact Agent + Memory 路径、正文 revision、草稿与恢复状态。
// OUTPUT: scope 隔离的文档基线、草稿、保存问题与纯合并规则。
// POS: Memory 文档状态真相；revision 只做条件写入前提。
import type { MemoryDocument } from "@/types/memory/memory";
import type { ResourceFailure } from "@/lib/error-message";
import type { WorkspaceFileContent } from "@/types/agent/agent";

import {
  type ScopedMemoryCommit,
  type ScopedMemoryScope,
  type ScopedMemoryScopeRef,
  useScopedMemoryState,
} from "../use-scoped-memory-state";

export type MemoryDocumentSaveIssue =
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
    };

export type UnknownMemorySaveIssue = Extract<
  MemoryDocumentSaveIssue,
  { kind: "outcome_unknown" }
>;

export type MemorySaveReconciliation = "conflict" | "not_applied" | "saved";

export interface MemoryDocumentState {
  command: "reconcile" | "save" | null;
  commandError: string | null;
  content: string;
  draft: string;
  editing: boolean;
  isLoading: boolean;
  resourceError: ResourceFailure | null;
  revision: string | null;
  saveIssue: MemoryDocumentSaveIssue | null;
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
  savedRevision: string,
): MemoryDocumentState {
  const draftWasUnchanged = current.draft === savedDraft;
  return {
    ...current,
    commandError: null,
    content: savedContent,
    draft: draftWasUnchanged ? savedContent : current.draft,
    editing: !draftWasUnchanged,
    revision: savedRevision,
    saveIssue: null,
  };
}

export function classifyMemorySaveReconciliation(
  issue: UnknownMemorySaveIssue,
  response: WorkspaceFileContent,
): MemorySaveReconciliation {
  if (response.content === issue.attemptedDraft) {
    return "saved";
  }
  if (response.revision === issue.expectedRevision) {
    return "not_applied";
  }
  return "conflict";
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
    revision: null,
    saveIssue: null,
    scopeKey,
  };
}
