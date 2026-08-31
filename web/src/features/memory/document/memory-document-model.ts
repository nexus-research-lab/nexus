// INPUT: Memory Header 状态、已消费 live 版本和正文 revision。
// OUTPUT: 可执行 Header 动作与 ignore/consume/apply/reload/conflict 实时意图。
// POS: Memory 纯状态投影；不执行 HTTP 或推断提交结果。
import type { WorkspaceLiveFileState } from "@/types/app/workspace-live";
import type { MemoryDocumentKind } from "@/types/memory/memory";

export type MemoryDocumentHeaderAction =
  | {
      disabled: boolean;
      kind: "edit";
    }
  | {
      cancelDisabled: boolean;
      kind: "editing";
      saveDisabled: boolean;
      saving: boolean;
    };

export interface MemoryDocumentHeaderModel {
  action: MemoryDocumentHeaderAction;
  deleteAction: {
    deleting: boolean;
    disabled: boolean;
    visible: boolean;
  };
}

interface MemoryDocumentHeaderState {
  commandBusy: boolean;
  deleteBusy: boolean;
  deleting: boolean;
  dirty: boolean;
  documentKind: MemoryDocumentKind;
  editing: boolean;
  isSaving: boolean;
  revisionReady: boolean;
  runtimeWriting: boolean;
  saveBlocked: boolean;
}

export function buildMemoryDocumentHeaderModel(
  state: MemoryDocumentHeaderState,
): MemoryDocumentHeaderModel {
  return {
    action: state.editing
      ? {
          cancelDisabled: state.commandBusy,
          kind: "editing",
          saveDisabled: !state.dirty
            || state.commandBusy
            || state.runtimeWriting
            || state.saveBlocked
            || !state.revisionReady,
          saving: state.isSaving,
        }
      : {
          disabled: state.runtimeWriting || !state.revisionReady,
          kind: "edit",
        },
    deleteAction: {
      deleting: state.deleting,
      disabled: state.deleteBusy || state.runtimeWriting,
      visible: state.documentKind !== "index" && !state.editing,
    },
  };
}

export interface ConsumedMemoryLiveVersion {
  scopeKey: string;
  version: number;
}

export type MemoryLiveUpdateIntent =
  | { kind: "ignore" }
  | { kind: "consume"; version: number }
  | { kind: "conflict"; version: number }
  | { content: string; kind: "apply"; revision: string; version: number }
  | { kind: "reload"; version: number };

interface MemoryLiveUpdateState {
  consumed: ConsumedMemoryLiveVersion;
  contentRevision: string | null;
  editing: boolean;
  liveState?: WorkspaceLiveFileState;
  saving: boolean;
  scopeKey: string;
}

const IGNORE_LIVE_UPDATE: MemoryLiveUpdateIntent = { kind: "ignore" };

export function resolveMemoryLiveUpdateIntent({
  consumed,
  contentRevision,
  editing,
  liveState,
  saving,
  scopeKey,
}: MemoryLiveUpdateState): MemoryLiveUpdateIntent {
  const activeLiveState = getActiveMemoryLiveState(
    consumed,
    liveState,
    scopeKey,
  );
  if (!activeLiveState) {
    return IGNORE_LIVE_UPDATE;
  }
  if (saving) {
    return IGNORE_LIVE_UPDATE;
  }
  if (
    contentRevision
    && activeLiveState.content_revision === contentRevision
  ) {
    return { kind: "consume", version: activeLiveState.version };
  }
  if (editing) {
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

function getActiveMemoryLiveState(
  consumed: ConsumedMemoryLiveVersion,
  liveState: WorkspaceLiveFileState | undefined,
  scopeKey: string,
): WorkspaceLiveFileState | null {
  if (
    !liveState
    || !scopeKey
    || liveState.status !== "updated"
    || (consumed.scopeKey === scopeKey && liveState.version <= consumed.version)
  ) {
    return null;
  }
  return liveState;
}
