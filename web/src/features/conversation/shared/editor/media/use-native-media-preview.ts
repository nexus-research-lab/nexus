// INPUT: Exact owner generation/file identity and native load/error or explicit retry events.
// OUTPUT: One event state and a fresh native element key per file/owner/reload scope; settled is not file-success proof.
// POS: Native media lifecycle; owns no fetch, iframe permissions, download or rendering geometry.
import { useCallback, useRef, useSyncExternalStore } from "react";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";

interface NativeMediaState {
  loadState: "error" | "settled" | "loading";
  reloadRevision: number;
}
const INITIAL_STATE: NativeMediaState = { loadState: "loading", reloadRevision: 0 };

export function useNativeMediaPreview(agentId: string, path: string) {
  const ownerGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  const scopeKey = JSON.stringify([ownerGeneration, agentId, path]);
  const scopeRef = useRef({ key: scopeKey, epoch: 0 });
  if (scopeRef.current.key !== scopeKey) {
    scopeRef.current = { key: scopeKey, epoch: scopeRef.current.epoch + 1 };
  }
  const scopeEpoch = scopeRef.current.epoch;
  const [state, setState] = useResettableState(INITIAL_STATE, scopeEpoch);
  const { reloadRevision } = state;
  const commit = useCallback((loadState: "error" | "settled") => {
    setState((current) => scopeRef.current.epoch === scopeEpoch
      && isAuthOwnerScopeGenerationCurrent(ownerGeneration)
      && current.reloadRevision === reloadRevision
      ? { ...current, loadState }
      : current);
  }, [ownerGeneration, reloadRevision, scopeEpoch, setState]);
  const reloadPreview = useCallback(() => {
    if (scopeRef.current.epoch !== scopeEpoch || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)) return;
    setState((current) => ({ loadState: "loading", reloadRevision: current.reloadRevision + 1 }));
  }, [ownerGeneration, scopeEpoch, setState]);

  return {
    loadState: state.loadState,
    previewKey: JSON.stringify([scopeKey, scopeEpoch, reloadRevision]),
    onLoad: () => commit("settled"),
    onError: () => commit("error"),
    reloadPreview,
  };
}
