import { useEffect, useRef } from "react";

import type { AssistantContentMode } from "../../message-item-projection";

interface UseProcessExpansionLifecycleOptions {
  assistantContentMode: AssistantContentMode;
  hasPendingPermissions: boolean;
  hasTimedOutQuestion: boolean;
  setIsProcessExpanded: (isOpen: boolean) => void;
}

export function useProcessExpansionLifecycle({
  assistantContentMode,
  hasPendingPermissions,
  hasTimedOutQuestion,
  setIsProcessExpanded,
}: UseProcessExpansionLifecycleOptions): void {
  const wasLiveModeRef = useRef(assistantContentMode === "dm_live");

  useEffect(() => {
    if (hasPendingPermissions || hasTimedOutQuestion) {
      setIsProcessExpanded(true);
    }
  }, [hasPendingPermissions, hasTimedOutQuestion, setIsProcessExpanded]);

  useEffect(() => {
    const isLiveMode = assistantContentMode === "dm_live";
    if (wasLiveModeRef.current && !isLiveMode) {
      setIsProcessExpanded(false);
    }
    wasLiveModeRef.current = isLiveMode;
  }, [assistantContentMode, setIsProcessExpanded]);
}
