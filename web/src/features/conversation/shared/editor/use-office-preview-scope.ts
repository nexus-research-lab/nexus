// INPUT: Exact owner generation/Agent/path and explicit retry intent.
// OUTPUT: Stable file/request reset keys and a fence rejecting superseded Office work.
// POS: Shared Office scope lifecycle; consumers own cancellation, parsing, DOM and resource disposal.
import { useCallback, useRef, useSyncExternalStore } from "react";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";

export function useOfficePreviewScope(agentId: string, path: string) {
  const ownerGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  const identity = JSON.stringify([ownerGeneration, agentId, path]);
  const scopeRef = useRef({ identity, epoch: 0 });
  if (scopeRef.current.identity !== identity) {
    scopeRef.current = { identity, epoch: scopeRef.current.epoch + 1 };
  }
  const scopeKey = scopeRef.current.epoch;
  const [retryRevision, setRetryRevision] = useResettableState(0, scopeKey);
  const requestKey = `${scopeKey}:${retryRevision}`;
  const requestRef = useRef(requestKey);
  requestRef.current = requestKey;
  const isCurrent = useCallback(() => requestRef.current === requestKey
    && isAuthOwnerScopeGenerationCurrent(ownerGeneration), [ownerGeneration, requestKey]);
  const retryPreview = useCallback(() => {
    if (isCurrent()) setRetryRevision((current) => current + 1);
  }, [isCurrent, setRetryRevision]);

  return { scopeKey, requestKey, isCurrent, retryPreview };
}
