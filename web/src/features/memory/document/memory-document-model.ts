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
  deleteBusy: boolean;
  deleting: boolean;
  dirty: boolean;
  documentKind: MemoryDocumentKind;
  editing: boolean;
  isSaving: boolean;
  runtimeWriting: boolean;
}

export function buildMemoryDocumentHeaderModel(
  state: MemoryDocumentHeaderState,
): MemoryDocumentHeaderModel {
  return {
    action: state.editing
      ? {
          cancelDisabled: state.isSaving,
          kind: "editing",
          saveDisabled: !state.dirty || state.isSaving || state.runtimeWriting,
          saving: state.isSaving,
        }
      : {
          disabled: state.runtimeWriting,
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
  | { content: string; kind: "apply"; version: number }
  | { kind: "reload"; version: number };

interface MemoryLiveUpdateState {
  consumed: ConsumedMemoryLiveVersion;
  editing: boolean;
  liveState?: WorkspaceLiveFileState;
  scopeKey: string;
}

const IGNORE_LIVE_UPDATE: MemoryLiveUpdateIntent = { kind: "ignore" };

export function resolveMemoryLiveUpdateIntent({
  consumed,
  editing,
  liveState,
  scopeKey,
}: MemoryLiveUpdateState): MemoryLiveUpdateIntent {
  const activeLiveState = getActiveMemoryLiveState(editing, liveState, scopeKey);
  if (!activeLiveState) {
    return IGNORE_LIVE_UPDATE;
  }
  if (typeof activeLiveState.live_content === "string") {
    return {
      content: activeLiveState.live_content,
      kind: "apply",
      version: activeLiveState.version,
    };
  }
  return shouldReloadMemoryLiveState(activeLiveState, scopeKey, consumed)
    ? { kind: "reload", version: activeLiveState.version }
    : IGNORE_LIVE_UPDATE;
}

function getActiveMemoryLiveState(
  editing: boolean,
  liveState: WorkspaceLiveFileState | undefined,
  scopeKey: string,
): WorkspaceLiveFileState | null {
  return editing || !liveState || !scopeKey ? null : liveState;
}

function shouldReloadMemoryLiveState(
  liveState: WorkspaceLiveFileState,
  scopeKey: string,
  consumed: ConsumedMemoryLiveVersion,
): boolean {
  return liveState.status === "updated"
    && (consumed.scopeKey !== scopeKey || liveState.version > consumed.version);
}
